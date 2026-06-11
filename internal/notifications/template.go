package notifications

import "strings"

// Event constants used in NotificationConfig.Config["events"].
const (
	EventAssignment         = "assignment"
	EventReviewComplete     = "review_complete"
	EventReReview           = "re_review"
	EventScoreBelowThreshold = "score_below_threshold"
)

// AllEvents is the canonical list of supported notification events.
var AllEvents = []string{EventAssignment, EventReviewComplete, EventReReview, EventScoreBelowThreshold}

const (
	defaultSlackAssignmentTpl = "👀 *Review requested* — @{{assignee}}, please review *<{{pr.url}}|{{pr.title}}>*\n> {{review.summary}}"
	defaultSlackReviewTpl     = "✅ Review complete for *<{{pr.url}}|{{pr.title}}>* — Score: *{{review.score}}/100*\n> {{review.summary}}"
	defaultEmailBodyTpl       = `<p>PR <a href="{{pr.url}}">{{pr.title}}</a> scored <strong>{{review.score}}/100</strong>.</p><blockquote>{{review.summary}}</blockquote><p><em>Powered by PR Reviewer</em></p>`
)

// RenderTemplate replaces {{key}} placeholders with values from vars.
func RenderTemplate(tpl string, vars map[string]string) string {
	args := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		args = append(args, "{{"+k+"}}", v)
	}
	return strings.NewReplacer(args...).Replace(tpl)
}

// OrDefault is exported so the HTTP handler can use it for test notifications.
func OrDefault(tpl, def string) string {
	if strings.TrimSpace(tpl) == "" {
		return def
	}
	return tpl
}

func hasEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}
