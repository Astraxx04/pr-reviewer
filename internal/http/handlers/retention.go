package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pr-reviewer/internal/db/models"
)

type RetentionHandler struct{ db *gorm.DB }

func NewRetentionHandler(db *gorm.DB) *RetentionHandler { return &RetentionHandler{db: db} }

type RetentionSettings struct {
	ReviewRetentionDays      int  `json:"review_retention_days"`       // 0 = keep forever
	PurgeEmbeddingsOnDisable bool `json:"purge_embeddings_on_disable"`
}

func (h *RetentionHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	writeJSON(w, http.StatusOK, LoadRetentionSettings(h.db))
}

func (h *RetentionHandler) Put(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var body RetentionSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	saveRetentionSettings(h.db, body)
	writeJSON(w, http.StatusOK, body)
}

func (h *RetentionHandler) EraseUser(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	login := r.PathValue("login")
	if login == "" {
		writeError(w, http.StatusBadRequest, "login required")
		return
	}

	ctx := r.Context()
	// Find the user's ID before deleting (for token cascade).
	var uid uint
	h.db.WithContext(ctx).Model(&models.User{}).
		Where("login = ?", login).
		Pluck("id", &uid)

	// Delete the user record.
	h.db.WithContext(ctx).Where("login = ?", login).Delete(&models.User{})
	// Anonymise audit log entries for the user rather than deleting them (maintain audit trail integrity).
	h.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("actor_login = ?", login).
		Update("actor_login", "[deleted]")
	// Delete API tokens.
	if uid > 0 {
		h.db.WithContext(ctx).Where("user_id = ?", uid).Delete(&models.APIToken{})
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "login": login})
}

// PurgeOldReviews deletes reviews older than retentionDays. Called by the scheduled purge job.
func PurgeOldReviews(db *gorm.DB, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	// Delete associated comments first (foreign key).
	db.Exec(`DELETE FROM review_comments WHERE review_id IN (SELECT id FROM reviews WHERE created_at < ?)`, cutoff)
	// Delete reviews.
	res := db.Where("created_at < ?", cutoff).Delete(&models.Review{})
	return res.RowsAffected, res.Error
}

// LoadRetentionSettings reads retention settings from SystemConfig.
// Exported so cmd/server/main.go can call it for the purge goroutine.
func LoadRetentionSettings(db *gorm.DB) RetentionSettings {
	var s RetentionSettings
	var v models.SystemConfig
	if db.Where("key = ?", "retention_review_days").First(&v).Error == nil && v.Value != "" {
		_, _ = fmt.Sscan(v.Value, &s.ReviewRetentionDays)
	}
	var p models.SystemConfig
	if db.Where("key = ?", "retention_purge_embeddings").First(&p).Error == nil {
		s.PurgeEmbeddingsOnDisable = p.Value == "true"
	}
	return s
}

func saveRetentionSettings(db *gorm.DB, s RetentionSettings) {
	db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.SystemConfig{Key: "retention_review_days", Value: fmt.Sprint(s.ReviewRetentionDays)})

	purgeVal := "false"
	if s.PurgeEmbeddingsOnDisable {
		purgeVal = "true"
	}
	db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.SystemConfig{Key: "retention_purge_embeddings", Value: purgeVal})
}

// RetentionContext is just to satisfy context usage in case it's referenced.
var _ = context.Background
