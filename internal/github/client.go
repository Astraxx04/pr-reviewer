package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"

	"github.com/Astraxx04/pr-reviewer/internal/telemetry"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// Client is the interface for interacting with GitHub.
type Client interface {
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
	GetPullRequestDiff(ctx context.Context, owner, repo string, number int) ([]FileDiff, error)
	PostComment(ctx context.Context, owner, repo string, number int, comment *ReviewComment) error
	// PostReview creates a PR review and returns the GitHub review ID.
	PostReview(ctx context.Context, owner, repo string, number int, review *ReviewSubmission) (int64, error)
	PostSummaryComment(ctx context.Context, owner, repo string, number int, review *ReviewSubmission, score int) error
	GetCODEOWNERS(ctx context.Context, owner, repo string) ([]CODEOWNERSRule, error)
	RequestReviewers(ctx context.Context, owner, repo string, number int, reviewers []string) error
	// GetReviewCommentsByReview lists the inline comments belonging to a specific review.
	GetReviewCommentsByReview(ctx context.Context, owner, repo string, number int, reviewID int64) ([]ReviewCommentRef, error)
	// PostReviewCommentReply posts a reply inside an existing review comment thread.
	PostReviewCommentReply(ctx context.Context, owner, repo string, number int, inReplyTo int64, body string) error
	// GetFileContent fetches a single file's content from the repository.
	GetFileContent(ctx context.Context, owner, repo, path string) (string, error)
	// EnsureLabel creates a label if it doesn't exist; no-op if already present.
	EnsureLabel(ctx context.Context, owner, repo, name, color, description string) error
	// AddLabelsToIssue applies labels to a PR/issue by number.
	AddLabelsToIssue(ctx context.Context, owner, repo string, number int, labels []string) error
	// GetRepoTreeEntries fetches the recursive file tree for a commit SHA, returning only blob entries.
	GetRepoTreeEntries(ctx context.Context, owner, repo, sha string) ([]TreeEntry, error)
	// GetDefaultBranchSHA returns the HEAD commit SHA of the repository's default branch.
	GetDefaultBranchSHA(ctx context.Context, owner, repo string) (string, error)
	// CreateStatus posts a commit status (used for branch-protection required checks).
	CreateStatus(ctx context.Context, owner, repo, sha string, status *CommitStatus) error
	// ListRepoCollaborators returns the GitHub logins of all users with access to the repo.
	ListRepoCollaborators(ctx context.Context, owner, repo string) ([]string, error)
}

// implements the Client interface.
type clientImpl struct {
	client *github.Client
}

// creates a new GitHub client.
func NewClient(token string) Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &clientImpl{
		client: github.NewClient(tc),
	}
}

// fetching the pull request
func (c *clientImpl) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	start := time.Now()
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	logger.ExternalCall(ctx, "github", "PullRequests.Get", start, err, "owner", owner, "repo", repo, "pr", number)
	if err != nil {
		return nil, err
	}

	return &PullRequest{
		ID:     pr.GetID(),
		Number: pr.GetNumber(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
		State:  pr.GetState(),
		Base: GitRef{
			Ref: pr.GetBase().GetRef(),
			Sha: pr.GetBase().GetSHA(),
		},
		Head: GitRef{
			Ref: pr.GetHead().GetRef(),
			Sha: pr.GetHead().GetSHA(),
		},
		Author: User{
			Login: pr.GetUser().GetLogin(),
			ID:    pr.GetUser().GetID(),
		},
		CreatedAt: pr.GetCreatedAt().Time,
		UpdatedAt: pr.GetUpdatedAt().Time,
	}, nil
}

// fetching the pull request diff
func (c *clientImpl) GetPullRequestDiff(ctx context.Context, owner, repo string, number int) ([]FileDiff, error) {
	// Strategy: Use ListFiles to get file metadata and Patch content.
	// This is robust for most PRs and provides structured data (filename, status, patch).
	// Note: Large diffs might be truncated by GitHub API. For extremely large PRs,
	// we might need to fallback to fetching the raw diff, but ListFiles is preferred for AI context.

	opts := &github.ListOptions{PerPage: 100}
	var allFiles []*github.CommitFile
	for {
		start := time.Now()
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		logger.ExternalCall(ctx, "github", "PullRequests.ListFiles", start, err, "owner", owner, "repo", repo, "pr", number, "page", opts.Page)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	var fileDiffs []FileDiff
	for _, f := range allFiles {
		// specific file handling can be added here (e.g., ignore if Patch is empty/nil for binaries)
		patch := f.GetPatch()
		if patch == "" && f.GetStatus() != "removed" {
			// Skip files with no patch (likely binary or too large) unless they are removed
			continue
		}

		fileDiffs = append(fileDiffs, FileDiff{
			Filename:  f.GetFilename(),
			Status:    f.GetStatus(),
			Patch:     patch,
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
		})
	}

	return fileDiffs, nil
}

func (c *clientImpl) PostReview(ctx context.Context, owner, repo string, number int, review *ReviewSubmission) (int64, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "github.post_review")
	defer span.End()
	span.SetAttributes(
		attribute.String("github.owner", owner),
		attribute.String("github.repo", repo),
		attribute.Int("github.pr", number),
	)

	getStart := time.Now()
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	logger.ExternalCall(ctx, "github", "PullRequests.Get", getStart, err, "owner", owner, "repo", repo, "pr", number)
	if err != nil {
		return 0, fmt.Errorf("could not get PR ref for review: %w", err)
	}

	var draftComments []*github.DraftReviewComment
	for i := range review.Comments {
		rc := &review.Comments[i]
		line := rc.Line
		side := rc.Side
		if side == "" {
			side = "RIGHT"
		}
		draftComments = append(draftComments, &github.DraftReviewComment{
			Path: &rc.Path,
			Body: &rc.Body,
			Line: &line,
			Side: &side,
		})
	}

	event := review.Event
	commitID := pr.Head.GetSHA()
	crStart := time.Now()
	ghReview, _, err := c.client.PullRequests.CreateReview(ctx, owner, repo, number, &github.PullRequestReviewRequest{
		CommitID: &commitID,
		Body:     &review.Body,
		Event:    &event,
		Comments: draftComments,
	})
	logger.ExternalCall(ctx, "github", "PullRequests.CreateReview", crStart, err, "owner", owner, "repo", repo, "pr", number, "event", event, "comments", len(draftComments))
	if err == nil {
		return ghReview.GetID(), nil
	}

	if is422(err) {
		body := buildBodyWithComments(review)
		if isSelfApprovalError(err) {
			event = "COMMENT"
		}
		retryStart := time.Now()
		ghReview, _, err = c.client.PullRequests.CreateReview(ctx, owner, repo, number, &github.PullRequestReviewRequest{
			CommitID: &commitID,
			Body:     &body,
			Event:    &event,
		})
		logger.ExternalCall(ctx, "github", "PullRequests.CreateReview(retry)", retryStart, err, "owner", owner, "repo", repo, "pr", number, "event", event)
		if err == nil {
			return ghReview.GetID(), nil
		}
	}
	return 0, err
}

func (c *clientImpl) GetReviewCommentsByReview(ctx context.Context, owner, repo string, number int, reviewID int64) ([]ReviewCommentRef, error) {
	opts := &github.ListOptions{PerPage: 100}
	var all []ReviewCommentRef
	for {
		start := time.Now()
		comments, resp, err := c.client.PullRequests.ListReviewComments(ctx, owner, repo, number, reviewID, opts)
		logger.ExternalCall(ctx, "github", "PullRequests.ListReviewComments", start, err, "owner", owner, "repo", repo, "pr", number, "review_id", reviewID)
		if err != nil {
			return nil, fmt.Errorf("github: list review comments: %w", err)
		}
		for _, c := range comments {
			all = append(all, ReviewCommentRef{
				ID:     c.GetID(),
				Body:   c.GetBody(),
				Author: c.GetUser().GetLogin(),
				Path:   c.GetPath(),
				Line:   c.GetLine(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func (c *clientImpl) PostReviewCommentReply(ctx context.Context, owner, repo string, number int, inReplyTo int64, body string) error {
	start := time.Now()
	_, _, err := c.client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, number, body, inReplyTo)
	logger.ExternalCall(ctx, "github", "PullRequests.CreateCommentInReplyTo", start, err, "owner", owner, "repo", repo, "pr", number, "in_reply_to", inReplyTo)
	return err
}

func is422(err error) bool {
	ghErr, ok := err.(*github.ErrorResponse)
	return ok && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusUnprocessableEntity
}

func isSelfApprovalError(err error) bool {
	ghErr, ok := err.(*github.ErrorResponse)
	if !ok {
		return false
	}
	msg := strings.ToLower(ghErr.Message)
	for _, e := range ghErr.Errors {
		if strings.Contains(strings.ToLower(e.Message), "approve your own") {
			return true
		}
	}
	return strings.Contains(msg, "approve your own")
}

func (c *clientImpl) PostSummaryComment(ctx context.Context, owner, repo string, number int, review *ReviewSubmission, score int) error {
	body := buildSummaryComment(review, score)
	start := time.Now()
	_, _, err := c.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &body,
	})
	logger.ExternalCall(ctx, "github", "Issues.CreateComment", start, err, "owner", owner, "repo", repo, "pr", number)
	return err
}

func buildSummaryComment(review *ReviewSubmission, score int) string {
	counts := map[string]int{"p0": 0, "p1": 0, "p2": 0, "p3": 0}
	for _, c := range review.Comments {
		p := c.Priority
		if p == "" {
			p = "p3"
		}
		counts[p]++
	}
	total := len(review.Comments)

	verdict := "✅ Approved"
	switch review.Event {
	case "REQUEST_CHANGES":
		verdict = "❌ Changes Requested"
	case "COMMENT":
		verdict = "💬 Commented"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## 🤖 AI Review Summary\n\n")
	fmt.Fprintf(&sb, "| Score | Verdict | 🔴 P0 | 🟠 P1 | 🟡 P2 | 🟢 P3 | Total |\n")
	fmt.Fprintf(&sb, "|-------|---------|--------|--------|--------|--------|-------|\n")
	fmt.Fprintf(&sb, "| **%d/100** | %s | %d | %d | %d | %d | %d |\n\n",
		score, verdict, counts["p0"], counts["p1"], counts["p2"], counts["p3"], total)
	if review.Body != "" {
		fmt.Fprintf(&sb, "> %s\n\n", review.Body)
	}
	if total == 0 {
		sb.WriteString("No issues found. Inline comments have the details.\n\n")
	} else {
		sb.WriteString("See the inline comments above for details on each finding.\n\n")
	}
	sb.WriteString("---\n_Powered by PR Reviewer_")
	return sb.String()
}

func buildBodyWithComments(review *ReviewSubmission) string {
	if len(review.Comments) == 0 {
		return review.Body
	}
	var sb strings.Builder
	sb.WriteString(review.Body)
	sb.WriteString("\n\n---\n\n### Inline Findings\n\n")
	for _, c := range review.Comments {
		fmt.Fprintf(&sb, "**%s** (line %d, %s): %s\n\n", c.Path, c.Line, c.Severity, c.Body)
	}
	return sb.String()
}

func (c *clientImpl) RequestReviewers(ctx context.Context, owner, repo string, number int, reviewers []string) error {
	start := time.Now()
	_, _, err := c.client.PullRequests.RequestReviewers(ctx, owner, repo, number, github.ReviewersRequest{
		Reviewers: reviewers,
	})
	logger.ExternalCall(ctx, "github", "PullRequests.RequestReviewers", start, err, "owner", owner, "repo", repo, "pr", number, "reviewers", len(reviewers))
	return err
}

func (c *clientImpl) GetFileContent(ctx context.Context, owner, repo, path string) (string, error) {
	start := time.Now()
	content, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, nil)
	logger.ExternalCall(ctx, "github", "Repositories.GetContents", start, err, "owner", owner, "repo", repo, "path", path)
	if err != nil {
		return "", err
	}
	raw, err := content.GetContent()
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (c *clientImpl) EnsureLabel(ctx context.Context, owner, repo, name, color, description string) error {
	getStart := time.Now()
	_, _, err := c.client.Issues.GetLabel(ctx, owner, repo, name)
	logger.ExternalCall(ctx, "github", "Issues.GetLabel", getStart, err, "owner", owner, "repo", repo, "label", name)
	if err == nil {
		return nil
	}
	createStart := time.Now()
	_, _, err = c.client.Issues.CreateLabel(ctx, owner, repo, &github.Label{
		Name:        github.Ptr(name),
		Color:       github.Ptr(color),
		Description: github.Ptr(description),
	})
	logger.ExternalCall(ctx, "github", "Issues.CreateLabel", createStart, err, "owner", owner, "repo", repo, "label", name)
	return err
}

func (c *clientImpl) AddLabelsToIssue(ctx context.Context, owner, repo string, number int, labels []string) error {
	start := time.Now()
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	logger.ExternalCall(ctx, "github", "Issues.AddLabelsToIssue", start, err, "owner", owner, "repo", repo, "pr", number, "labels", len(labels))
	return err
}

func (c *clientImpl) GetRepoTreeEntries(ctx context.Context, owner, repo, sha string) ([]TreeEntry, error) {
	start := time.Now()
	tree, _, err := c.client.Git.GetTree(ctx, owner, repo, sha, true)
	logger.ExternalCall(ctx, "github", "Git.GetTree", start, err, "owner", owner, "repo", repo, "sha", sha)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for _, e := range tree.Entries {
		if e.GetType() == "blob" {
			entries = append(entries, TreeEntry{
				Path: e.GetPath(),
				Type: "blob",
				Size: e.GetSize(),
			})
		}
	}
	return entries, nil
}

func (c *clientImpl) GetDefaultBranchSHA(ctx context.Context, owner, repo string) (string, error) {
	getStart := time.Now()
	r, _, err := c.client.Repositories.Get(ctx, owner, repo)
	logger.ExternalCall(ctx, "github", "Repositories.Get", getStart, err, "owner", owner, "repo", repo)
	if err != nil {
		return "", err
	}
	branchStart := time.Now()
	branch, _, err := c.client.Repositories.GetBranch(ctx, owner, repo, r.GetDefaultBranch(), 0)
	logger.ExternalCall(ctx, "github", "Repositories.GetBranch", branchStart, err, "owner", owner, "repo", repo, "branch", r.GetDefaultBranch())
	if err != nil {
		return "", err
	}
	return branch.GetCommit().GetSHA(), nil
}

func (c *clientImpl) CreateStatus(ctx context.Context, owner, repo, sha string, status *CommitStatus) error {
	if sha == "" {
		return fmt.Errorf("github: cannot post commit status without a SHA")
	}
	desc := status.Description
	if len(desc) > 140 {
		desc = desc[:140]
	}
	repoStatus := &github.RepoStatus{
		State:       github.Ptr(status.State),
		Description: github.Ptr(desc),
		Context:     github.Ptr(status.Context),
	}
	if status.TargetURL != "" {
		repoStatus.TargetURL = github.Ptr(status.TargetURL)
	}
	start := time.Now()
	_, _, err := c.client.Repositories.CreateStatus(ctx, owner, repo, sha, repoStatus)
	logger.ExternalCall(ctx, "github", "Repositories.CreateStatus", start, err, "owner", owner, "repo", repo, "sha", sha, "state", status.State)
	return err
}

func (c *clientImpl) PostComment(ctx context.Context, owner, repo string, number int, comment *ReviewComment) error {
	ghComment := &github.PullRequestComment{
		Body: &comment.Body,
		Path: &comment.Path,
		Line: &comment.Line,
		Side: &comment.Side,
	}
	// For PR review comments (on lines), we actually need to CreateComment (review comment) or CreateReview.
	// Using CreateComment for single line comments.
	// Note: 'Line' in API v3 is the line in the diff. Newer APIs use CreateReview for batched comments.
	// To keep it simple, we assume single comments on lines.
	// However, usually we need the CommitID for the comment.

	// Fetch logical PR to get Head SHA
	getStart := time.Now()
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	logger.ExternalCall(ctx, "github", "PullRequests.Get", getStart, err, "owner", owner, "repo", repo, "pr", number)
	if err != nil {
		return fmt.Errorf("could not get PR ref for comment: %w", err)
	}

	ghComment.CommitID = pr.Head.SHA // Required for review comments

	commentStart := time.Now()
	_, _, err = c.client.PullRequests.CreateComment(ctx, owner, repo, number, ghComment)
	logger.ExternalCall(ctx, "github", "PullRequests.CreateComment", commentStart, err, "owner", owner, "repo", repo, "pr", number)
	return err
}

// ListRepoCollaborators returns the GitHub logins of all users with access to the
// repository (direct collaborators plus those granted via teams). Used to scope
// which users may see a repo in the app.
func (c *clientImpl) ListRepoCollaborators(ctx context.Context, owner, repo string) ([]string, error) {
	opts := &github.ListCollaboratorsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var logins []string
	for {
		start := time.Now()
		users, resp, err := c.client.Repositories.ListCollaborators(ctx, owner, repo, opts)
		logger.ExternalCall(ctx, "github", "Repositories.ListCollaborators", start, err, "owner", owner, "repo", repo)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if login := u.GetLogin(); login != "" {
				logins = append(logins, login)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return logins, nil
}
