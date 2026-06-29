package agents

import (
	"context"
	"fmt"

	"github.com/Astraxx04/pr-reviewer/internal/ai/llm"
	"github.com/Astraxx04/pr-reviewer/internal/ai/mcp"
	"github.com/Astraxx04/pr-reviewer/internal/metrics"
)

const databaseSystem = `You are a senior engineer reviewing a pull request diff specifically for database and data-layer concerns.
Respond ONLY with a valid JSON object — no markdown, no explanation — in this exact format:
{
  "summary": "One sentence overall data-layer assessment",
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
  p0 = Critical: destructive or non-reversible migration, data loss, a migration that locks a large table
  p1 = High: schema change without an index on a new foreign key or queried column, non-concurrent index
       creation on a large table, missing NOT NULL/default that breaks existing rows, unsafe column drop/rename
  p2 = Medium: missing transaction boundary, query without pagination/limit, inconsistent constraint, N+1 access pattern
  p3 = Low: naming, missing migration comment, minor schema hygiene

Rules:
- Only comment on lines that are part of the diff (added or context lines).
- Focus exclusively on the data layer: SQL/ORM queries, schema and migration safety (especially online/zero-downtime
  concerns), indexes, constraints, transactions, and connection handling.
- Do NOT comment on unrelated application logic, style, or security — other agents cover those.
- Only report genuine issues. If nothing is wrong, return an empty comments array.`

type DatabaseAgent struct {
	registry *llm.ProviderRegistry
}

func NewDatabaseAgent(registry *llm.ProviderRegistry) *DatabaseAgent {
	return &DatabaseAgent{registry: registry}
}

func (a *DatabaseAgent) Process(ctx context.Context, req mcp.Request) (*mcp.Response, error) {
	provider, model, err := a.resolveProvider(req)
	if err != nil {
		return nil, err
	}

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: databaseSystem,
		UserPrompt:   req.Query,
		Model:        model,
	})
	if err != nil {
		return nil, fmt.Errorf("database agent: %w", err)
	}
	metrics.RecordLLMTokens(model, resp.InputTokens, resp.OutputTokens)

	if err := validateAgentJSON(resp.Content); err != nil {
		return nil, fmt.Errorf("database agent: invalid response JSON: %w", err)
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

func (a *DatabaseAgent) resolveProvider(req mcp.Request) (llm.Provider, string, error) {
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
