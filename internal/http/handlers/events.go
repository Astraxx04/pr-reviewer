package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Astraxx04/pr-reviewer/internal/events"
)

// EventsHandler streams server-sent events to authenticated clients.
type EventsHandler struct {
	hub       *events.Hub
	jwtSecret string
}

func NewEventsHandler(hub *events.Hub, jwtSecret string) *EventsHandler {
	return &EventsHandler{hub: hub, jwtSecret: jwtSecret}
}

// Subscribe validates the JWT from the ?token= query parameter (since EventSource
// does not support custom headers) and then streams events until the client disconnects.
func (h *EventsHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Accept token from query param for EventSource compatibility.
	tokenStr := strings.TrimPrefix(r.URL.Query().Get("token"), "Bearer ")
	if tokenStr == "" {
		// Also accept Authorization header for non-EventSource callers.
		tokenStr = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if tokenStr == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(ch)

	// Initial heartbeat so the browser knows the connection is open.
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	// Periodic heartbeat keeps the connection alive through proxies and
	// tunnels (e.g. ngrok) that close idle long-lived connections.
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
