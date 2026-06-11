package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"pr-reviewer/pkg/logger"
)

// openaiEmbedder calls the OpenAI Embeddings API directly.
// Uses text-embedding-3-small (1536 dims) by default.
type openaiEmbedder struct {
	apiKey string
	model  string
}

func NewOpenAIEmbedder(apiKey, model string) Embedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &openaiEmbedder{apiKey: apiKey, model: model}
}

func (e *openaiEmbedder) Dim() int   { return dimForModel(e.model) }
func (e *openaiEmbedder) ID() string { return "openai/" + e.model }

func (e *openaiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	return vecs[0], nil
}

func (e *openaiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(map[string]any{
		"input": texts,
		"model": e.model,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	logger.ExternalCall(ctx, "openai-embeddings", "POST /v1/embeddings", start, err, "model", e.model, "inputs", len(texts))
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai embed: status %d", resp.StatusCode)
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("openai embed: decode: %w", err)
	}

	result := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		result[i] = d.Embedding
	}
	return result, nil
}
