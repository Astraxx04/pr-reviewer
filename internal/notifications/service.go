package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// SlackChannelConfig is the JSON shape stored in NotificationConfig.Config for channel="slack".
type SlackChannelConfig struct {
	WebhookURL     string   `json:"webhook_url"`
	Events         []string `json:"events"`
	ScoreThreshold int      `json:"score_threshold"` // 0 = always notify
	Template       string   `json:"template"`
}

// EmailChannelConfig is the JSON shape for channel="email". SMTP fields are
// optional per channel; blank fields fall back to the server's env defaults
// (see ResolveEmail).
type EmailChannelConfig struct {
	SMTPHost       string   `json:"smtp_host,omitempty"`
	SMTPPort       int      `json:"smtp_port,omitempty"`
	SMTPUsername   string   `json:"smtp_username,omitempty"`
	SMTPPassword   string   `json:"smtp_password,omitempty"`
	From           string   `json:"from,omitempty"`
	To             []string `json:"to"`
	Events         []string `json:"events"`
	Digest         string   `json:"digest"` // none | daily | weekly
	Template       string   `json:"template"`
	ScoreThreshold int      `json:"score_threshold"` // threshold for the score_below_threshold event (0 = disabled)
}

// WebhookChannelConfig is the JSON shape for channel="webhook".
type WebhookChannelConfig struct {
	URL            string   `json:"url"`
	Secret         string   `json:"secret,omitempty"`
	Events         []string `json:"events"`
	Template       string   `json:"template"`
	ScoreThreshold int      `json:"score_threshold"` // threshold for the score_below_threshold event (0 = disabled)
}

// Service dispatches notifications for review events.
type Service interface {
	NotifyAssignment(ctx context.Context, repoID uint, assignee, prTitle, prURL, summary string)
	NotifyReviewComplete(ctx context.Context, repoID uint, prTitle, prURL, summary string, score int, isReReview bool)
}

// shouldNotifyComplete decides whether a channel subscribed to the given events
// should fire for a completed review. A channel is notified if it subscribes to
// review_complete, OR to re_review when this run was a re-review, OR to
// score_below_threshold when a threshold is set and the score falls below it.
func shouldNotifyComplete(events []string, threshold, score int, isReReview bool) bool {
	if hasEvent(events, EventReviewComplete) {
		return true
	}
	if isReReview && hasEvent(events, EventReReview) {
		return true
	}
	if threshold > 0 && score < threshold && hasEvent(events, EventScoreBelowThreshold) {
		return true
	}
	return false
}

type notifService struct {
	db       *gorm.DB
	userRepo *repo.UserRepo
}

func NewService(db *gorm.DB) Service {
	return &notifService{
		db:       db,
		userRepo: repo.NewUserRepo(db),
	}
}

func (s *notifService) loadConfigs(ctx context.Context, repoID uint) []models.NotificationConfig {
	if repoID == 0 {
		return nil
	}
	var cfgs []models.NotificationConfig
	s.db.WithContext(ctx).
		Where("enabled = true AND (repo_id IS NULL OR repo_id = ?)", repoID).
		Order("repo_id ASC NULLS FIRST").
		Find(&cfgs)
	return cfgs
}

func (s *notifService) NotifyAssignment(ctx context.Context, repoID uint, assignee, prTitle, prURL, summary string) {
	vars := map[string]string{
		"assignee": assignee, "pr.title": prTitle, "pr.url": prURL,
		"review.summary": summary, "review.score": "",
	}
	for _, cfg := range s.loadConfigs(ctx, repoID) {
		switch cfg.Channel {
		case "slack":
			var sc SlackChannelConfig
			if json.Unmarshal(cfg.Config, &sc) == nil && sc.WebhookURL != "" && hasEvent(sc.Events, EventAssignment) {
				_ = PostSlack(ctx, sc.WebhookURL, RenderTemplate(OrDefault(sc.Template, defaultSlackAssignmentTpl), vars))
			}
		case "email":
			var ec EmailChannelConfig
			if json.Unmarshal(cfg.Config, &ec) == nil && hasEvent(ec.Events, EventAssignment) {
				to := append([]string{}, ec.To...)
				if u, err := s.userRepo.FindByLogin(ctx, assignee); err == nil && u.Email != "" {
					to = append(to, u.Email)
				}
				if len(to) > 0 {
					smtp, from := ResolveEmail(ec)
					_ = SendEmail(ctx, smtp, from, to,
						fmt.Sprintf("Review requested: %s", prTitle),
						RenderTemplate(OrDefault(ec.Template, defaultSlackAssignmentTpl), vars))
				}
			}
		case "webhook":
			var wc WebhookChannelConfig
			if json.Unmarshal(cfg.Config, &wc) == nil && wc.URL != "" && hasEvent(wc.Events, EventAssignment) {
				_ = PostWebhook(ctx, wc.URL, DecryptSecret(wc.Secret), WebhookPayload{
					Event: EventAssignment, Assignees: []string{assignee},
					PR: map[string]any{"title": prTitle, "url": prURL}, Timestamp: time.Now(),
				})
			}
		}
	}
}

func (s *notifService) NotifyReviewComplete(ctx context.Context, repoID uint, prTitle, prURL, summary string, score int, isReReview bool) {
	vars := map[string]string{
		"pr.title": prTitle, "pr.url": prURL,
		"review.summary": summary, "review.score": fmt.Sprint(score), "assignee": "",
	}
	for _, cfg := range s.loadConfigs(ctx, repoID) {
		switch cfg.Channel {
		case "slack":
			var sc SlackChannelConfig
			if json.Unmarshal(cfg.Config, &sc) == nil && sc.WebhookURL != "" &&
				shouldNotifyComplete(sc.Events, sc.ScoreThreshold, score, isReReview) {
				_ = PostSlack(ctx, sc.WebhookURL, RenderTemplate(OrDefault(sc.Template, defaultSlackReviewTpl), vars))
			}
		case "email":
			var ec EmailChannelConfig
			if json.Unmarshal(cfg.Config, &ec) == nil && len(ec.To) > 0 &&
				shouldNotifyComplete(ec.Events, ec.ScoreThreshold, score, isReReview) {
				smtp, from := ResolveEmail(ec)
				_ = SendEmail(ctx, smtp, from, ec.To,
					fmt.Sprintf("[PR Reviewer] %s — %d/100", prTitle, score),
					RenderTemplate(OrDefault(ec.Template, defaultEmailBodyTpl), vars))
			}
		case "webhook":
			var wc WebhookChannelConfig
			if json.Unmarshal(cfg.Config, &wc) == nil && wc.URL != "" &&
				shouldNotifyComplete(wc.Events, wc.ScoreThreshold, score, isReReview) {
				_ = PostWebhook(ctx, wc.URL, DecryptSecret(wc.Secret), WebhookPayload{
					Event: EventReviewComplete, Score: score,
					PR:        map[string]any{"title": prTitle, "url": prURL},
					Review:    map[string]any{"summary": summary, "score": score},
					Timestamp: time.Now(),
				})
			}
		}
	}
}

func PostSlack(ctx context.Context, webhookURL, text string) error {
	body, _ := json.Marshal(map[string]any{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	logger.ExternalCall(ctx, "slack-webhook", "POST", start, err)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}
