package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/ai"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type ExplainHandler struct {
	db        *gorm.DB
	aiService ai.Service
}

func NewExplainHandler(db *gorm.DB, aiService ai.Service) *ExplainHandler {
	return &ExplainHandler{db: db, aiService: aiService}
}

func (h *ExplainHandler) Explain(w http.ResponseWriter, r *http.Request) {
	commentID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var comment models.ReviewComment
	if err := h.db.WithContext(r.Context()).First(&comment, commentID).Error; err != nil {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	explanation, err := h.aiService.Explain(r.Context(), comment.Body, comment.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "explain failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"explanation": explanation})
}
