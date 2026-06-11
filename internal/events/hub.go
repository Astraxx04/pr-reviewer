package events

import (
	"encoding/json"
	"sync"
)

// Event is a server-sent event payload.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub broadcasts events to all active SSE subscribers.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan []byte]struct{})}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	close(ch)
	h.mu.Unlock()
}

func (h *Hub) Publish(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- data:
		default: // drop if consumer is slow
		}
	}
}
