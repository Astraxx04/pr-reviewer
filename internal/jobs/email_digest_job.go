package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// EmailDigestJobArgs triggers digest emails for all email notification configs whose
// configured cadence matches Period ("daily" or "weekly").
type EmailDigestJobArgs struct {
	Period string `json:"period"` // daily | weekly
}

func (EmailDigestJobArgs) Kind() string { return "email_digest" }

// EmailDigestWorker aggregates recent reviews and emails a summary to configured recipients.
// Email transport (SMTP settings + from address) is resolved per channel with an
// env-default fallback via notifications.ResolveEmail.
type EmailDigestWorker struct {
	river.WorkerDefaults[EmailDigestJobArgs]

	DB  *gorm.DB
	Log *logger.Logger
}

func (w *EmailDigestWorker) Work(ctx context.Context, job *river.Job[EmailDigestJobArgs]) error {
	period := job.Args.Period
	if period == "" {
		period = "daily"
	}
	window := 24 * time.Hour
	if period == "weekly" {
		window = 7 * 24 * time.Hour
	}
	since := time.Now().Add(-window)

	var configs []models.NotificationConfig
	if err := w.DB.WithContext(ctx).
		Where("channel = ? AND enabled = true", "email").
		Find(&configs).Error; err != nil {
		return err
	}

	sent := 0
	for _, cfg := range configs {
		var ec notifications.EmailChannelConfig
		if json.Unmarshal(cfg.Config, &ec) != nil {
			continue
		}
		if ec.Digest != period || len(ec.To) == 0 {
			continue
		}

		rows := w.aggregate(ctx, cfg.RepoID, since)
		if len(rows) == 0 {
			continue // nothing to report this period
		}

		smtp, from := notifications.ResolveEmail(ec)
		subject := fmt.Sprintf("[PR Reviewer] %s digest — %d reviews", capitalize(period), len(rows))
		if err := notifications.SendEmail(ctx, smtp, from, ec.To, subject, renderDigestHTML(period, since, rows)); err != nil {
			w.Log.Error("digest email failed", "config_id", cfg.ID, "error", err)
			continue
		}
		sent++
	}
	if sent > 0 {
		w.Log.Info("digest emails sent", "period", period, "count", sent)
	}
	return nil
}

// digestRow is one review summarised for the digest.
type digestRow struct {
	Owner     string
	RepoName  string
	PRNumber  int
	Title     string
	Status    string
	Score     int
	CreatedAt time.Time
}

func (w *EmailDigestWorker) aggregate(ctx context.Context, repoID *uint, since time.Time) []digestRow {
	tx := w.DB.WithContext(ctx).
		Table("reviews").
		Select(`repositories.owner, repositories.name as repo_name, pull_requests.number as pr_number,
			pull_requests.title, reviews.status, reviews.score, reviews.created_at`).
		Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
		Joins("JOIN repositories ON repositories.id = pull_requests.repo_id").
		Where("reviews.created_at >= ?", since).
		Order("reviews.created_at desc")
	if repoID != nil {
		tx = tx.Where("repositories.id = ?", *repoID)
	}
	var rows []digestRow
	_ = tx.Scan(&rows).Error
	return rows
}

func renderDigestHTML(period string, since time.Time, rows []digestRow) string {
	var total, approvals, changes int
	perRepo := map[string]int{}
	for _, r := range rows {
		total += r.Score
		switch r.Status {
		case "APPROVE":
			approvals++
		case "REQUEST_CHANGES":
			changes++
		}
		perRepo[r.Owner+"/"+r.RepoName]++
	}
	avg := 0.0
	if len(rows) > 0 {
		avg = float64(total) / float64(len(rows))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<h2>PR Reviewer — %s digest</h2>`, capitalize(period))
	fmt.Fprintf(&sb, `<p style="color:#666">Since %s</p>`, since.Format("2006-01-02 15:04"))
	fmt.Fprintf(&sb, `<p><strong>%d</strong> reviews · avg score <strong>%.1f/100</strong> · %d approved · %d changes requested</p>`,
		len(rows), avg, approvals, changes)

	// Top repos.
	if len(perRepo) > 0 {
		repos := make([]string, 0, len(perRepo))
		for k := range perRepo {
			repos = append(repos, k)
		}
		sort.Slice(repos, func(i, j int) bool { return perRepo[repos[i]] > perRepo[repos[j]] })
		sb.WriteString(`<p><strong>By repository:</strong> `)
		parts := make([]string, 0, len(repos))
		for _, k := range repos {
			parts = append(parts, fmt.Sprintf("%s (%d)", k, perRepo[k]))
		}
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString(`</p>`)
	}

	sb.WriteString(`<table cellpadding="6" style="border-collapse:collapse;font-size:14px">`)
	sb.WriteString(`<tr style="background:#f0f0f0"><th align="left">PR</th><th align="left">Title</th><th align="left">Status</th><th align="right">Score</th></tr>`)
	limit := len(rows)
	if limit > 30 {
		limit = 30
	}
	for _, r := range rows[:limit] {
		fmt.Fprintf(&sb,
			`<tr style="border-top:1px solid #eee"><td>%s/%s#%d</td><td>%s</td><td>%s</td><td align="right">%d</td></tr>`,
			r.Owner, r.RepoName, r.PRNumber, htmlEscape(r.Title), r.Status, r.Score)
	}
	sb.WriteString(`</table>`)
	sb.WriteString(`<p style="color:#999;font-size:12px">Powered by PR Reviewer</p>`)
	return sb.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
