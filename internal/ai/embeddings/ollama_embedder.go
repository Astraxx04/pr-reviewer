package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pr-reviewer/pkg/logger"
)

type ollamaEmbedder struct {
	baseURL string
	model   string
}

func NewOllamaEmbedder(baseURL, model string) Embedder {
	return &ollamaEmbedder{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
	}
}

func (e *ollamaEmbedder) Dim() int   { return dimForModel(e.model) }
func (e *ollamaEmbedder) ID() string { return "ollama/" + e.model }

func (e *ollamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  e.model,
		"prompt": text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	logger.ExternalCall(ctx, "ollama-embeddings", "POST /api/embeddings", start, err, "model", e.model)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed: status %d", resp.StatusCode)
	}

	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama embed: decode: %w", err)
	}
	return out.Embedding, nil
}

func (e *ollamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))
	for _, t := range texts {
		vec, err := e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		result = append(result, vec)
	}
	return result, nil
}
