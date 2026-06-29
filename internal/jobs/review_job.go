package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/riverqueue/river"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel/attribute"

	"github.com/Astraxx04/pr-reviewer/internal/ai"
	"github.com/Astraxx04/pr-reviewer/internal/ai/rag"
	"github.com/Astraxx04/pr-reviewer/internal/assignments"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	"github.com/Astraxx04/pr-reviewer/internal/events"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/integration/jira"
	slackint "github.com/Astraxx04/pr-reviewer/internal/integration/slack"
	"github.com/Astraxx04/pr-reviewer/internal/metrics"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
	"github.com/Astraxx04/pr-reviewer/internal/pr"
	"github.com/Astraxx04/pr-reviewer/internal/review"
	"github.com/Astraxx04/pr-reviewer/internal/rules"
	"github.com/Astraxx04/pr-reviewer/internal/telemetry"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// repoReviewConfig holds per-repo Section 8 settings stored in Repository.Config JSON.
type repoReviewConfig struct {
	Agents             map[string]ai.AgentConfig `json:"agents"`
	MaxDiffLines       int                       `json:"max_diff_lines"`
	AutoLabel          bool                      `json:"auto_label"`
	ConsensusThreshold int                       `json:"consensus_threshold"`
	CommitStatus       commitStatusConfig        `json:"commit_status"`
}

// commitStatusConfig controls posting a GitHub commit status for branch protection.
type commitStatusConfig struct {
	Enabled  bool `json:"enabled"`
	MinScore int  `json:"min_score"` // score below this fails the check
}

// commitStatusContext is the unique status label shown in the GitHub merge box.
const commitStatusContext = "pr-reviewer"

// ReviewJobArgs is the payload enqueued by the webhook handler.
type ReviewJobArgs struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Action string `json:"action"`
	// SlackResponseURL, when set (e.g. by a /review slash command), receives the
	// review result so the requester gets an answer back in Slack.
	SlackResponseURL string `json:"slack_response_url,omitempty"`
}

func (ReviewJobArgs) Kind() string { return "review" }

// ReviewWorker processes a single PR review end-to-end.
type ReviewWorker struct {
	river.WorkerDefaults[ReviewJobArgs]

	PRService     pr.Service
	AIService     ai.Service
	Aggregator    review.Aggregator
	GHClient      gh.Client
	DB            *gorm.DB
	Log           *logger.Logger
	Indexer       *rag.Indexer          // optional: indexes comments for RAG
	NotifService  notifications.Service // optional: sends Slack/email/webhook notifications
	EventHub      *events.Hub           // optional: publishes SSE events to connected clients
	EncryptionKey string                // for decrypting integration credentials (e.g. Jira)
	FrontendURL   string                // base URL for commit-status target links
}

// Timeout overrides River's 1-minute default. A review runs several AI agents
// (each an LLM call) plus RAG retrieval and GitHub API calls, which easily
// exceeds a minute on a non-trivial PR — the default would cancel it mid-review.
func (w *ReviewWorker) Timeout(*river.Job[ReviewJobArgs]) time.Duration {
	return 15 * time.Minute
}

func (w *ReviewWorker) Work(ctx context.Context, job *river.Job[ReviewJobArgs]) error {
	args := job.Args
	ctx, span := telemetry.Tracer().Start(ctx, "review.job")
	defer span.End()
	span.SetAttributes(
		attribute.String("pr.owner", args.Owner),
		attribute.String("pr.repo", args.Repo),
		attribute.Int("pr.number", args.Number),
	)
	start := time.Now()

	prCtx, err := w.PRService.BuildContext(ctx, args.Owner, args.Repo, args.Number, args.Action)
	if err != nil {
		w.Log.Error("failed to build PR context", "error", err)
		return err
	}

	// Resolve DB repo early so we can pass RepoID to the AI service for RAG scoping.
	var dbRepo *models.Repository
	if w.DB != nil {
		dbRepo, _ = repo.FindOrCreateRepo(ctx, w.DB, args.Owner, args.Repo)
	}

	// Parse per-repo config (Section 8 settings + agent overrides).
	var fullCfg repoReviewConfig
	var repoConfig map[string]ai.AgentConfig
	if dbRepo != nil && len(dbRepo.Config) > 0 {
		_ = json.Unmarshal(dbRepo.Config, &fullCfg)
		if len(fullCfg.Agents) > 0 {
			repoConfig = fullCfg.Agents
		} else {
			// Backward compat: flat format without "agents" nesting.
			_ = json.Unmarshal(dbRepo.Config, &repoConfig)
		}
	}
	maxDiffLines := fullCfg.MaxDiffLines
	if maxDiffLines == 0 {
		maxDiffLines = 3000
	}

	// Post a pending commit status up front so the PR shows the check is running.
	if fullCfg.CommitStatus.Enabled {
		w.postCommitStatus(ctx, args, prCtx.PR.Head.Sha, "pending", "AI review in progress…")
	}

	// Auto-resolve stale bot comments on changed paths when PR is updated.
	if args.Action == "synchronize" && w.DB != nil {
		changedPaths := make(map[string]bool, len(prCtx.Diff))
		for _, f := range prCtx.Diff {
			changedPaths[f.Filename] = true
		}
		var stale []models.BotComment
		w.DB.WithContext(ctx).Where("resolved = false").Find(&stale)
		for _, bc := range stale {
			if !changedPaths[bc.Path] {
				continue
			}
			if err := w.GHClient.PostReviewCommentReply(ctx, args.Owner, args.Repo, args.Number,
				bc.GithubCommentID, "✅ This concern appears to have been addressed in the latest push."); err != nil {
				w.Log.Error("failed to post stale-comment reply", "comment_id", bc.GithubCommentID, "error", err)
			} else {
				w.DB.WithContext(ctx).Model(&bc).Update("resolved", true)
				// 9.4: index this fix so future reviews can recall known-good patterns.
				if w.Indexer != nil && dbRepo != nil {
					_ = w.Indexer.IndexFix(ctx, dbRepo.ID, bc.Path, bc.Body)
				}
			}
		}
	}

	// Fetch .pr-reviewer-ignore and filter diff accordingly.
	diff := prCtx.Diff
	if ignoreContent, err := w.GHClient.GetFileContent(ctx, args.Owner, args.Repo, ".pr-reviewer-ignore"); err == nil {
		var ignorePatterns []string
		for _, line := range strings.Split(ignoreContent, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				ignorePatterns = append(ignorePatterns, line)
			}
		}
		if len(ignorePatterns) > 0 {
			filtered := diff[:0]
			for _, f := range diff {
				if !rules.ShouldIgnore(f.Filename, ignorePatterns) {
					filtered = append(filtered, f)
				}
			}
			diff = filtered
		}
	}

	// Apply max_diff_lines guard.
	var diffTruncated bool
	totalLines := 0
	for _, f := range diff {
		totalLines += strings.Count(f.Patch, "\n")
	}
	if totalLines > maxDiffLines {
		diffTruncated = true
		diff = nil // drop diff; prompt will note it was truncated
	}

	// Fetch and evaluate .pr-reviewer.yml custom rules.
	var customViolations []string
	if ruleCfg, err := rules.FetchAndParse(ctx, w.GHClient, args.Owner, args.Repo); err != nil {
		w.Log.Error("failed to parse .pr-reviewer.yml", "error", err)
	} else if ruleCfg != nil {
		customViolations = rules.Evaluate(ruleCfg, diff)
		if len(customViolations) > 0 {
			w.Log.Info("custom rule violations found", "count", len(customViolations))
		}
	}

	// Fetch PR template for template-awareness check.
	prTemplate, _ := w.GHClient.GetFileContent(ctx, args.Owner, args.Repo, ".github/pull_request_template.md")

	// Load false positive patterns: comment bodies that received 2+ thumbs-down votes.
	var falsePositivePatterns []string
	if w.DB != nil {
		type fpRow struct{ Body string }
		var fps []fpRow
		w.DB.WithContext(ctx).Raw(`
			SELECT rc.body
			FROM review_comments rc
			JOIN comment_feedbacks cf ON cf.review_comment_id = rc.id
			WHERE cf.vote = -1
			GROUP BY rc.body
			HAVING COUNT(*) >= 2
		`).Scan(&fps)
		for _, fp := range fps {
			falsePositivePatterns = append(falsePositivePatterns, fp.Body)
		}
	}

	// Fetch Jira ticket context for any ticket refs in the PR title or body.
	ticketContext := jira.FetchContextForPR(ctx, w.DB, w.EncryptionKey, prCtx.Title+" "+prCtx.Body)
	if ticketContext != "" {
		w.Log.Info("jira context injected", "keys", jira.DetectRefs(prCtx.Title+" "+prCtx.Body), "chars", len(ticketContext))
	}

	result, err := w.AIService.Review(ctx, ai.AnalysisRequest{
		Diff:                  diff,
		Title:                 prCtx.Title,
		Body:                  prCtx.Body,
		RepoID:                repoIDOf(dbRepo),
		RepoConfig:            repoConfig,
		TicketContext:         ticketContext,
		FalsePositivePatterns: falsePositivePatterns,
		CustomViolations:      customViolations,
		DiffTruncated:         diffTruncated,
		PRTemplate:            prTemplate,
		ConsensusThreshold:    fullCfg.ConsensusThreshold,
	})
	if err != nil {
		w.Log.Error("AI review failed", "error", err)
		return err
	}

	finalReview, err := w.Aggregator.Aggregate(ctx, []ai.ReviewResult{*result})
	if err != nil {
		w.Log.Error("aggregation failed", "error", err)
		return err
	}

	ghReviewID, err := w.GHClient.PostReview(ctx, args.Owner, args.Repo, args.Number, &gh.ReviewSubmission{
		Body:     finalReview.Summary,
		Event:    finalReview.Status,
		Comments: finalReview.Comments,
	})
	if err != nil {
		w.Log.Error("failed to post review to GitHub", "error", err)
		// Don't retry on permanent client errors (4xx) — retrying won't fix them.
		if isClientError(err) {
			return river.JobCancel(err)
		}
		return err
	}

	latency := time.Since(start).Milliseconds()
	w.Log.Info("review posted", "status", finalReview.Status, "comments", len(finalReview.Comments), "ms", latency)

	// Post a human-readable summary comment with grouped P0–P3 findings.
	if err := w.GHClient.PostSummaryComment(ctx, args.Owner, args.Repo, args.Number, &gh.ReviewSubmission{
		Body:     finalReview.Summary,
		Event:    finalReview.Status,
		Comments: finalReview.Comments,
	}, result.Score); err != nil {
		w.Log.Error("failed to post summary comment", "error", err)
		// Non-fatal — inline review was already posted.
	}

	// Update the commit status with the final verdict (non-fatal).
	if fullCfg.CommitStatus.Enabled {
		state := "success"
		desc := fmt.Sprintf("Score %d/100 — passed", result.Score)
		if result.Score < fullCfg.CommitStatus.MinScore {
			state = "failure"
			desc = fmt.Sprintf("Score %d/100 — below threshold of %d", result.Score, fullCfg.CommitStatus.MinScore)
		}
		w.postCommitStatus(ctx, args, prCtx.PR.Head.Sha, state, desc)
	}

	// Persist to database (non-fatal).
	reviewRow, err := w.persist(ctx, dbRepo, args, prCtx, finalReview, result.Score, latency, result.InputTokens, result.OutputTokens)
	if err != nil {
		w.Log.Error("failed to persist review", "error", err)
	}

	// Notify on review complete (non-fatal).
	if w.NotifService != nil && dbRepo != nil {
		prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", args.Owner, args.Repo, args.Number)
		// A re-review is an explicitly re-requested review (the /re-review button
		// or a Slack trigger) — not the initial "opened" review or an automatic
		// "synchronize" on push.
		isReReview := args.Action == "re-review" || args.Action == "slack" || args.Action == "slack_mention"
		w.NotifService.NotifyReviewComplete(ctx, dbRepo.ID, prCtx.Title, prURL, finalReview.Summary, result.Score, isReReview)
	}

	// Track bot comment IDs so we can detect replies for conversational re-review.
	if w.DB != nil && ghReviewID != 0 && reviewRow != nil {
		if ghComments, err := w.GHClient.GetReviewCommentsByReview(ctx, args.Owner, args.Repo, args.Number, ghReviewID); err != nil {
			w.Log.Error("failed to fetch review comments for tracking", "error", err)
		} else {
			now := time.Now()
			for _, c := range ghComments {
				w.DB.WithContext(ctx).Create(&models.BotComment{
					ReviewID:        reviewRow.ID,
					GithubCommentID: c.ID,
					Path:            c.Path,
					Line:            c.Line,
					Body:            c.Body,
					CreatedAt:       now,
				})
			}
			if len(ghComments) > 0 {
				w.Log.Info("bot comments tracked", "count", len(ghComments))
			}
		}
	}

	// Index comments for future RAG retrieval (non-fatal).
	if w.Indexer != nil && dbRepo != nil && len(finalReview.Comments) > 0 {
		if err := w.Indexer.IndexComments(ctx, dbRepo.ID, finalReview.Comments); err != nil {
			w.Log.Error("failed to index review comments", "error", err)
		}
	}

	// 9.3: Incrementally re-index files changed in this PR (non-fatal).
	if w.Indexer != nil && dbRepo != nil && len(prCtx.Diff) > 0 {
		for _, f := range prCtx.Diff {
			if f.Status == "removed" || f.Patch == "" {
				continue
			}
			content, err := w.GHClient.GetFileContent(ctx, args.Owner, args.Repo, f.Filename)
			if err != nil {
				continue
			}
			if err := w.Indexer.IndexFile(ctx, dbRepo.ID, f.Filename, content); err != nil {
				w.Log.Error("incremental re-index failed", "path", f.Filename, "error", err)
			}
		}
	}

	// Assign reviewers according to configured rules (non-fatal).
	if dbRepo != nil {
		if err := w.assign(ctx, args, prCtx, dbRepo, reviewRow, finalReview); err != nil {
			w.Log.Error("assignment failed", "error", err)
		}
	}

	// Apply auto-labels based on review score (non-fatal).
	if fullCfg.AutoLabel {
		if err := w.applyLabels(ctx, args, result.Score); err != nil {
			w.Log.Error("auto-label failed", "error", err)
		}
	}

	metrics.ReviewDuration.WithLabelValues(finalReview.Status).
		Observe(time.Since(start).Seconds())

	// Reply to the Slack /review slash command that triggered this review (non-fatal).
	if args.SlackResponseURL != "" {
		prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", args.Owner, args.Repo, args.Number)
		msg := fmt.Sprintf("%s *<%s|%s/%s#%d>* — score *%d/100*\n> %s",
			verdictEmoji(finalReview.Status), prURL, args.Owner, args.Repo, args.Number, result.Score,
			truncateSummary(finalReview.Summary, 280))
		if err := slackint.PostResponseURL(ctx, args.SlackResponseURL, msg); err != nil {
			w.Log.Error("failed to post Slack response", "error", err)
		}
	}

	// Notify connected UI clients via SSE.
	if w.EventHub != nil {
		w.EventHub.Publish(events.Event{
			Type: "review_complete",
			Data: map[string]any{
				"owner":  args.Owner,
				"repo":   args.Repo,
				"number": args.Number,
				"score":  result.Score,
				"status": finalReview.Status,
			},
		})
	}

	return nil
}

func (w *ReviewWorker) persist(
	ctx context.Context,
	dbRepo *models.Repository, // may be nil (in-process mode with no DB)
	args ReviewJobArgs,
	prCtx *pr.PRContext,
	finalReview *review.FinalReview,
	score int,
	latencyMS int64,
	inputTokens int,
	outputTokens int,
) (*models.Review, error) {
	if dbRepo == nil {
		return nil, nil
	}

	prRow := &models.PullRequest{
		RepoID:  dbRepo.ID,
		Number:  args.Number,
		Title:   prCtx.Title,
		Author:  prCtx.PR.Author.Login,
		HeadSHA: prCtx.PR.Head.Sha,
	}
	prRepo := repo.NewPRRepo(w.DB)
	if err := prRepo.Upsert(ctx, prRow); err != nil {
		return nil, err
	}

	var comments []models.ReviewComment
	for _, c := range finalReview.Comments {
		comments = append(comments, models.ReviewComment{
			Path:     c.Path,
			Line:     c.Line,
			Side:     c.Side,
			Body:     c.Body,
			Severity: c.Severity,
			Priority: c.Priority,
		})
	}

	reviewRow := &models.Review{
		PRID:         prRow.ID,
		Status:       finalReview.Status,
		Score:        score,
		Summary:      finalReview.Summary,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		LatencyMS:    latencyMS,
		Comments:     comments,
	}
	if err := repo.NewReviewRepo(w.DB).Create(ctx, reviewRow); err != nil {
		return nil, err
	}

	return reviewRow, nil
}

func (w *ReviewWorker) assign(
	ctx context.Context,
	args ReviewJobArgs,
	prCtx *pr.PRContext,
	dbRepo *models.Repository,
	reviewRow *models.Review, // may be nil if persist failed
	finalReview *review.FinalReview,
) error {
	rules, err := repo.NewAssignmentRuleRepo(w.DB).List(ctx, dbRepo.ID)
	if err != nil || len(rules) == 0 {
		return err
	}

	codeowners, _ := w.GHClient.GetCODEOWNERS(ctx, args.Owner, args.Repo)

	seen := make(map[string]bool)
	var allAssignees []string

	for _, rule := range rules {
		res, err := assignments.Evaluate(ctx, w.DB, &rule, prCtx, codeowners, dbRepo.InstallationID)
		if err != nil {
			w.Log.Error("evaluate assignment rule", "rule_id", rule.ID, "error", err)
			continue
		}
		for _, a := range res.Assignees {
			if !seen[a] {
				seen[a] = true
				allAssignees = append(allAssignees, a)
			}
		}
	}

	if len(allAssignees) == 0 {
		return nil
	}

	w.Log.Info("assigning reviewers", "assignees", allAssignees)

	// Persist assignment rows.
	if reviewRow != nil {
		now := time.Now()
		for _, login := range allAssignees {
			w.DB.WithContext(ctx).Create(&models.Assignment{
				ReviewID:      reviewRow.ID,
				AssigneeLogin: login,
				AssignedAt:    now,
			})
		}
	}

	// Request reviewers on GitHub (best-effort, non-fatal).
	if err := w.GHClient.RequestReviewers(ctx, args.Owner, args.Repo, args.Number, allAssignees); err != nil {
		w.Log.Error("RequestReviewers failed", "error", err)
	}

	// Send notifications.
	if w.NotifService != nil {
		prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", args.Owner, args.Repo, args.Number)
		for _, login := range allAssignees {
			w.NotifService.NotifyAssignment(ctx, dbRepo.ID, login, prCtx.Title, prURL, finalReview.Summary)
		}
	}

	return nil
}

// applyLabels ensures the score-appropriate label exists and adds it to the PR.
func (w *ReviewWorker) applyLabels(ctx context.Context, args ReviewJobArgs, score int) error {
	var labelName, color string
	switch {
	case score >= 80:
		labelName, color = "pr-reviewer: approved", "0e8a16"
	case score >= 60:
		labelName, color = "pr-reviewer: needs-minor-changes", "e4e669"
	default:
		labelName, color = "pr-reviewer: needs-changes", "d93f0b"
	}
	if err := w.GHClient.EnsureLabel(ctx, args.Owner, args.Repo, labelName, color, "Applied by pr-reviewer bot"); err != nil {
		return err
	}
	return w.GHClient.AddLabelsToIssue(ctx, args.Owner, args.Repo, args.Number, []string{labelName})
}

// postCommitStatus posts a GitHub commit status for branch-protection use (non-fatal).
func (w *ReviewWorker) postCommitStatus(ctx context.Context, args ReviewJobArgs, sha, state, desc string) {
	if sha == "" {
		return
	}
	targetURL := ""
	if w.FrontendURL != "" {
		targetURL = fmt.Sprintf("%s/prs/%s/%s/%d", strings.TrimRight(w.FrontendURL, "/"), args.Owner, args.Repo, args.Number)
	}
	if err := w.GHClient.CreateStatus(ctx, args.Owner, args.Repo, sha, &gh.CommitStatus{
		State:       state,
		Description: desc,
		Context:     commitStatusContext,
		TargetURL:   targetURL,
	}); err != nil {
		w.Log.Error("failed to post commit status", "state", state, "error", err)
	}
}

func verdictEmoji(status string) string {
	switch status {
	case "REQUEST_CHANGES":
		return "❌ Changes requested"
	case "COMMENT":
		return "💬 Commented"
	case "APPROVE":
		return "✅ Approved"
	default:
		return status
	}
}

func truncateSummary(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func repoIDOf(r *models.Repository) uint {
	if r == nil {
		return 0
	}
	return r.ID
}

// isClientError returns true for HTTP 4xx errors from GitHub — retrying won't help.
func isClientError(err error) bool {
	if ghErr, ok := err.(*gogithub.ErrorResponse); ok {
		if ghErr.Response != nil {
			code := ghErr.Response.StatusCode
			return code >= http.StatusBadRequest && code < http.StatusInternalServerError
		}
	}
	return false
}
