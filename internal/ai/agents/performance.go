package agents

import (
	"context"
	"fmt"

	"pr-reviewer/internal/ai/llm"
	"pr-reviewer/internal/ai/mcp"
	"pr-reviewer/internal/metrics"
)

const performanceSystem = `You are a senior engineer reviewing a pull request diff specifically for performance.
Respond ONLY with a valid JSON object — no markdown, no explanation — in this exact format:
{
  "summary": "One sentence overall performance assessment",
  "comments": [
    {
      "path": "relative/file/path",
      "line": <positive integer — must be a line present in the diff hunk>,
      "side": "RIGHT",
      "body": "Concise, actionable feedback",
      "priority": "p0|p1|p2|p3"
    }
  ]
}

Priority levels:
  p0 = Critical: pathological complexity or a query/loop that will not scale and blocks functionality
  p1 = High: N+1 query, unbounded allocation, blocking call on a hot path, missing pagination
  p2 = Medium: avoidable repeated work, inefficient data structure, unnecessary copy
  p3 = Low: minor optimisation or micro-inefficiency

Rules:
- Only comment on lines that are part of the diff (added or context lines).
- Focus exclusively on performance: algorithmic complexity, N+1 database queries, allocations,
  blocking I/O in hot paths, missing caching/pagination, and concurrency bottlenecks.
- Do NOT comment on style, naming, or correctness unrelated to performance — other agents cover those.
- Only report genuine issues. If nothing is wrong, return an empty comments array.`

type PerformanceAgent struct {
	registry *llm.ProviderRegistry
}

func NewPerformanceAgent(registry *llm.ProviderRegistry) *PerformanceAgent {
	return &PerformanceAgent{registry: registry}
}

func (a *PerformanceAgent) Process(ctx context.Context, req mcp.Request) (*mcp.Response, error) {
	provider, model, err := a.resolveProvider(req)
	if err != nil {
		return nil, err
	}

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: performanceSystem,
		UserPrompt:   req.Query,
		Model:        model,
	})
	if err != nil {
		return nil, fmt.Errorf("performance agent: %w", err)
	}
	metrics.RecordLLMTokens(model, resp.InputTokens, resp.OutputTokens)

	if err := validateAgentJSON(resp.Content); err != nil {
		return nil, fmt.Errorf("performance agent: invalid response JSON: %w", err)
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

func (a *PerformanceAgent) resolveProvider(req mcp.Request) (llm.Provider, string, error) {
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
