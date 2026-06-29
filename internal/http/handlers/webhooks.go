package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
)

type WebhookDeliveryHandler struct {
	db           *gorm.DB
	deliveryRepo *repo.DeliveryRepo
}

func NewWebhookDeliveryHandler(db *gorm.DB) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{db: db, deliveryRepo: repo.NewDeliveryRepo(db)}
}

func (h *WebhookDeliveryHandler) List(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	perPage := queryInt(r, "per_page", 50)
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	rows, total, err := h.deliveryRepo.List(r.Context(), perPage, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries": rows,
		"total":      total,
		"page":       page,
		"per_page":   perPage,
	})
}
