package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

type HealthHandler struct{ db *gorm.DB }

func NewHealthHandler(db *gorm.DB) *HealthHandler { return &HealthHandler{db: db} }

type healthResponse struct {
	Status    string `json:"status"`
	DB        string `json:"db"`
	Providers int    `json:"providers"`
	Timestamp string `json:"timestamp"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		DB:        "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil || sqlDB.PingContext(r.Context()) != nil {
			resp.Status = "degraded"
			resp.DB = "unreachable"
		} else {
			var count int64
			h.db.WithContext(r.Context()).Model(&models.ProviderConfig{}).Count(&count)
			resp.Providers = int(count)
		}
	} else {
		resp.DB = "disabled"
	}

	code := http.StatusOK
	if resp.Status != "ok" {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
