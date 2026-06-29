package adapters

import "github.com/Astraxx04/pr-reviewer/internal/ai/llm"

// NewOpenAICompatible creates a Provider for any OpenAI-compatible endpoint
// (Groq, Together AI, Mistral, LMStudio, vLLM, etc.).
func NewOpenAICompatible(apiKey, baseURL, model string) llm.Provider {
	a := NewOpenAI(apiKey, baseURL, model).(*openaiAdapter)
	a.model = model
	return &openaiCompatibleAdapter{openaiAdapter: a}
}

type openaiCompatibleAdapter struct {
	*openaiAdapter
}

func (a *openaiCompatibleAdapter) Name() string { return "openai_compatible" }
