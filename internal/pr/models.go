package pr

import (
	"github.com/Astraxx04/pr-reviewer/internal/github"
	"time"
)

// PRContext contains all necessary context for a PR review.
// This object will flow through the entire AI system (Reviewer, RAG, Agents, Evaluation).
type PRContext struct {
	Repo      string // "owner/repo"
	Number    int
	Title     string
	Body      string
	Action    string
	Timestamp time.Time

	// Detailed Github Data (kept for internal use if needed)
	PR   *github.PullRequest
	Diff []github.FileDiff
}
