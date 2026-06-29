package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
)

// JobEnqueuer is the subset of river.Client the handler needs.
type JobEnqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type TeamHandler struct {
	db          *gorm.DB
	enqueuer    JobEnqueuer // optional; nil disables manual sync trigger
	frontendURL string
}

func NewTeamHandler(db *gorm.DB, enqueuer JobEnqueuer, frontendURL string) *TeamHandler {
	return &TeamHandler{db: db, enqueuer: enqueuer, frontendURL: frontendURL}
}

// installationIDForUser returns the installation ID for the given user.
// It first tries to find an installation owned by the user's login (personal app),
// then falls back to the first installation in the DB (org-level app or single-tenant deploy).
func installationIDForUser(db *gorm.DB, login string) uint {
	var inst models.Installation
	if db.Where("account_login = ?", login).First(&inst).Error == nil {
		return inst.ID
	}
	// Single-tenant fallback: return the first installation regardless of owner.
	db.Order("id ASC").First(&inst)
	return inst.ID
}

func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	instID := installationIDForUser(h.db, user.Login)
	var members []models.TeamMember
	h.db.WithContext(r.Context()).Where("installation_id = ?", instID).Find(&members)
	writeJSON(w, http.StatusOK, members)
}

func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Login string `json:"login"`
		Role  string `json:"role"`
		Email string `json:"email"` // optional: used for invite email
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Login == "" {
		writeError(w, http.StatusBadRequest, "login required")
		return
	}
	if body.Role == "" {
		body.Role = "reviewer"
	}
	instID := installationIDForUser(h.db, user.Login)
	member := models.TeamMember{
		InstallationID: instID,
		Login:          body.Login,
		Role:           body.Role,
	}
	if err := h.db.WithContext(r.Context()).Create(&member).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}
	writeJSON(w, http.StatusCreated, member)
	audit.Log(h.db, r, user.Login, user.ID, "team_member.add", "team_member",
		body.Login, nil, map[string]any{"login": body.Login, "role": body.Role})

	// Fire-and-forget invite notifications.
	go h.sendInviteNotifications(r.Context(), instID, body.Login, body.Role, body.Email, user.Login)
}

func (h *TeamHandler) sendInviteNotifications(ctx context.Context, instID uint, login, role, email, addedBy string) {
	loginURL := h.frontendURL + "/login"
	message := fmt.Sprintf("👋 *%s* has been added to PR Reviewer as *%s* by *%s*.\nThey can sign in at: %s", login, role, addedBy, loginURL)

	var cfgs []models.NotificationConfig
	h.db.WithContext(ctx).
		Where("installation_id = ? AND enabled = true", instID).
		Find(&cfgs)

	for _, cfg := range cfgs {
		switch cfg.Channel {
		case "slack":
			var sc notifications.SlackChannelConfig
			if err := json.Unmarshal(cfg.Config, &sc); err == nil && sc.WebhookURL != "" {
				_ = notifications.PostSlack(ctx, sc.WebhookURL, message)
			}
		case "email":
			if email == "" {
				continue
			}
			var ec notifications.EmailChannelConfig
			if err := json.Unmarshal(cfg.Config, &ec); err != nil {
				continue
			}
			smtp, from := notifications.ResolveEmail(ec)
			subject := fmt.Sprintf("You've been invited to PR Reviewer as %s", role)
			body := fmt.Sprintf("<p>Hi <strong>%s</strong>,</p><p><strong>%s</strong> has added you to PR Reviewer as <strong>%s</strong>.</p><p><a href=\"%s\">Sign in with GitHub</a> to get started.</p>", login, addedBy, role, loginURL)
			_ = notifications.SendEmail(ctx, smtp, from, []string{email}, subject, body)
		}
	}
}

func (h *TeamHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Role == "" {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}
	validRoles := map[string]bool{"admin": true, "reviewer": true, "viewer": true}
	if !validRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin|reviewer|viewer")
		return
	}
	instID := installationIDForUser(h.db, user.Login)
	if err := h.db.WithContext(r.Context()).
		Model(&models.TeamMember{}).
		Where("id = ? AND installation_id = ?", id, instID).
		Update("role", body.Role).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
	audit.Log(h.db, r, user.Login, user.ID, "team_member.update", "team_member",
		fmt.Sprint(id), nil, map[string]any{"role": body.Role})
}

func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID := installationIDForUser(h.db, user.Login)
	h.db.WithContext(r.Context()).
		Where("id = ? AND installation_id = ?", id, instID).
		Delete(&models.TeamMember{})
	audit.Log(h.db, r, user.Login, user.ID, "team_member.remove", "team_member",
		fmt.Sprint(id), map[string]any{"id": id}, nil)
	w.WriteHeader(http.StatusNoContent)
}

// GetSyncConfig returns the team sync rule configuration stored in SystemConfig.
func (h *TeamHandler) GetSyncConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var cfg models.SystemConfig
	if err := h.db.WithContext(r.Context()).Where("key = ?", "team_sync_rules").First(&cfg).Error; err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	var rules []any
	_ = json.Unmarshal([]byte(cfg.Value), &rules)
	writeJSON(w, http.StatusOK, rules)
}

// PutSyncConfig stores a JSON array of {org, team, role} sync rules.
func (h *TeamHandler) PutSyncConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var rules []struct {
		Org  string `json:"org"`
		Team string `json:"team"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	validRoles := map[string]bool{"admin": true, "reviewer": true, "viewer": true}
	for _, rule := range rules {
		if rule.Org == "" || rule.Team == "" {
			writeError(w, http.StatusBadRequest, "each rule requires org and team")
			return
		}
		if rule.Role == "" {
			rule.Role = "reviewer"
		}
		if !validRoles[rule.Role] {
			writeError(w, http.StatusBadRequest, "role must be admin|reviewer|viewer")
			return
		}
	}
	raw, _ := json.Marshal(rules)
	h.db.WithContext(r.Context()).
		Where(models.SystemConfig{Key: "team_sync_rules"}).
		Assign(models.SystemConfig{Key: "team_sync_rules", Value: string(raw)}).
		FirstOrCreate(&models.SystemConfig{})
	w.WriteHeader(http.StatusNoContent)
}

// TriggerSync enqueues an immediate team sync job.
func (h *TeamHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.enqueuer == nil {
		writeError(w, http.StatusServiceUnavailable, "job queue not available")
		return
	}
	if _, err := h.enqueuer.Insert(r.Context(), jobs.TeamSyncJobArgs{}, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue sync job")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
