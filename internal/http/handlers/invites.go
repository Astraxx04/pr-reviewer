package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
)

// List handles GET /api/invites (admin only).
// Returns all invites (pending and accepted) with a computed "pending" field.
func (h *InviteHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	q := h.db.WithContext(r.Context())
	switch r.URL.Query().Get("status") {
	case "pending":
		q = q.Where("accepted_at IS NULL AND expires_at > ?", time.Now())
	case "accepted":
		q = q.Where("accepted_at IS NOT NULL")
	}

	var invites []models.Invite
	q.Order("created_at desc").Find(&invites)

	type row struct {
		ID         string     `json:"id"`
		Email      string     `json:"email"`
		Role       string     `json:"role"`
		InvitedBy  string     `json:"invited_by"`
		ExpiresAt  time.Time  `json:"expires_at"`
		AcceptedAt *time.Time `json:"accepted_at,omitempty"`
		AcceptedBy string     `json:"accepted_by,omitempty"`
		CreatedAt  time.Time  `json:"created_at"`
		Pending    bool       `json:"pending"`
	}
	resp := make([]row, 0, len(invites))
	now := time.Now()
	for _, inv := range invites {
		resp = append(resp, row{
			ID:         inv.ID,
			Email:      inv.Email,
			Role:       inv.Role,
			InvitedBy:  inv.InvitedBy,
			ExpiresAt:  inv.ExpiresAt,
			AcceptedAt: inv.AcceptedAt,
			AcceptedBy: inv.AcceptedBy,
			CreatedAt:  inv.CreatedAt,
			Pending:    inv.AcceptedAt == nil && inv.ExpiresAt.After(now),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/invites/{id} (admin only).
// Only pending (not yet accepted) invites can be revoked.
func (h *InviteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var invite models.Invite
	if err := h.db.WithContext(r.Context()).
		Where("id = ? AND accepted_at IS NULL", id).
		First(&invite).Error; err != nil {
		writeError(w, http.StatusNotFound, "invite not found or already accepted")
		return
	}
	if err := h.db.WithContext(r.Context()).Delete(&invite).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	audit.Log(h.db, r, user.Login, user.ID, "invite.revoked", "user", id,
		map[string]any{"email": invite.Email, "role": invite.Role}, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Resend handles POST /api/invites/{id}/resend (admin only).
// Rotates the token and expiry, then re-sends the invite email.
func (h *InviteHandler) Resend(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var invite models.Invite
	if err := h.db.WithContext(r.Context()).
		Where("id = ? AND accepted_at IS NULL", id).
		First(&invite).Error; err != nil {
		writeError(w, http.StatusNotFound, "invite not found or already accepted")
		return
	}

	rawToken, tokenHash, err := generateInviteToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	newExpiry := time.Now().Add(time.Duration(h.ttlHours) * time.Hour)
	if err := h.db.WithContext(r.Context()).Model(&invite).Updates(map[string]any{
		"token_hash": tokenHash,
		"expires_at": newExpiry,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	audit.Log(h.db, r, user.Login, user.ID, "invite.resent", "user", id,
		nil, map[string]any{"email": invite.Email, "role": invite.Role})
	go h.sendInviteEmail(context.Background(), invite.Email, invite.Role, user.Login, rawToken, newExpiry)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         invite.ID,
		"email":      invite.Email,
		"expires_at": newExpiry,
	})
}

// Validate handles GET /api/invites/validate?token=pri_xxx.
// Public endpoint — lets the frontend confirm a token is valid before starting OAuth.
func (h *InviteHandler) Validate(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	var invite models.Invite
	if err := h.db.WithContext(r.Context()).
		Where("token_hash = ? AND accepted_at IS NULL AND expires_at > ?", hash, time.Now()).
		First(&invite).Error; err != nil {
		writeError(w, http.StatusNotFound, "invite not found or expired")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email":      invite.Email,
		"role":       invite.Role,
		"invited_by": invite.InvitedBy,
		"expires_at": invite.ExpiresAt,
	})
}

type InviteHandler struct {
	db          *gorm.DB
	frontendURL string
	ttlHours    int // default invite lifetime; configurable via INVITE_TTL_HOURS
}

func NewInviteHandler(db *gorm.DB, frontendURL string, ttlHours int) *InviteHandler {
	if ttlHours <= 0 {
		ttlHours = 7 * 24
	}
	return &InviteHandler{db: db, frontendURL: frontendURL, ttlHours: ttlHours}
}

var validInviteRoles = map[string]bool{"admin": true, "reviewer": true}

// Create handles POST /api/invites.
// Body: { "email": "...", "role": "reviewer|admin" }
func (h *InviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	if body.Role == "" {
		body.Role = "reviewer"
	}
	if !validInviteRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin or reviewer")
		return
	}

	rawToken, tokenHash, err := generateInviteToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	invite := models.Invite{
		Email:     body.Email,
		Role:      body.Role,
		TokenHash: tokenHash,
		InvitedBy: user.Login,
		ExpiresAt: time.Now().Add(time.Duration(h.ttlHours) * time.Hour),
	}
	if err := h.db.WithContext(r.Context()).Create(&invite).Error; err != nil {
		// Unique index violation means a pending invite already exists for this email.
		writeError(w, http.StatusConflict, "a pending invite already exists for this email")
		return
	}

	audit.Log(h.db, r, user.Login, user.ID, "invite.created", "user", invite.ID,
		nil, map[string]any{"email": body.Email, "role": body.Role, "invited_by": user.Login})

	go h.sendInviteEmail(context.Background(), body.Email, body.Role, user.Login, rawToken, invite.ExpiresAt)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         invite.ID,
		"email":      invite.Email,
		"role":       invite.Role,
		"invited_by": invite.InvitedBy,
		"expires_at": invite.ExpiresAt,
	})
}

// sendInviteEmail sends the invite email via the configured email channel.
func (h *InviteHandler) sendInviteEmail(ctx context.Context, email, role, invitedBy, rawToken string, expiresAt time.Time) {
	var cfgs []models.NotificationConfig
	h.db.WithContext(ctx).
		Where("channel = ? AND enabled = true", "email").
		Find(&cfgs)
	if len(cfgs) == 0 {
		return
	}

	link := fmt.Sprintf("%s/accept-invite?token=%s", h.frontendURL, rawToken)
	subject := fmt.Sprintf("%s invited you to PR Reviewer as %s", invitedBy, role)
	body := fmt.Sprintf(`<p><strong>%s</strong> has invited you to join PR Reviewer as <strong>%s</strong>.</p>
<p><a href="%s">Accept invitation</a></p>
<p>This link expires on %s.</p>
<p style="color:#888;font-size:0.9em">If you didn't expect this email, you can ignore it.</p>`,
		invitedBy, role, link, expiresAt.UTC().Format("2 Jan 2006"))

	for _, cfg := range cfgs {
		var ec notifications.EmailChannelConfig
		if err := json.Unmarshal(cfg.Config, &ec); err != nil {
			continue
		}
		smtp, from := notifications.ResolveEmail(ec)
		_ = notifications.SendEmail(ctx, smtp, from, []string{email}, subject, body)
	}
}

const bulkInviteLimit = 500

// Bulk handles POST /api/invites/bulk.
// Body: { "role": "reviewer|admin", "emails": ["a@b.com", ...] }
// Returns per-email result: "sent" | "already_pending" | "error".
func (h *InviteHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Role   string   `json:"role"`
		Emails []string `json:"emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Role == "" {
		body.Role = "reviewer"
	}
	if !validInviteRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin or reviewer")
		return
	}
	if len(body.Emails) == 0 {
		writeError(w, http.StatusBadRequest, "emails required")
		return
	}
	if len(body.Emails) > bulkInviteLimit {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("maximum %d emails per request", bulkInviteLimit))
		return
	}

	expiresAt := time.Now().Add(time.Duration(h.ttlHours) * time.Hour)

	results := make(map[string]string, len(body.Emails))
	var toSend []struct{ email, rawToken string }

	for _, email := range body.Emails {
		if email == "" {
			continue
		}
		rawToken, tokenHash, err := generateInviteToken()
		if err != nil {
			results[email] = "error"
			continue
		}
		invite := models.Invite{
			Email:     email,
			Role:      body.Role,
			TokenHash: tokenHash,
			InvitedBy: user.Login,
			ExpiresAt: expiresAt,
		}
		if err := h.db.WithContext(r.Context()).Create(&invite).Error; err != nil {
			results[email] = "already_pending"
			continue
		}
		results[email] = "sent"
		toSend = append(toSend, struct{ email, rawToken string }{email, rawToken})
	}

	sent := 0
	for _, v := range results {
		if v == "sent" {
			sent++
		}
	}
	audit.Log(h.db, r, user.Login, user.ID, "invite.bulk_created", "user", "",
		nil, map[string]any{"count": sent, "role": body.Role, "invited_by": user.Login})

	// Send emails concurrently, capped at 10 in-flight at once to avoid SMTP rate limits.
	go func() {
		sem := make(chan struct{}, 10)
		for _, item := range toSend {
			sem <- struct{}{}
			go func(email, rawToken string) {
				defer func() { <-sem }()
				h.sendInviteEmail(context.Background(), email, body.Role, user.Login, rawToken, expiresAt)
			}(item.email, item.rawToken)
		}
		// Drain semaphore to wait for all goroutines before the outer goroutine exits.
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}
	}()

	writeJSON(w, http.StatusCreated, map[string]any{"results": results})
}

// generateInviteToken returns a raw token (for the email link) and its SHA-256 hex hash (for storage).
func generateInviteToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	raw = "pri_" + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return
}
