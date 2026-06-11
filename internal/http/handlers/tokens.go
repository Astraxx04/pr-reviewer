package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/audit"
	"pr-reviewer/internal/db/models"
)

type APITokenHandler struct{ db *gorm.DB }

func NewAPITokenHandler(db *gorm.DB) *APITokenHandler { return &APITokenHandler{db: db} }

func (h *APITokenHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var tokens []models.APIToken
	h.db.WithContext(r.Context()).
		Where("user_id = ?", user.ID).
		Select("id, user_id, name, scope, prefix, last_used_at, expires_at, created_at").
		Order("created_at desc").
		Find(&tokens)
	writeJSON(w, http.StatusOK, tokens)
}

func (h *APITokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Name      string `json:"name"`
		Scope     string `json:"scope"`      // read | readwrite
		ExpiresAt string `json:"expires_at"` // YYYY-MM-DD or RFC3339; empty = no expiry
		ExpiresIn int    `json:"expires_in"` // days (alternative to expires_at); 0 = no expiry
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if body.Scope != "read" && body.Scope != "readwrite" {
		body.Scope = "read"
	}

	expiresAt, err := parseExpiry(body.ExpiresAt, body.ExpiresIn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid expiry: "+err.Error())
		return
	}

	// Generate a 32-byte cryptographically random token.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	rawStr := "prt_" + base64.RawURLEncoding.EncodeToString(raw) // "prt_" prefix + 43 chars
	prefix := rawStr[:8]

	sum := sha256.Sum256([]byte(rawStr))
	hash := hex.EncodeToString(sum[:])

	token := models.APIToken{
		UserID:    user.ID,
		Name:      body.Name,
		Scope:     body.Scope,
		Hash:      hash,
		Prefix:    prefix,
		ExpiresAt: expiresAt,
	}
	if err := h.db.WithContext(r.Context()).Create(&token).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	// Audit (never logs the raw token or its hash — only non-secret metadata).
	audit.Log(h.db, r, user.Login, user.ID, "api_token.create", "api_token", fmt.Sprint(token.ID),
		nil, map[string]any{"name": token.Name, "scope": token.Scope, "expires_at": token.ExpiresAt})

	// Return the raw token only once. Keys are PascalCase to match the List
	// response shape (the model is serialized directly there) so the frontend can
	// drop this straight into its token list without a casing mismatch.
	writeJSON(w, http.StatusCreated, map[string]any{
		"ID":         token.ID,
		"Name":       token.Name,
		"Scope":      token.Scope,
		"Prefix":     token.Prefix,
		"LastUsedAt": token.LastUsedAt,
		"ExpiresAt":  token.ExpiresAt,
		"CreatedAt":  token.CreatedAt,
		"token":      rawStr, // raw token — shown only once
	})
}

func (h *APITokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Load first so the audit entry can record which token was revoked.
	var existing models.APIToken
	if err := h.db.WithContext(r.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&existing).Error; err != nil {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	if err := h.db.WithContext(r.Context()).Delete(&existing).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "revoke failed")
		return
	}
	audit.Log(h.db, r, user.Login, user.ID, "api_token.revoke", "api_token", fmt.Sprint(existing.ID),
		map[string]any{"name": existing.Name, "scope": existing.Scope, "prefix": existing.Prefix}, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// parseExpiry resolves a token expiry from either an expires_at string
// (YYYY-MM-DD from a date picker, or RFC3339) or a legacy expires_in day count.
// A date-only value expires at the end of that day (UTC). Returns nil for "no
// expiry", and an error for an unparseable or past expiry.
func parseExpiry(expiresAt string, expiresInDays int) (*time.Time, error) {
	if expiresAt != "" {
		if t, err := time.Parse("2006-01-02", expiresAt); err == nil {
			// End of the selected day so the token is valid through that date.
			t = t.Add(24*time.Hour - time.Second)
			if t.Before(time.Now()) {
				return nil, fmt.Errorf("expiry date is in the past")
			}
			return &t, nil
		}
		if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
			if t.Before(time.Now()) {
				return nil, fmt.Errorf("expiry date is in the past")
			}
			return &t, nil
		}
		return nil, fmt.Errorf("expected YYYY-MM-DD or RFC3339")
	}
	if expiresInDays > 0 {
		t := time.Now().AddDate(0, 0, expiresInDays)
		return &t, nil
	}
	return nil, nil
}
