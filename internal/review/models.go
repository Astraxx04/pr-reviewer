package review

import "pr-reviewer/internal/github"

// FinalReview represents the aggregated review result.
type FinalReview struct {
	Comments []github.ReviewComment
	Summary  string
	Status   string // APPROVE, REQUEST_CHANGES, COMMENT
}
