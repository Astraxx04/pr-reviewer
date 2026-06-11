package github

import "time"

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	ID        int64     `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Draft     bool      `json:"draft"`
	Base      GitRef    `json:"base"`
	Head      GitRef    `json:"head"`
	Author    User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GitRef represents a Git reference (branch/commit).
type GitRef struct {
	Ref string `json:"ref"`
	Sha string `json:"sha"`
}

// User represents a GitHub user.
type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// FileDiff represents the difference in a file.
type FileDiff struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"` // added, modified, removed
	Patch     string `json:"patch"`  // The diff patch
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// ReviewComment represents a comment to be posted on a PR.
type ReviewComment struct {
	Path     string `json:"path"`
	Body     string `json:"body"`
	Line     int    `json:"line"`
	Side     string `json:"side"`     // LEFT or RIGHT
	Severity string `json:"severity"` // kept for backward compat; prefer Priority
	Priority string `json:"priority"` // p0 | p1 | p2 | p3
}

// ReviewSubmission is the payload for a batched PR review.
type ReviewSubmission struct {
	Body     string // overall summary
	Event    string // APPROVE | REQUEST_CHANGES | COMMENT
	Comments []ReviewComment
}

// ReviewCommentRef is a lightweight reference to an existing GitHub review comment.
type ReviewCommentRef struct {
	ID     int64
	Body   string
	Author string
	Path   string
	Line   int
}

// CommitStatus is a commit status to post for branch-protection / required-check use.
type CommitStatus struct {
	State       string // pending | success | failure | error
	Description string // short human-readable summary (max 140 chars)
	Context     string // unique label for this check, e.g. "pr-reviewer"
	TargetURL   string // link to the full review
}

// TreeEntry represents a single entry from a GitHub repository tree (blobs only).
type TreeEntry struct {
	Path string
	Type string // "blob"
	Size int
}
