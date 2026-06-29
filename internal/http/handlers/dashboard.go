package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type DashboardHandler struct{ db *gorm.DB }

func NewDashboardHandler(db *gorm.DB) *DashboardHandler { return &DashboardHandler{db: db} }

func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	var totalReviews int64
	var avgScore float64
	var approvals, changes, comments int64

	h.db.WithContext(r.Context()).Model(&models.Review{}).Count(&totalReviews)
	h.db.WithContext(r.Context()).Model(&models.Review{}).Select("coalesce(avg(score),0)").Scan(&avgScore)
	h.db.WithContext(r.Context()).Model(&models.Review{}).Where("status = 'APPROVE'").Count(&approvals)
	h.db.WithContext(r.Context()).Model(&models.Review{}).Where("status = 'REQUEST_CHANGES'").Count(&changes)
	h.db.WithContext(r.Context()).Model(&models.Review{}).Where("status = 'COMMENT'").Count(&comments)

	var totalRepos int64
	var enabledRepos int64
	h.db.WithContext(r.Context()).Model(&models.Repository{}).Count(&totalRepos)
	h.db.WithContext(r.Context()).Model(&models.Repository{}).Where("enabled = true").Count(&enabledRepos)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_reviews":   totalReviews,
		"avg_score":       avgScore,
		"approvals":       approvals,
		"request_changes": changes,
		"comments":        comments,
		"total_repos":     totalRepos,
		"enabled_repos":   enabledRepos,
	})
}
