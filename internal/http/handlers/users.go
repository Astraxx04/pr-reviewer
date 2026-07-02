package handlers

import (
	"fmt"
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
)

type UserHandler struct {
	userRepo *repo.UserRepo
	db       *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{
		userRepo: repo.NewUserRepo(db),
		db:       db,
	}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	users, err := h.userRepo.FindAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	type userView struct {
		ID        uint   `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		Role      string `json:"role"`
		Status    string `json:"status"`
	}
	out := make([]userView, len(users))
	for i, u := range users {
		out[i] = userView{
			ID:        u.ID,
			Login:     u.Login,
			Email:     u.Email,
			AvatarURL: u.AvatarURL,
			Role:      u.Role,
			Status:    u.Status,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	targetID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Owner role is assigned at first login and cannot be changed via this endpoint.
	target, err := h.userRepo.FindByID(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == "owner" {
		writeError(w, http.StatusForbidden, "owner role cannot be changed")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	validRoles := map[string]bool{"admin": true, "reviewer": true}
	if !validRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "invalid role; must be admin or reviewer")
		return
	}
	if err := h.userRepo.UpdateRole(r.Context(), targetID, body.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	// Wipe all sessions so the next request forces a re-login with the new role.
	h.db.WithContext(r.Context()).Where("user_id = ?", targetID).Delete(&models.Session{})

	audit.Log(h.db, r, user.Login, user.ID, "user.role_changed", "user",
		fmt.Sprint(targetID),
		map[string]any{"role": target.Role},
		map[string]any{"role": body.Role})

	w.WriteHeader(http.StatusNoContent)
}

// Remove suspends a user and wipes their sessions. Only admins/owners can call
// this; the owner account itself cannot be removed.
func (h *UserHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	targetID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	target, err := h.userRepo.FindByID(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == "owner" {
		writeError(w, http.StatusForbidden, "owner cannot be removed")
		return
	}
	if err := h.userRepo.UpdateStatus(r.Context(), targetID, "suspended"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove user")
		return
	}
	h.db.WithContext(r.Context()).Where("user_id = ?", targetID).Delete(&models.Session{})
	audit.Log(h.db, r, user.Login, user.ID, "user.removed", "user",
		fmt.Sprint(targetID),
		map[string]any{"role": target.Role, "status": target.Status},
		map[string]any{"status": "suspended"})
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	targetID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.userRepo.UpdateStatus(r.Context(), targetID, "active"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) Reject(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	targetID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.userRepo.UpdateStatus(r.Context(), targetID, "rejected"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
