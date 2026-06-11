package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/repo"
)

type SessionHandler struct {
	sessionRepo *repo.SessionRepo
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{sessionRepo: repo.NewSessionRepo(db)}
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessions, err := h.sessionRepo.ListForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	type sessionView struct {
		ID           string `json:"id"`
		UserAgent    string `json:"user_agent"`
		IPAddress    string `json:"ip_address"`
		LastActiveAt string `json:"last_active_at"`
		ExpiresAt    string `json:"expires_at"`
		CreatedAt    string `json:"created_at"`
		Current      bool   `json:"current"`
	}
	out := make([]sessionView, len(sessions))
	for i, s := range sessions {
		out[i] = sessionView{
			ID:           s.ID,
			UserAgent:    s.UserAgent,
			IPAddress:    s.IPAddress,
			LastActiveAt: s.LastActiveAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt:    s.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Current:      s.ID == user.SessionID,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *SessionHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}
	if err := h.sessionRepo.Delete(r.Context(), sessionID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) RevokeAll(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.sessionRepo.DeleteAllForUser(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
