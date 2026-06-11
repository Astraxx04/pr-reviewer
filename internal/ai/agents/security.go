package agents

import (
	"context"
	"fmt"

	"pr-reviewer/internal/ai/llm"
	"pr-reviewer/internal/ai/mcp"
	"pr-reviewer/internal/metrics"
)

const securitySystem = `You are a security engineer reviewing a pull request diff.
Respond ONLY with a valid JSON object — no markdown, no explanation — in this exact format:
{
  "summary": "One sentence security assessment",
  "comments": [
    {
      "path": "relative/file/path",
      "line": <positive integer — must be a line present in the diff hunk>,
      "side": "RIGHT",
      "body": "Description of the security issue and how to fix it",
      "priority": "p0|p1|p2|p3"
    }
  ]
}

Priority levels for security:
  p0 = Critical: RCE, SQLi, XSS, auth bypass, exposed secrets, SSRF
  p1 = High: insecure deserialization, path traversal, privilege escalation, missing auth check
  p2 = Medium: missing rate limiting, verbose error messages, weak crypto
  p3 = Low: missing security header, informational finding

Rules:
- Only comment on lines that are part of the diff (added or context lines).
- Only report genuine security findings. If nothing is found, return an empty comments array.`

type SecurityAgent struct {
	registry *llm.ProviderRegistry
}

func NewSecurityAgent(registry *llm.ProviderRegistry) *SecurityAgent {
	return &SecurityAgent{registry: registry}
}

func (a *SecurityAgent) Process(ctx context.Context, req mcp.Request) (*mcp.Response, error) {
	provider, model, err := a.resolveProvider(req)
	if err != nil {
		return nil, err
	}

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: securitySystem,
		UserPrompt:   req.Query,
		Model:        model,
	})
	if err != nil {
		return nil, fmt.Errorf("security agent: %w", err)
	}
	metrics.RecordLLMTokens(model, resp.InputTokens, resp.OutputTokens)

	if err := validateAgentJSON(resp.Content); err != nil {
		return nil, fmt.Errorf("security agent: invalid response JSON: %w", err)
	}

	return &mcp.Response{
		Content: resp.Content,
		Metadata: map[string]any{
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
			"provider":      provider.Name(),
		},
	}, nil
}

func (a *SecurityAgent) resolveProvider(req mcp.Request) (llm.Provider, string, error) {
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
