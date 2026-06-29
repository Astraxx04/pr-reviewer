package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/Astraxx04/pr-reviewer/internal/ai/llm"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicAdapter struct {
	client anthropic.Client
	model  string
}

func NewAnthropic(apiKey, model string) llm.Provider {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &anthropicAdapter{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

func (a *anthropicAdapter) Name() string { return "anthropic" }

func (a *anthropicAdapter) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	start := time.Now()
	iter := a.client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
	var models []llm.ModelInfo
	for iter.Next() {
		m := iter.Current()
		models = append(models, llm.ModelInfo{ID: m.ID, DisplayName: m.DisplayName})
	}
	err := iter.Err()
	logger.ExternalCall(ctx, "anthropic", "Models.List", start, err, "count", len(models))
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}
	return models, nil
}

func (a *anthropicAdapter) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}
	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserPrompt)),
		},
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}

	start := time.Now()
	msg, err := a.client.Messages.New(ctx, params)
	logger.ExternalCall(ctx, "anthropic", "Messages.New", start, err, "model", model, "max_tokens", maxTokens)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}
	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("anthropic: empty response")
	}

	return &llm.CompletionResponse{
		Content:      msg.Content[0].AsText().Text,
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}, nil
}
