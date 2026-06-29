package review

import "github.com/Astraxx04/pr-reviewer/internal/github"

// FinalReview represents the aggregated review result.
type FinalReview struct {
	Comments []github.ReviewComment
	Summary  string
	Status   string // APPROVE, REQUEST_CHANGES, COMMENT
}
