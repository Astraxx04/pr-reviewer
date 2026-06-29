package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	slackint "github.com/Astraxx04/pr-reviewer/internal/integration/slack"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// SlackAppHandler serves both the admin config endpoints (JWT-protected) and the
// public inbound Slack endpoints (slash commands + Events API), which authenticate
// via the Slack request signature instead of a JWT.
type SlackAppHandler struct {
	db        *gorm.DB
	encKey    string
	enqueuer  jobs.JobEnqueuer // may be nil
	log       *logger.Logger
	serverURL string // public base URL Slack must reach (from SERVER_URL)
}

func NewSlackAppHandler(db *gorm.DB, encKey, serverURL string, enqueuer jobs.JobEnqueuer, log *logger.Logger) *SlackAppHandler {
	return &SlackAppHandler{db: db, encKey: encKey, serverURL: serverURL, enqueuer: enqueuer, log: log}
}

// --- admin config endpoints (JWT) ---

// Get returns the Slack App config status, never the secrets themselves.
func (h *SlackAppHandler) Get(w http.ResponseWriter, r *http.Request) {
	var cfg models.SlackAppConfig
	if err := h.db.WithContext(r.Context()).First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			_ = json.NewEncoder(w).Encode(map[string]any{"configured": false, "server_url": h.serverURL})
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"configured":      true,
		"enabled":         cfg.Enabled,
		"has_bot_token":   cfg.BotTokenEncrypted != "",
		"has_signing_key": cfg.SigningSecretEncrypted != "",
		"created_at":      cfg.CreatedAt,
		"updated_at":      cfg.UpdatedAt,
		"server_url":      h.serverURL,
	})
}

// Put creates or updates the Slack App config. Empty secret fields keep the existing value.
func (h *SlackAppHandler) Put(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SigningSecret string `json:"signing_secret"`
		BotToken      string `json:"bot_token"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var cfg models.SlackAppConfig
	h.db.WithContext(r.Context()).First(&cfg) // existing or zero-value

	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	} else if cfg.ID == 0 {
		cfg.Enabled = true
	}

	if body.SigningSecret != "" {
		enc, err := db.Encrypt(body.SigningSecret, h.encKey)
		if err != nil {
			http.Error(w, "encryption failed", http.StatusInternalServerError)
			return
		}
		cfg.SigningSecretEncrypted = enc
	} else if cfg.SigningSecretEncrypted == "" {
		http.Error(w, "signing_secret is required for a new configuration", http.StatusBadRequest)
		return
	}

	if body.BotToken != "" {
		enc, err := db.Encrypt(body.BotToken, h.encKey)
		if err != nil {
			http.Error(w, "encryption failed", http.StatusInternalServerError)
			return
		}
		cfg.BotTokenEncrypted = enc
	}

	if err := h.db.WithContext(r.Context()).Save(&cfg).Error; err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete removes the Slack App config.
func (h *SlackAppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.db.WithContext(r.Context()).Delete(&models.SlackAppConfig{}, "1=1").Error; err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test verifies the stored bot token via Slack's auth.test.
func (h *SlackAppHandler) Test(w http.ResponseWriter, r *http.Request) {
	_, botToken, _, err := h.loadConfig(r.Context())
	if err != nil {
		http.Error(w, "Slack not configured", http.StatusBadRequest)
		return
	}
	info, err := slackint.TestAuth(r.Context(), botToken)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"team":    info.Team,
		"team_id": info.TeamID,
		"user":    info.User,
		"bot_id":  info.BotID,
		"url":     info.URL,
	})
}

// --- public inbound endpoints (Slack signature auth) ---

// HandleCommand processes slash commands (/review, /review-status). It must verify the
// Slack signature against the raw body before parsing the form.
func (h *SlackAppHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	raw, ok := h.verified(w, r)
	if !ok {
		return
	}
	form, err := url.ParseQuery(string(raw))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	command := form.Get("command")
	text := form.Get("text")
	responseURL := form.Get("response_url")

	ref, found := slackint.ParsePRRef(text)
	if !found {
		writeSlack(w, fmt.Sprintf("Usage: `%s owner/repo#123`", command))
		return
	}

	switch {
	case strings.Contains(command, "status"):
		writeSlack(w, h.latestReviewText(r.Context(), ref))
	default: // /review
		if h.enqueuer == nil {
			writeSlack(w, "⚠️ Reviews are not available (no job queue configured).")
			return
		}
		_, err := h.enqueuer.Insert(r.Context(), jobs.ReviewJobArgs{
			Owner:            ref.Owner,
			Repo:             ref.Repo,
			Number:           ref.Number,
			Action:           "slack",
			SlackResponseURL: responseURL,
		}, nil)
		if err != nil {
			if h.log != nil {
				h.log.Error("slack: enqueue review failed", "error", err)
			}
			writeSlack(w, "⚠️ Failed to queue the review. Please try again.")
			return
		}
		writeSlack(w, fmt.Sprintf("⏳ Review queued for *%s/%s#%d* — results will post to the PR and back here.", ref.Owner, ref.Repo, ref.Number))
	}
}

// HandleEvents processes the Slack Events API: the url_verification handshake and
// app_mention events (which trigger a re-review of any referenced PR).
func (h *SlackAppHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// The URL-verification handshake is sent before the signing secret is necessarily
	// in use, but we still verify when possible. Peek at the type first.
	var probe struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	_ = json.Unmarshal(raw, &probe)

	signingSecret, botToken, enabled, cfgErr := h.loadConfig(r.Context())
	if cfgErr == nil {
		if err := slackint.VerifySignature(signingSecret,
			r.Header.Get("X-Slack-Request-Timestamp"), raw,
			r.Header.Get("X-Slack-Signature"), time.Now()); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	if probe.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": probe.Challenge})
		return
	}

	// Acknowledge immediately; do the work without blocking Slack's 3s deadline.
	w.WriteHeader(http.StatusOK)

	if cfgErr != nil || !enabled {
		return
	}

	var evt struct {
		Event struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Channel string `json:"channel"`
		} `json:"event"`
	}
	if json.Unmarshal(raw, &evt) != nil || evt.Event.Type != "app_mention" {
		return
	}
	ref, found := slackint.ParsePRRef(evt.Event.Text)
	if !found || h.enqueuer == nil {
		return
	}
	bg := context.Background()
	if _, err := h.enqueuer.Insert(bg, jobs.ReviewJobArgs{
		Owner: ref.Owner, Repo: ref.Repo, Number: ref.Number, Action: "slack_mention",
	}, nil); err != nil {
		if h.log != nil {
			h.log.Error("slack: enqueue from mention failed", "error", err)
		}
		return
	}
	if botToken != "" && evt.Event.Channel != "" {
		_ = slackint.PostMessage(bg, botToken, evt.Event.Channel,
			fmt.Sprintf("⏳ Re-review queued for *%s/%s#%d*.", ref.Owner, ref.Repo, ref.Number))
	}
}

// --- helpers ---

// verified reads the raw body and validates the Slack signature; on failure it writes
// the HTTP error and returns ok=false.
func (h *SlackAppHandler) verified(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return nil, false
	}
	signingSecret, _, enabled, cfgErr := h.loadConfig(r.Context())
	if cfgErr != nil || !enabled {
		http.Error(w, "slack not configured", http.StatusServiceUnavailable)
		return nil, false
	}
	if err := slackint.VerifySignature(signingSecret,
		r.Header.Get("X-Slack-Request-Timestamp"), raw,
		r.Header.Get("X-Slack-Signature"), time.Now()); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return nil, false
	}
	return raw, true
}

// loadConfig returns the decrypted signing secret, bot token, and enabled flag.
func (h *SlackAppHandler) loadConfig(ctx context.Context) (signingSecret, botToken string, enabled bool, err error) {
	var cfg models.SlackAppConfig
	if err = h.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return "", "", false, err
	}
	if cfg.SigningSecretEncrypted != "" {
		signingSecret, _ = db.Decrypt(cfg.SigningSecretEncrypted, h.encKey)
	}
	if cfg.BotTokenEncrypted != "" {
		botToken, _ = db.Decrypt(cfg.BotTokenEncrypted, h.encKey)
	}
	return signingSecret, botToken, cfg.Enabled, nil
}

// latestReviewText formats the most recent review for a PR as a Slack message.
func (h *SlackAppHandler) latestReviewText(ctx context.Context, ref slackint.PRRef) string {
	var row struct {
		Status    string
		Score     int
		Summary   string
		CreatedAt time.Time
	}
	err := h.db.WithContext(ctx).
		Table("reviews").
		Select("reviews.status, reviews.score, reviews.summary, reviews.created_at").
		Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
		Joins("JOIN repositories ON repositories.id = pull_requests.repo_id").
		Where("repositories.owner = ? AND repositories.name = ? AND pull_requests.number = ?", ref.Owner, ref.Repo, ref.Number).
		Order("reviews.created_at DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil || row.CreatedAt.IsZero() {
		return fmt.Sprintf("No review found yet for *%s/%s#%d*.", ref.Owner, ref.Repo, ref.Number)
	}
	return fmt.Sprintf("*%s/%s#%d* — %s · score *%d/100*\n> %s",
		ref.Owner, ref.Repo, ref.Number, verdictEmoji(row.Status), row.Score, truncate(row.Summary, 280))
}

func verdictEmoji(status string) string {
	switch status {
	case "REQUEST_CHANGES":
		return "❌ Changes requested"
	case "COMMENT":
		return "💬 Commented"
	case "APPROVE":
		return "✅ Approved"
	default:
		return status
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// writeSlack writes an ephemeral slash-command response.
func writeSlack(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"response_type": "ephemeral",
		"text":          text,
	})
}
