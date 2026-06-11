package audit

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

// Log writes one audit entry. Non-fatal: if db is nil or write fails, silently continues.
func Log(db *gorm.DB, r *http.Request, actorLogin string, actorID uint,
	action, entityType, entityID string, before, after any) {
	if db == nil {
		return
	}
	var beforeJSON, afterJSON []byte
	if before != nil {
		beforeJSON, _ = json.Marshal(before)
	}
	if after != nil {
		afterJSON, _ = json.Marshal(after)
	}
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}
	db.Create(&models.AuditLog{
		ActorLogin: actorLogin,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Before:     beforeJSON,
		After:      afterJSON,
		IPAddress:  ip,
	})
}
