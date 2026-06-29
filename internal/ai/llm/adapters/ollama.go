package adapters

import "github.com/Astraxx04/pr-reviewer/internal/ai/llm"

const defaultOllamaBase = "http://localhost:11434/v1"

// NewOllama creates a Provider backed by a local Ollama server.
// Ollama exposes an OpenAI-compatible API at /v1.
func NewOllama(baseURL, model string) llm.Provider {
	if baseURL == "" {
		baseURL = defaultOllamaBase
	}
	a := NewOpenAI("ollama", baseURL, model).(*openaiAdapter)
	return &ollamaAdapter{openaiAdapter: a}
}

type ollamaAdapter struct {
	*openaiAdapter
}

func (a *ollamaAdapter) Name() string { return "ollama" }
