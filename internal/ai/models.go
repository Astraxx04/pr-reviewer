package ai

import (
	"pr-reviewer/internal/github"
)

// ReviewResult represents the outcome of an AI review.
type ReviewResult struct {
	Comments     []github.ReviewComment
	Summary      string
	Score        int // 0-100 quality score
	InputTokens  int
	OutputTokens int
}

// AgentConfig holds per-agent provider overrides stored in Repository.Config.
type AgentConfig struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	// Enabled opts an optional agent into the review fan-out. It is ignored for
	// core agents (code-review, security), which always run.
	Enabled bool `json:"enabled"`
}

// AnalysisRequest represents the input to the AI reviewer.
type AnalysisRequest struct {
	Diff       []github.FileDiff
	Title      string
	Body       string
	RepoID     uint                   // used to scope RAG retrieval; 0 disables RAG
	RepoConfig map[string]AgentConfig // agent name → provider+model override
	PRContext  interface{}            // broader context (tickets, docs, etc.)

	TicketContext string // formatted Jira ticket summaries for injection into the prompt

	// New fields for Section 8 features:
	FalsePositivePatterns []string // comment bodies previously marked as false positives
	CustomViolations      []string // pre-formatted violations from .pr-reviewer.yml
	DiffTruncated         bool     // true if diff exceeded max_diff_lines and was dropped
	PRTemplate            string   // content of .github/pull_request_template.md
	ConsensusThreshold    int      // 0=disabled; N=require N agents to agree for p2/p3
}
