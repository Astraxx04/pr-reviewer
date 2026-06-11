package llm

import (
	"fmt"
	"sync"
)

type entry struct {
	provider     Provider
	defaultModel string
}

type ProviderRegistry struct {
	mu        sync.RWMutex
	entries   map[string]entry
	defaultID string
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{entries: make(map[string]entry)}
}

func (r *ProviderRegistry) Register(id, defaultModel string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = entry{provider: p, defaultModel: defaultModel}
	if r.defaultID == "" {
		r.defaultID = id
	}
}

func (r *ProviderRegistry) Get(id string) (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, "", fmt.Errorf("llm: provider %q not registered", id)
	}
	return e.provider, e.defaultModel, nil
}

func (r *ProviderRegistry) Default() (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.defaultID == "" {
		return nil, "", fmt.Errorf("llm: no providers registered")
	}
	e := r.entries[r.defaultID]
	return e.provider, e.defaultModel, nil
}

// DefaultID returns the id of the default provider, or "" if none are registered.
func (r *ProviderRegistry) DefaultID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultID
}
