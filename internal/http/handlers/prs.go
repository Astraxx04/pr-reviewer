package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
)

// prEnqueuer is the subset of river.Client used to trigger re-reviews.
type prEnqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type PRHandler struct {
	db       *gorm.DB
	enqueuer prEnqueuer // nil when river is not configured
	ghClient gh.Client  // optional; enables the /diff endpoint
}

func NewPRHandler(db *gorm.DB, enqueuer prEnqueuer) *PRHandler {
	return &PRHandler{db: db, enqueuer: enqueuer}
}

func (h *PRHandler) WithGHClient(c gh.Client) *PRHandler {
	h.ghClient = c
	return h
}

// prStatus maps the latest review status to a human-readable PR status.
func prStatus(reviewStatus string) string {
	switch reviewStatus {
	case "APPROVE":
		return "APPROVED"
	case "REQUEST_CHANGES":
		return "CHANGES_REQUESTED"
	case "COMMENT":
		return "COMMENTED"
	default:
		return "PENDING"
	}
}

// ---- List ---------------------------------------------------------------

type prListRow struct {
	ID             uint
	Number         int
	Title          string
	Author         string
	HeadSHA        string
	Owner          string
	RepoName       string
	CurrentScore   int
	CurrentStatus  string
	ReviewCount    int64
	LastReviewedAt *time.Time
	CreatedAt      time.Time
}

func (h *PRHandler) List(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	perPage := queryInt(r, "per_page", 20)
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	q := r.URL.Query()
	repoFilter := q.Get("repo") // "owner/name"
	authorFilter := q.Get("author")
	statusFilter := q.Get("status") // approved | changes_requested | commented | pending

	base := `
		SELECT
			pr.id, pr.number, pr.title, pr.author, pr.head_sha, pr.created_at,
			repo.owner, repo.name AS repo_name,
			COALESCE(rv_latest.score, 0)  AS current_score,
			COALESCE(rv_latest.status, '') AS current_status,
			COALESCE(rv_count.cnt, 0)     AS review_count,
			rv_latest.created_at          AS last_reviewed_at
		FROM pull_requests pr
		JOIN repositories repo ON repo.id = pr.repo_id
		LEFT JOIN LATERAL (
			SELECT score, status, created_at
			FROM reviews WHERE pr_id = pr.id
			ORDER BY created_at DESC LIMIT 1
		) rv_latest ON true
		LEFT JOIN (
			SELECT pr_id, COUNT(*) AS cnt FROM reviews GROUP BY pr_id
		) rv_count ON rv_count.pr_id = pr.id
		WHERE 1=1`

	args := []any{}

	if repoFilter != "" {
		base += " AND repo.owner || '/' || repo.name = ?"
		args = append(args, repoFilter)
	}
	if authorFilter != "" {
		base += " AND pr.author = ?"
		args = append(args, authorFilter)
	}
	switch statusFilter {
	case "approved":
		base += " AND rv_latest.status = 'APPROVE'"
	case "changes_requested":
		base += " AND rv_latest.status = 'REQUEST_CHANGES'"
	case "commented":
		base += " AND rv_latest.status = 'COMMENT'"
	case "pending":
		base += " AND rv_latest.status IS NULL"
	}

	// Total count
	var total int64
	countSQL := "SELECT COUNT(*) FROM (" + base + ") AS sub"
	h.db.WithContext(r.Context()).Raw(countSQL, args...).Scan(&total)

	// Paginated rows
	base += " ORDER BY COALESCE(rv_latest.created_at, pr.created_at) DESC LIMIT ? OFFSET ?"
	args = append(args, perPage, offset)

	var rows []prListRow
	h.db.WithContext(r.Context()).Raw(base, args...).Scan(&rows)

	type prSummary struct {
		ID             uint       `json:"id"`
		Number         int        `json:"number"`
		Title          string     `json:"title"`
		Author         string     `json:"author"`
		Repo           string     `json:"repo"`
		HeadSHA        string     `json:"head_sha"`
		PRStatus       string     `json:"pr_status"`
		CurrentScore   int        `json:"current_score"`
		ReviewCount    int64      `json:"review_count"`
		LastReviewedAt *time.Time `json:"last_reviewed_at"`
		CreatedAt      time.Time  `json:"created_at"`
	}
	out := make([]prSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, prSummary{
			ID:             row.ID,
			Number:         row.Number,
			Title:          row.Title,
			Author:         row.Author,
			Repo:           row.Owner + "/" + row.RepoName,
			HeadSHA:        row.HeadSHA,
			PRStatus:       prStatus(row.CurrentStatus),
			CurrentScore:   row.CurrentScore,
			ReviewCount:    row.ReviewCount,
			LastReviewedAt: row.LastReviewedAt,
			CreatedAt:      row.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"prs":      out,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// ---- Detail -------------------------------------------------------------

func (h *PRHandler) Get(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repoName := r.PathValue("repo")
	numberStr := r.PathValue("number")
	number, err := strconv.Atoi(numberStr)
	if err != nil || owner == "" || repoName == "" {
		writeError(w, http.StatusBadRequest, "invalid parameters")
		return
	}

	// Find repository
	var repo models.Repository
	if err := h.db.WithContext(r.Context()).
		Where("owner = ? AND name = ?", owner, repoName).
		First(&repo).Error; err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	// Find PR
	var pr models.PullRequest
	if err := h.db.WithContext(r.Context()).
		Where("repo_id = ? AND number = ?", repo.ID, number).
		First(&pr).Error; err != nil {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}

	// All reviews in chronological order (for score history)
	var reviews []models.Review
	h.db.WithContext(r.Context()).
		Preload("Comments").
		Preload("Assignments").
		Where("pr_id = ?", pr.ID).
		Order("created_at asc").
		Find(&reviews)

	// Derive PR status from latest review
	currentStatus := ""
	if len(reviews) > 0 {
		currentStatus = reviews[len(reviews)-1].Status
	}

	// Score history
	type reviewSummary struct {
		ID           uint      `json:"id"`
		Status       string    `json:"status"`
		Score        int       `json:"score"`
		Summary      string    `json:"summary"`
		CommentCount int       `json:"comment_count"`
		CreatedAt    time.Time `json:"created_at"`
	}
	reviewSummaries := make([]reviewSummary, 0, len(reviews))
	for _, rv := range reviews {
		reviewSummaries = append(reviewSummaries, reviewSummary{
			ID:           rv.ID,
			Status:       rv.Status,
			Score:        rv.Score,
			Summary:      rv.Summary,
			CommentCount: len(rv.Comments),
			CreatedAt:    rv.CreatedAt,
		})
	}

	// Latest review comments with reply indicator
	type commentOut struct {
		ID       uint   `json:"id"`
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Side     string `json:"side"`
		Body     string `json:"body"`
		Severity string `json:"severity"`
		Priority string `json:"priority"`
		HasReply bool   `json:"has_reply"`
	}
	var latestComments []commentOut
	if len(reviews) > 0 {
		latest := reviews[len(reviews)-1]
		// Fetch bot comment IDs for these review comments to check reply status.
		var botComments []models.BotComment
		h.db.WithContext(r.Context()).Where("review_id = ?", latest.ID).Find(&botComments)
		botCommentSet := make(map[int64]bool, len(botComments))
		for _, bc := range botComments {
			botCommentSet[bc.GithubCommentID] = true
		}
		// Fetch which have replies
		if len(botCommentSet) > 0 {
			ids := make([]int64, 0, len(botCommentSet))
			for id := range botCommentSet {
				ids = append(ids, id)
			}
			var replied []models.BotReply
			h.db.WithContext(r.Context()).Where("github_comment_id IN ?", ids).Find(&replied)
			for _, rep := range replied {
				botCommentSet[rep.GithubCommentID] = false // mark as "has reply"
			}
		}

		for _, c := range latest.Comments {
			hasReply := false
			for _, bc := range botComments {
				if bc.Path == c.Path && bc.Line == c.Line {
					hasReply = !botCommentSet[bc.GithubCommentID]
					break
				}
			}
			latestComments = append(latestComments, commentOut{
				ID: c.ID, Path: c.Path, Line: c.Line, Side: c.Side,
				Body: c.Body, Severity: c.Severity, Priority: c.Priority,
				HasReply: hasReply,
			})
		}
	}

	// Assignees from latest review
	var assignees []string
	if len(reviews) > 0 {
		for _, a := range reviews[len(reviews)-1].Assignments {
			assignees = append(assignees, a.AssigneeLogin)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":              pr.ID,
		"number":          pr.Number,
		"title":           pr.Title,
		"author":          pr.Author,
		"repo":            owner + "/" + repoName,
		"head_sha":        pr.HeadSHA,
		"pr_status":       prStatus(currentStatus),
		"reviews":         reviewSummaries,
		"latest_comments": latestComments,
		"assignees":       assignees,
		"created_at":      pr.CreatedAt,
	})
}

// ---- Diff ---------------------------------------------------------------

// Diff proxies the PR unified diff from GitHub so the frontend can render it.
func (h *PRHandler) Diff(w http.ResponseWriter, r *http.Request) {
	if h.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub client not configured")
		return
	}
	owner := r.PathValue("owner")
	repoName := r.PathValue("repo")
	numberStr := r.PathValue("number")
	number, err := strconv.Atoi(numberStr)
	if err != nil || owner == "" || repoName == "" {
		writeError(w, http.StatusBadRequest, "invalid parameters")
		return
	}
	diffs, err := h.ghClient.GetPullRequestDiff(r.Context(), owner, repoName, number)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch diff: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diffs)
}

// ---- Re-review ----------------------------------------------------------

func (h *PRHandler) ReReview(w http.ResponseWriter, r *http.Request) {
	if h.enqueuer == nil {
		writeError(w, http.StatusServiceUnavailable, "queue not available")
		return
	}
	owner := r.PathValue("owner")
	repoName := r.PathValue("repo")
	numberStr := r.PathValue("number")
	number, err := strconv.Atoi(numberStr)
	if err != nil || owner == "" || repoName == "" {
		writeError(w, http.StatusBadRequest, "invalid parameters")
		return
	}
	insertOpts := &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable},
		},
	}
	if _, err := h.enqueuer.Insert(r.Context(), jobs.ReviewJobArgs{
		Owner:  owner,
		Repo:   repoName,
		Number: number,
		Action: "re-review",
	}, insertOpts); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue re-review")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}
