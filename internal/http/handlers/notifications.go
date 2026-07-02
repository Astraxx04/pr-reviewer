package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
)

type NotificationHandler struct {
	db       *gorm.DB
	enqueuer jobs.JobEnqueuer // optional: enables on-demand digest triggering
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// WithEnqueuer wires the River client so admins can trigger a digest on demand.
func (h *NotificationHandler) WithEnqueuer(e jobs.JobEnqueuer) *NotificationHandler {
	h.enqueuer = e
	return h
}

// TriggerDigest enqueues an email-digest job immediately for the given period
// (?period=daily|weekly, default daily). Used by the "Send digest now" button.
func (h *NotificationHandler) TriggerDigest(w http.ResponseWriter, r *http.Request) {
	if h.enqueuer == nil {
		writeError(w, http.StatusServiceUnavailable, "job queue not available")
		return
	}
	period := r.URL.Query().Get("period")
	if period != "weekly" {
		period = "daily"
	}
	if _, err := h.enqueuer.Insert(r.Context(), jobs.EmailDigestJobArgs{Period: period}, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue digest")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "period": period})
}

// List returns all notification configs.
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	var cfgs []models.NotificationConfig
	if err := h.db.WithContext(r.Context()).
		Order("id ASC").
		Find(&cfgs).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	for i := range cfgs {
		cfgs[i].Config = datatypes.JSON(notifications.RedactConfig(cfgs[i].Channel, cfgs[i].Config))
	}
	writeJSON(w, http.StatusOK, cfgs)
}

type notifConfigInput struct {
	Channel string          `json:"channel"`
	RepoID  *uint           `json:"repo_id,omitempty"`
	Config  json.RawMessage `json:"config"` // decoded into datatypes.JSON on use
	Enabled *bool           `json:"enabled,omitempty"`
}

// Create adds a new notification config.
func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input notifConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Channel != "slack" && input.Channel != "email" && input.Channel != "webhook" {
		writeError(w, http.StatusBadRequest, "channel must be slack, email, or webhook")
		return
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	stored, err := notifications.PrepareConfigForStore(input.Channel, input.Config, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid config")
		return
	}
	cfg := models.NotificationConfig{
		RepoID:    input.RepoID,
		Channel:   input.Channel,
		Config:    datatypes.JSON(stored),
		Enabled:   enabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.db.WithContext(r.Context()).Create(&cfg).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	cfg.Config = datatypes.JSON(notifications.RedactConfig(cfg.Channel, cfg.Config))
	user := getUser(r)
	audit.Log(h.db, r, user.Login, user.ID, "config.notification_created", "config", fmt.Sprint(cfg.ID),
		nil, map[string]any{"channel": cfg.Channel})
	writeJSON(w, http.StatusCreated, cfg)
}

// Update replaces a notification config.
func (h *NotificationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var existing models.NotificationConfig
	if err := h.db.WithContext(r.Context()).First(&existing, id).Error; err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var input notifConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	oldConfig := existing.Config // already-encrypted; used to preserve blank secrets
	if input.Channel != "" {
		if input.Channel != "slack" && input.Channel != "email" && input.Channel != "webhook" {
			writeError(w, http.StatusBadRequest, "channel must be slack, email, or webhook")
			return
		}
		existing.Channel = input.Channel
	}
	if len(input.Config) > 0 {
		stored, err := notifications.PrepareConfigForStore(existing.Channel, input.Config, oldConfig)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid config")
			return
		}
		existing.Config = datatypes.JSON(stored)
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	existing.UpdatedAt = time.Now()
	if err := h.db.WithContext(r.Context()).Save(&existing).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	existing.Config = datatypes.JSON(notifications.RedactConfig(existing.Channel, existing.Config))
	user := getUser(r)
	audit.Log(h.db, r, user.Login, user.ID, "config.notification_updated", "config", fmt.Sprint(existing.ID),
		nil, map[string]any{"channel": existing.Channel})
	writeJSON(w, http.StatusOK, existing)
}

// Delete removes a notification config.
func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	result := h.db.WithContext(r.Context()).Delete(&models.NotificationConfig{}, id)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	user := getUser(r)
	audit.Log(h.db, r, user.Login, user.ID, "config.notification_deleted", "config", fmt.Sprint(id), nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

// Test sends a test notification for the given config.
func (h *NotificationHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var cfg models.NotificationConfig
	if err := h.db.WithContext(r.Context()).First(&cfg, id).Error; err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// For an email channel with no recipients configured (assignment emails
	// auto-route to the assignee), send the test to the admin who clicked Test.
	var testerEmail string
	if u := getUser(r); u != nil {
		var dbUser models.User
		if h.db.WithContext(r.Context()).First(&dbUser, u.ID).Error == nil {
			testerEmail = dbUser.Email
		}
	}

	err := sendTestNotification(r.Context(), cfg, testerEmail)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// sendTestNotification sends a sample notification for the given channel.
// fallbackEmail is used as the recipient for email channels that have no
// recipients configured (e.g. when relying on per-assignee auto-routing).
func sendTestNotification(ctx context.Context, cfg models.NotificationConfig, fallbackEmail string) error {
	vars := map[string]string{
		"assignee":       "octocat",
		"pr.title":       "Test PR — notification check",
		"pr.url":         "https://github.com",
		"review.summary": "This is a test notification from PR Reviewer.",
		"review.score":   "85",
	}
	switch cfg.Channel {
	case "slack":
		var sc notifications.SlackChannelConfig
		if err := json.Unmarshal(cfg.Config, &sc); err != nil || sc.WebhookURL == "" {
			return &testErr{"invalid slack config: missing webhook_url"}
		}
		tpl := notifications.OrDefault(sc.Template, "🔔 *Test notification* from PR Reviewer — config ID working correctly.")
		return notifications.PostSlack(ctx, sc.WebhookURL, notifications.RenderTemplate(tpl, vars))
	case "email":
		var ec notifications.EmailChannelConfig
		if err := json.Unmarshal(cfg.Config, &ec); err != nil {
			return &testErr{"invalid email config"}
		}
		to := ec.To
		if len(to) == 0 && fallbackEmail != "" {
			to = []string{fallbackEmail} // send the test to the admin running it
		}
		if len(to) == 0 {
			return &testErr{"no recipients configured, and your account has no email on file to send a test to — add a recipient or sign in again to refresh your email"}
		}
		smtp, from := notifications.ResolveEmail(ec)
		subject := "PR Reviewer — test notification"
		body := notifications.RenderTemplate(notifications.OrDefault(ec.Template, "<p>This is a test notification from <strong>PR Reviewer</strong>.</p>"), vars)
		return notifications.SendEmail(ctx, smtp, from, to, subject, body)
	case "webhook":
		var wc notifications.WebhookChannelConfig
		if err := json.Unmarshal(cfg.Config, &wc); err != nil || wc.URL == "" {
			return &testErr{"invalid webhook config: missing url"}
		}
		return notifications.PostWebhook(ctx, wc.URL, notifications.DecryptSecret(wc.Secret), notifications.WebhookPayload{
			Event:     "test",
			PR:        map[string]any{"title": "Test PR", "url": "https://github.com"},
			Timestamp: time.Now(),
		})
	}
	return &testErr{"unknown channel"}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
