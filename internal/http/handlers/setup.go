package handlers

import (
	"net/http"
	"os"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pr-reviewer/internal/db/models"
)

type SetupHandler struct {
	db *gorm.DB
}

func NewSetupHandler(db *gorm.DB) *SetupHandler {
	return &SetupHandler{db: db}
}

type setupStatus struct {
	DatabaseOK     bool `json:"database_ok"`
	GithubConfigured bool `json:"github_configured"`
	SetupComplete  bool `json:"setup_complete"`
}

func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := setupStatus{DatabaseOK: true}

	status.GithubConfigured = os.Getenv("GITHUB_CLIENT_ID") != "" && os.Getenv("GITHUB_CLIENT_SECRET") != ""

	var cfg models.SystemConfig
	dbFlagSet := h.db.Where("key = ?", "setup_complete").First(&cfg).Error == nil && cfg.Value == "true"
	allRequired := status.GithubConfigured &&
		os.Getenv("DATABASE_URL") != "" &&
		os.Getenv("ENCRYPTION_KEY") != ""
	status.SetupComplete = dbFlagSet && allRequired

	writeJSON(w, http.StatusOK, status)
}

func (h *SetupHandler) Complete(w http.ResponseWriter, r *http.Request) {
	rec := models.SystemConfig{Key: "setup_complete", Value: "true"}
	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&rec).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark setup complete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *SetupHandler) Reset(w http.ResponseWriter, r *http.Request) {
	h.db.WithContext(r.Context()).
		Where("key = ?", "setup_complete").
		Delete(&models.SystemConfig{})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
