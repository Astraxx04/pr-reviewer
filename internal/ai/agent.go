package ai

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"pr-reviewer/internal/ai/mcp"
	"pr-reviewer/internal/telemetry"
)

// AgentOrchestrator manages multiple agents.
type AgentOrchestrator struct {
	agents map[string]mcp.Agent
}

// NewAgentOrchestrator creates a new orchestrator.
func NewAgentOrchestrator() *AgentOrchestrator {
	return &AgentOrchestrator{
		agents: make(map[string]mcp.Agent),
	}
}

// RegisterAgent registers an agent.
func (o *AgentOrchestrator) RegisterAgent(name string, agent mcp.Agent) {
	o.agents[name] = agent
}

// Dispatch sends a request to a specific agent.
func (o *AgentOrchestrator) Dispatch(ctx context.Context, agentName string, req mcp.Request) (*mcp.Response, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "agent.dispatch")
	defer span.End()
	span.SetAttributes(attribute.String("agent.name", agentName))

	agent, ok := o.agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	return agent.Process(ctx, req)
}
