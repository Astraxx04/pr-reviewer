package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"pr-reviewer/internal/db"
	"pr-reviewer/internal/db/models"
	"pr-reviewer/internal/integration/jira"
)

// IntegrationHandler manages third-party integration configs (currently Jira).
type IntegrationHandler struct {
	db     *gorm.DB
	encKey string
}

func NewIntegrationHandler(db *gorm.DB, encKey string) *IntegrationHandler {
	return &IntegrationHandler{db: db, encKey: encKey}
}

// GetJira returns the Jira config, masking the API token.
func (h *IntegrationHandler) GetJira(w http.ResponseWriter, r *http.Request) {
	var cfg models.JiraConfig
	if err := h.db.WithContext(r.Context()).First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         cfg.ID,
		"base_url":   cfg.BaseURL,
		"email":      cfg.Email,
		"enabled":    cfg.Enabled,
		"configured": true,
		"created_at": cfg.CreatedAt,
		"updated_at": cfg.UpdatedAt,
	})
}

// PutJira creates or updates the Jira config.
func (h *IntegrationHandler) PutJira(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BaseURL  string `json:"base_url"`
		Email    string `json:"email"`
		APIToken string `json:"api_token"` // empty = keep existing
		Enabled  *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.BaseURL == "" || body.Email == "" {
		http.Error(w, "base_url and email are required", http.StatusBadRequest)
		return
	}

	var cfg models.JiraConfig
	h.db.WithContext(r.Context()).First(&cfg) // existing or zero-value

	cfg.BaseURL = body.BaseURL
	cfg.Email = body.Email
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	} else {
		cfg.Enabled = true
	}

	if body.APIToken != "" {
		encrypted, err := db.Encrypt(body.APIToken, h.encKey)
		if err != nil {
			http.Error(w, "encryption failed", http.StatusInternalServerError)
			return
		}
		cfg.APITokenEncrypted = encrypted
	} else if cfg.APITokenEncrypted == "" {
		http.Error(w, "api_token is required for a new integration", http.StatusBadRequest)
		return
	}

	if err := h.db.WithContext(r.Context()).Save(&cfg).Error; err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteJira removes the Jira config.
func (h *IntegrationHandler) DeleteJira(w http.ResponseWriter, r *http.Request) {
	if err := h.db.WithContext(r.Context()).Delete(&models.JiraConfig{}, "1=1").Error; err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestJira verifies that the stored credentials can authenticate with Jira.
func (h *IntegrationHandler) TestJira(w http.ResponseWriter, r *http.Request) {
	var cfg models.JiraConfig
	if err := h.db.WithContext(r.Context()).First(&cfg).Error; err != nil {
		http.Error(w, "Jira not configured", http.StatusBadRequest)
		return
	}

	apiToken, err := db.Decrypt(cfg.APITokenEncrypted, h.encKey)
	if err != nil {
		http.Error(w, "decrypt error", http.StatusInternalServerError)
		return
	}

	client := jira.NewClient(cfg.BaseURL, cfg.Email, apiToken)
	info, err := client.TestAuth(r.Context())
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"display_name": info.DisplayName,
		"email":        info.Email,
	})
}
