package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/ai/llm"
	"github.com/Astraxx04/pr-reviewer/internal/ai/llm/adapters"
	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
)

// providerPresetBaseURLs maps named preset provider types to their base URLs.
// These types all use an OpenAI-compatible API.
var providerPresetBaseURLs = map[string]string{
	"groq":        "https://api.groq.com/openai/v1",
	"mistral":     "https://api.mistral.ai/v1",
	"together_ai": "https://api.together.xyz/v1",
	"gemini":      "https://generativelanguage.googleapis.com/v1beta/openai/",
	"perplexity":  "https://api.perplexity.ai",
}

// canonicalProviderType maps preset types to their underlying adapter type.
func canonicalProviderType(t string) string {
	if _, ok := providerPresetBaseURLs[t]; ok {
		return "openai_compatible"
	}
	return t
}

type ProviderHandler struct {
	db            *gorm.DB
	repo          *repo.ProviderRepo
	encryptionKey string
}

func NewProviderHandler(db *gorm.DB, encryptionKey string) *ProviderHandler {
	return &ProviderHandler{db: db, repo: repo.NewProviderRepo(db), encryptionKey: encryptionKey}
}

func (h *ProviderHandler) installationID(r *http.Request) uint {
	user := getUser(r)
	if user == nil {
		return 0
	}
	return installationIDForUser(h.db, user.Login)
}

func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	instID := h.installationID(r)
	providers, err := h.repo.List(r.Context(), instID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// Mask API keys in response.
	type safe struct {
		ID                 uint   `json:"id"`
		Name               string `json:"name"`
		Type               string `json:"type"`
		BaseURL            string `json:"base_url"`
		DefaultModel       string `json:"default_model"`
		SupportsEmbeddings bool   `json:"supports_embeddings"`
		EmbeddingModel     string `json:"embedding_model"`
		HasAPIKey          bool   `json:"has_api_key"`
	}
	out := make([]safe, len(providers))
	for i, p := range providers {
		out[i] = safe{
			ID: p.ID, Name: p.Name, Type: p.Type,
			BaseURL: p.BaseURL, DefaultModel: p.DefaultModel,
			SupportsEmbeddings: p.SupportsEmbeddings,
			EmbeddingModel:     p.EmbeddingModel,
			HasAPIKey:          p.APIKeyEncrypted != "",
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	instID := h.installationID(r)
	var body struct {
		Name               string `json:"name"`
		Type               string `json:"type"`
		APIKey             string `json:"api_key"`
		BaseURL            string `json:"base_url"`
		DefaultModel       string `json:"default_model"`
		SupportsEmbeddings bool   `json:"supports_embeddings"`
		EmbeddingModel     string `json:"embedding_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Type == "" {
		writeError(w, http.StatusBadRequest, "type required")
		return
	}
	enc, err := h.encryptKey(body.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}
	// Auto-fill base URL for preset provider types.
	baseURL := body.BaseURL
	if baseURL == "" {
		if preset, ok := providerPresetBaseURLs[body.Type]; ok {
			baseURL = preset
		}
	}
	p := models.ProviderConfig{
		InstallationID:     instID,
		Name:               body.Name,
		Type:               body.Type,
		APIKeyEncrypted:    enc,
		BaseURL:            baseURL,
		DefaultModel:       body.DefaultModel,
		SupportsEmbeddings: body.SupportsEmbeddings,
		EmbeddingModel:     body.EmbeddingModel,
	}
	if err := h.repo.Create(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID})
}

func (h *ProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID := h.installationID(r)
	p, err := h.repo.FindByID(r.Context(), id, instID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var body struct {
		Name               *string `json:"name"`
		APIKey             *string `json:"api_key"`
		BaseURL            *string `json:"base_url"`
		DefaultModel       *string `json:"default_model"`
		SupportsEmbeddings *bool   `json:"supports_embeddings"`
		EmbeddingModel     *string `json:"embedding_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.APIKey != nil && *body.APIKey != "" {
		enc, err := h.encryptKey(*body.APIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		p.APIKeyEncrypted = enc
	}
	if body.BaseURL != nil {
		p.BaseURL = *body.BaseURL
	}
	if body.DefaultModel != nil {
		p.DefaultModel = *body.DefaultModel
	}
	if body.SupportsEmbeddings != nil {
		p.SupportsEmbeddings = *body.SupportsEmbeddings
	}
	if body.EmbeddingModel != nil {
		p.EmbeddingModel = *body.EmbeddingModel
	}
	if err := h.repo.Update(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": p.ID})
}

func (h *ProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID := h.installationID(r)
	if err := h.repo.Delete(r.Context(), id, instID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProviderHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID := h.installationID(r)
	p, err := h.repo.FindByID(r.Context(), id, instID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	apiKey, err := h.decryptKey(p.APIKeyEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decryption failed")
		return
	}
	start := time.Now()
	ok2, msg := testProviderConnection(r.Context(), p.Type, p.BaseURL, apiKey, p.DefaultModel)
	latency := time.Since(start).Milliseconds()

	// Persist health record.
	health := models.ProviderHealth{
		ProviderConfigID: p.ID,
		LastTestedAt:     start,
		LatencyMS:        latency,
		OK:               ok2,
		UpdatedAt:        time.Now(),
	}
	if !ok2 {
		health.ErrorMsg = msg
	}
	upsertProviderHealth(h.db, &health)

	writeJSON(w, http.StatusOK, map[string]any{"ok": ok2, "message": msg, "latency_ms": latency})
}

// ListModels fetches the models a provider offers from its live API. It accepts
// either inline credentials (type/base_url/api_key) for the add-provider form, or
// an existing provider id to reuse its stored, encrypted key.
func (h *ProviderHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      uint   `json:"id"` // optional: use a stored provider's saved key
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	providerType, baseURL, apiKey := body.Type, body.BaseURL, body.APIKey
	if body.ID != 0 {
		p, err := h.repo.FindByID(r.Context(), body.ID, h.installationID(r))
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		providerType = p.Type
		if baseURL == "" {
			baseURL = p.BaseURL
		}
		if apiKey == "" {
			if dec, err := h.decryptKey(p.APIKeyEncrypted); err == nil {
				apiKey = dec
			}
		}
	}

	if providerType == "" {
		writeError(w, http.StatusBadRequest, "type required")
		return
	}
	if apiKey == "" && canonicalProviderType(providerType) != "ollama" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no API key configured", "models": []llm.ModelInfo{}})
		return
	}

	provider, err := buildProvider(providerType, baseURL, apiKey, "")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error(), "models": []llm.ModelInfo{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	models, err := provider.ListModels(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error(), "models": []llm.ModelInfo{}})
		return
	}
	if models == nil {
		models = []llm.ModelInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "models": models})
}

func (h *ProviderHandler) Health(w http.ResponseWriter, r *http.Request) {
	instID := h.installationID(r)
	providers, err := h.repo.List(r.Context(), instID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	type healthEntry struct {
		ProviderID   uint       `json:"provider_id"`
		ProviderName string     `json:"provider_name"`
		ProviderType string     `json:"provider_type"`
		LastTestedAt *time.Time `json:"last_tested_at"`
		LatencyMS    *int64     `json:"latency_ms"`
		OK           *bool      `json:"ok"`
		ErrorMsg     string     `json:"error_msg,omitempty"`
		Status       string     `json:"status"` // healthy | degraded | unreachable | untested
	}

	out := make([]healthEntry, 0, len(providers))
	for _, p := range providers {
		entry := healthEntry{
			ProviderID:   p.ID,
			ProviderName: p.Name,
			ProviderType: p.Type,
			Status:       "untested",
		}
		var health models.ProviderHealth
		if err := h.db.WithContext(r.Context()).
			Where("provider_config_id = ?", p.ID).First(&health).Error; err == nil {
			entry.LastTestedAt = &health.LastTestedAt
			entry.LatencyMS = &health.LatencyMS
			entry.OK = &health.OK
			entry.ErrorMsg = health.ErrorMsg
			switch {
			case health.OK && health.LatencyMS < 5000:
				entry.Status = "healthy"
			case health.OK:
				entry.Status = "degraded"
			default:
				entry.Status = "unreachable"
			}
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, out)
}

func upsertProviderHealth(db *gorm.DB, health *models.ProviderHealth) {
	var existing models.ProviderHealth
	if db.Where("provider_config_id = ?", health.ProviderConfigID).First(&existing).Error == nil {
		health.ID = existing.ID
	}
	db.Save(health)
}

func (h *ProviderHandler) encryptKey(key string) (string, error) {
	if key == "" || h.encryptionKey == "" {
		return key, nil
	}
	return dbpkg.Encrypt(key, h.encryptionKey)
}

func (h *ProviderHandler) decryptKey(enc string) (string, error) {
	if enc == "" || h.encryptionKey == "" {
		return enc, nil
	}
	return dbpkg.Decrypt(enc, h.encryptionKey)
}

// TestProviderConnection is exported for use by the background health-check goroutine in main.
func TestProviderConnection(ctx context.Context, providerType, baseURL, apiKey, model string) (bool, string) {
	return testProviderConnection(ctx, providerType, baseURL, apiKey, model)
}

func testProviderConnection(ctx context.Context, providerType, baseURL, apiKey, model string) (bool, string) {
	if apiKey == "" && canonicalProviderType(providerType) != "ollama" {
		return false, "no API key configured"
	}

	provider, err := buildProvider(providerType, baseURL, apiKey, model)
	if err != nil {
		return false, err.Error()
	}

	testCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if _, err := provider.Complete(testCtx, llm.CompletionRequest{
		UserPrompt: "Reply with the single word: ok",
		MaxTokens:  8,
	}); err != nil {
		return false, fmt.Sprintf("connection failed: %v", err)
	}
	return true, "connection successful"
}

// buildProvider constructs an llm.Provider from a provider type, base URL, API
// key, and model, auto-filling preset base URLs. Shared by connection tests and
// model listing.
func buildProvider(providerType, baseURL, apiKey, model string) (llm.Provider, error) {
	if baseURL == "" {
		if preset, ok := providerPresetBaseURLs[providerType]; ok {
			baseURL = preset
		}
	}
	switch canonicalProviderType(providerType) {
	case "anthropic":
		return adapters.NewAnthropic(apiKey, model), nil
	case "openai", "openai_compatible":
		if providerType == "openai" {
			return adapters.NewOpenAI(apiKey, baseURL, model), nil
		}
		return adapters.NewOpenAICompatible(apiKey, baseURL, model), nil
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return adapters.NewOllama(baseURL, model), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}
