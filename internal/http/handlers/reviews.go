package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

type ReviewHandler struct{ db *gorm.DB }

func NewReviewHandler(db *gorm.DB) *ReviewHandler { return &ReviewHandler{db: db} }

func (h *ReviewHandler) List(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	perPage := queryInt(r, "per_page", 20)
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	var reviews []models.Review
	var total int64
	h.db.WithContext(r.Context()).Model(&models.Review{}).Count(&total)
	h.db.WithContext(r.Context()).Preload("Comments").
		Order("created_at desc").Offset(offset).Limit(perPage).Find(&reviews)

	writeJSON(w, http.StatusOK, map[string]any{
		"reviews": reviews,
		"total":   total,
		"page":    page,
		"per_page": perPage,
	})
}

func (h *ReviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var review models.Review
	if err := h.db.WithContext(r.Context()).
		Preload("Comments").Preload("Assignments").
		First(&review, id).Error; err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, review)
}
