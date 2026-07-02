package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type TeamHandler struct {
	db          *gorm.DB
	frontendURL string
}

func NewTeamHandler(db *gorm.DB, _ interface{}, frontendURL string) *TeamHandler {
	return &TeamHandler{db: db, frontendURL: frontendURL}
}

// List returns all users (any status) for the installation.
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	type memberView struct {
		ID        uint   `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Role      string `json:"role"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	var users []models.User
	h.db.WithContext(r.Context()).
		Where("role IN ?", []string{"owner", "admin", "reviewer"}).
		Order("created_at ASC").
		Find(&users)
	out := make([]memberView, len(users))
	for i, u := range users {
		out[i] = memberView{
			ID:        u.ID,
			Login:     u.Login,
			AvatarURL: u.AvatarURL,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	writeJSON(w, http.StatusOK, out)
}
