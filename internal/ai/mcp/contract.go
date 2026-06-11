package mcp

import "context"

// Request is the input passed to an agent: the prompt to run and an optional
// context map (e.g. provider_id/model overrides).
type Request struct {
	Query   string         `json:"query"`
	Context map[string]any `json:"context"`
}

// Response is an agent's output: the generated content plus optional metadata
// (e.g. input_tokens/output_tokens).
type Response struct {
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Agent defines the contract for an AI agent.
type Agent interface {
	Process(ctx context.Context, req Request) (*Response, error)
}
