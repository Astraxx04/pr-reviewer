package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type AuditHandler struct{ db *gorm.DB }

func NewAuditHandler(db *gorm.DB) *AuditHandler { return &AuditHandler{db: db} }

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	page := queryInt(r, "page", 1)
	perPage := queryInt(r, "per_page", 50)
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	q := h.db.WithContext(r.Context()).Model(&models.AuditLog{})
	if et := r.URL.Query().Get("entity_type"); et != "" {
		q = q.Where("entity_type = ?", et)
	}
	if actor := r.URL.Query().Get("actor"); actor != "" {
		q = q.Where("actor_login = ?", actor)
	}
	if since := r.URL.Query().Get("since"); since != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, since); err == nil {
				q = q.Where("created_at >= ?", t)
				break
			}
		}
	}

	var total int64
	q.Count(&total)

	var rows []models.AuditLog
	q.Order("created_at desc").Limit(perPage).Offset(offset).Find(&rows)

	writeJSON(w, http.StatusOK, map[string]any{
		"logs":     rows,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *AuditHandler) Export(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}

	var rows []models.AuditLog
	q := h.db.WithContext(r.Context()).Model(&models.AuditLog{})
	if since := r.URL.Query().Get("since"); since != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, since); err == nil {
				q = q.Where("created_at >= ?", t)
				break
			}
		}
	}
	q.Order("created_at desc").Limit(10000).Find(&rows)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-%s.csv"`, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "created_at", "actor_login", "actor_id", "action", "entity_type", "entity_id", "ip_address"})
	for _, row := range rows {
		_ = cw.Write([]string{
			fmt.Sprint(row.ID),
			row.CreatedAt.Format(time.RFC3339),
			row.ActorLogin,
			fmt.Sprint(row.ActorID),
			row.Action,
			row.EntityType,
			row.EntityID,
			row.IPAddress,
		})
	}
	cw.Flush()
}
