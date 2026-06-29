package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
)

// singletonConfigID pins the GitHub App config to a single row. There is only ever
// one App configured per deployment; every save upserts this row, so a second one
// can never be created.
const singletonConfigID = 1

type GithubAppHandler struct {
	db            *gorm.DB
	encryptionKey string
}

func NewGithubAppHandler(db *gorm.DB, encryptionKey string) *GithubAppHandler {
	return &GithubAppHandler{db: db, encryptionKey: encryptionKey}
}

type githubAppStatus struct {
	Configured       bool  `json:"configured"`
	AppID            int64 `json:"app_id,omitempty"`
	HasWebhookSecret bool  `json:"has_webhook_secret"`
	HasGitHubToken   bool  `json:"has_github_token"`
}

func (h *GithubAppHandler) Get(w http.ResponseWriter, r *http.Request) {
	var cfg models.GithubAppConfig
	if err := h.db.First(&cfg).Error; err != nil {
		writeJSON(w, http.StatusOK, githubAppStatus{Configured: false})
		return
	}
	writeJSON(w, http.StatusOK, githubAppStatus{
		Configured:       true,
		AppID:            cfg.AppID,
		HasWebhookSecret: cfg.WebhookSecretEncrypted != "",
		HasGitHubToken:   cfg.GitHubTokenEncrypted != "",
	})
}

func (h *GithubAppHandler) Put(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppID         int64  `json:"app_id"`
		PrivateKey    string `json:"private_key"`    // raw PEM; omit to leave unchanged
		WebhookSecret string `json:"webhook_secret"` // raw; omit to leave unchanged
		GitHubToken   string `json:"github_token"`   // PAT; omit to leave unchanged
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Load the singleton row by its fixed ID so partial updates don't overwrite
	// unchanged fields. Querying by ID (not First) guarantees we always read and
	// write the same row, so a second config can never be created.
	var cfg models.GithubAppConfig
	h.db.Where("id = ?", singletonConfigID).First(&cfg) // ignore not-found; we'll upsert below
	cfg.ID = singletonConfigID

	if body.AppID != 0 {
		cfg.AppID = body.AppID
	}
	if cfg.AppID == 0 {
		writeError(w, http.StatusBadRequest, "app_id required")
		return
	}

	if body.PrivateKey != "" {
		if _, err := gh.CreateAppJWT(cfg.AppID, []byte(body.PrivateKey)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid private key: "+err.Error())
			return
		}
		enc, err := dbpkg.Encrypt(body.PrivateKey, h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		cfg.PrivateKeyEncrypted = enc
	}
	if cfg.PrivateKeyEncrypted == "" {
		writeError(w, http.StatusBadRequest, "private_key required")
		return
	}

	if body.WebhookSecret != "" {
		enc, err := dbpkg.Encrypt(body.WebhookSecret, h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		cfg.WebhookSecretEncrypted = enc
	}

	if body.GitHubToken != "" {
		enc, err := dbpkg.Encrypt(body.GitHubToken, h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		cfg.GitHubTokenEncrypted = enc
	}

	if err := h.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cfg).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "save failed")
		return
	}
	writeJSON(w, http.StatusOK, githubAppStatus{
		Configured:       true,
		AppID:            cfg.AppID,
		HasWebhookSecret: cfg.WebhookSecretEncrypted != "",
		HasGitHubToken:   cfg.GitHubTokenEncrypted != "",
	})
}

// Delete removes the stored GitHub App configuration. Webhook deliveries will be
// rejected until new credentials are saved.
func (h *GithubAppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Where("1 = 1").Delete(&models.GithubAppConfig{}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	writeJSON(w, http.StatusOK, githubAppStatus{Configured: false})
}

// Test verifies the stored GitHub App credentials end-to-end: it loads the config,
// decrypts the private key, signs an App JWT, and calls GitHub's GET /app endpoint.
// Reaching GitHub confirms the App ID and key actually match a real App — not just
// that the key is structurally valid.
func (h *GithubAppHandler) Test(w http.ResponseWriter, r *http.Request) {
	var cfg models.GithubAppConfig
	if err := h.db.First(&cfg).Error; err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "GitHub App not configured"})
		return
	}
	pk, err := dbpkg.Decrypt(cfg.PrivateKeyEncrypted, h.encryptionKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "key decryption failed"})
		return
	}
	slug, err := gh.VerifyAppCredentials(r.Context(), cfg.AppID, []byte(pk))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("credentials valid (app: %s)", slug)})
}
