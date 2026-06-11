package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"pr-reviewer/internal/ai/llm"
	"pr-reviewer/pkg/logger"
)

type openaiAdapter struct {
	client openai.Client
	model  string
}

func NewOpenAI(apiKey, baseURL, model string) llm.Provider {
	if model == "" {
		model = "gpt-4o"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &openaiAdapter{
		client: openai.NewClient(opts...),
		model:  model,
	}
}

func (a *openaiAdapter) Name() string { return "openai" }

func (a *openaiAdapter) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	start := time.Now()
	iter := a.client.Models.ListAutoPaging(ctx)
	var models []llm.ModelInfo
	for iter.Next() {
		models = append(models, llm.ModelInfo{ID: iter.Current().ID})
	}
	err := iter.Err()
	logger.ExternalCall(ctx, "openai", "Models.List", start, err, "count", len(models))
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	return models, nil
}

func (a *openaiAdapter) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}
	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(req.UserPrompt),
	}
	if req.SystemPrompt != "" {
		messages = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(req.SystemPrompt)}, messages...)
	}

	start := time.Now()
	resp, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     model,
		Messages:  messages,
		MaxTokens: openai.Int(maxTokens),
	})
	logger.ExternalCall(ctx, "openai", "Chat.Completions.New", start, err, "model", model, "max_tokens", maxTokens)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty response")
	}

	return &llm.CompletionResponse{
		Content:      resp.Choices[0].Message.Content,
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}, nil
}
