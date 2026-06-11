package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"pr-reviewer/internal/ai/llm"
	"pr-reviewer/internal/ai/mcp"
	"pr-reviewer/internal/metrics"
)

const conversationSystem = `You are an AI code reviewer in a GitHub pull request discussion.
A human has replied to one of your previous inline review comments. Respond helpfully and concisely.
Respond ONLY with a valid JSON object — no markdown, no explanation — in this exact format:
{
  "action": "acknowledge|clarify",
  "body": "Your reply text in GitHub Markdown"
}

- Use "acknowledge" when the human has accepted or understood the feedback, thanked you, or indicated they will fix it.
- Use "clarify" when the human has a question, pushback, or needs more explanation.
- Keep replies concise (1-3 sentences). Be collaborative, not defensive.
- Do not repeat your original comment verbatim.`

// ConversationAgent handles a human reply to a bot review comment.
type ConversationAgent struct {
	registry *llm.ProviderRegistry
}

func NewConversationAgent(registry *llm.ProviderRegistry) *ConversationAgent {
	return &ConversationAgent{registry: registry}
}

func (a *ConversationAgent) Process(ctx context.Context, req mcp.Request) (*mcp.Response, error) {
	provider, model, err := a.resolveProvider(req)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Query)
	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: conversationSystem,
		UserPrompt:   content,
		Model:        model,
	})
	if err != nil {
		return nil, fmt.Errorf("conversation agent: %w", err)
	}
	metrics.RecordLLMTokens(model, resp.InputTokens, resp.OutputTokens)

	text := strings.TrimSpace(resp.Content)
	if strings.HasPrefix(text, "```") {
		text = text[strings.Index(text, "\n")+1:]
		text = text[:strings.LastIndex(text, "```")]
		text = strings.TrimSpace(text)
	}

	if err := validateConversationJSON(text); err != nil {
		return nil, fmt.Errorf("conversation agent: invalid response JSON: %w", err)
	}

	return &mcp.Response{
		Content: text,
		Metadata: map[string]any{
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
		},
	}, nil
}

func (a *ConversationAgent) resolveProvider(req mcp.Request) (llm.Provider, string, error) {
	if id, ok := req.Context["provider_id"].(string); ok && id != "" {
		p, model, err := a.registry.Get(id)
		if err != nil {
			return nil, "", err
		}
		if override, ok := req.Context["model"].(string); ok && override != "" {
			model = override
		}
		return p, model, nil
	}
	return a.registry.Default()
}

func validateConversationJSON(s string) error {
	s = stripJSONFence(s)
	var v struct {
		Action string `json:"action"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return err
	}
	if v.Action != "acknowledge" && v.Action != "clarify" {
		return fmt.Errorf("unknown action %q", v.Action)
	}
	return nil
}
