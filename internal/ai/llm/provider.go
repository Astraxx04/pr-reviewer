package llm

import "context"

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// ListModels queries the provider's models endpoint and returns the model
	// identifiers it offers, so callers can pick a default model from a live list
	// instead of typing one by hand.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	Name() string
}

// ModelInfo describes a single model offered by a provider. DisplayName is
// optional (OpenAI-compatible endpoints only return an ID).
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
}

type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Model        string
	MaxTokens    int
}

type CompletionResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

type ProviderConfig struct {
	ID           string
	Type         string // openai | anthropic | ollama | openai_compatible
	APIKey       string
	BaseURL      string
	DefaultModel string
}
