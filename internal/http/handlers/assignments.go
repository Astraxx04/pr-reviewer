package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
)

type AssignmentHandler struct {
	db   *gorm.DB
	repo *repo.AssignmentRuleRepo
}

func NewAssignmentHandler(db *gorm.DB) *AssignmentHandler {
	return &AssignmentHandler{db: db, repo: repo.NewAssignmentRuleRepo(db)}
}

func (h *AssignmentHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	repoID, ok := pathID(r, "repo_id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo_id")
		return
	}
	rules, err := h.repo.List(r.Context(), repoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *AssignmentHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	repoID, ok := pathID(r, "repo_id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo_id")
		return
	}
	var body struct {
		Strategy string         `json:"strategy"`
		Config   map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Strategy == "" {
		writeError(w, http.StatusBadRequest, "strategy required")
		return
	}
	raw, _ := json.Marshal(body.Config)
	rule := models.AssignmentRule{
		RepoID:   repoID,
		Strategy: body.Strategy,
		Config:   datatypes.JSON(raw),
	}
	if err := h.repo.Create(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}
