package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"pr-reviewer/internal/ai/embeddings"
	"pr-reviewer/internal/ai/mcp"
	"pr-reviewer/internal/ai/rag"
	"pr-reviewer/internal/config"
	"pr-reviewer/internal/github"
	"pr-reviewer/internal/telemetry"
	"pr-reviewer/pkg/logger"

	"go.opentelemetry.io/otel/attribute"
)

// optionalAgents are review agents that run only when a repo opts in via its
// per-repo config (agents.<name>.enabled = true). Core agents (code-review,
// security) are not listed here — they always run.
var optionalAgents = []string{"performance", "database"}

type Service interface {
	Review(ctx context.Context, req AnalysisRequest) (*ReviewResult, error)
	// Explain provides a detailed explanation for a specific AI review comment.
	Explain(ctx context.Context, commentBody, filePath string) (string, error)
}

type reviewerImpl struct {
	cfg          *config.Config
	log          *logger.Logger
	embedder     embeddings.Embedder
	retriever    rag.Retriever
	orchestrator *AgentOrchestrator
}

func NewReviewer(
	cfg *config.Config,
	log *logger.Logger,
	embedder embeddings.Embedder,
	retriever rag.Retriever,
	orchestrator *AgentOrchestrator,
) Service {
	return &reviewerImpl{
		cfg:          cfg,
		log:          log,
		embedder:     embedder,
		retriever:    retriever,
		orchestrator: orchestrator,
	}
}

type agentJSON struct {
	Summary  string `json:"summary"`
	Comments []struct {
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Side     string `json:"side"`
		Body     string `json:"body"`
		Priority string `json:"priority"` // p0|p1|p2|p3
		Severity string `json:"severity"` // legacy fallback
	} `json:"comments"`
}

func (r *reviewerImpl) Review(ctx context.Context, req AnalysisRequest) (*ReviewResult, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "ai.review")
	defer span.End()
	span.SetAttributes(
		attribute.Int("review.diff_files", len(req.Diff)),
		attribute.Bool("review.diff_truncated", req.DiffTruncated),
		attribute.Bool("review.rag_enabled", r.retriever != nil && req.RepoID > 0),
	)
	r.log.Info("Starting AI review")

	// RAG context (unchanged)
	ragContext := ""
	if r.retriever != nil && req.RepoID > 0 {
		query := buildRAGQuery(req)
		docs, err := r.retriever.Retrieve(ctx, req.RepoID, query, 5)
		if err != nil {
			r.log.Error("RAG retrieval failed", "error", err)
		} else if len(docs) > 0 {
			ragContext = formatRAGContext(docs)
			r.log.Info("RAG context retrieved", "docs", len(docs))
		}
	}

	// Format false positives and custom violations for prompt
	fpStr := strings.Join(req.FalsePositivePatterns, "\n")
	violationsStr := strings.Join(req.CustomViolations, "\n")

	prompt := ReviewPrompt.Render(map[string]interface{}{
		"Title":            req.Title,
		"Body":             req.Body,
		"Diff":             formatDiff(req.Diff),
		"RAGContext":       ragContext,
		"TicketContext":    req.TicketContext,
		"PRTemplate":       req.PRTemplate,
		"FalsePositives":   fpStr,
		"CustomViolations": violationsStr,
		"DiffTruncated":    req.DiffTruncated,
	})

	// Core agents always run; optional agents run only when a repo opts in via
	// its per-repo config (agents.<name>.enabled = true).
	agentNames := []string{"code-review", "security"}
	for _, name := range optionalAgents {
		if ac, ok := req.RepoConfig[name]; ok && ac.Enabled {
			agentNames = append(agentNames, name)
		}
	}
	type result struct {
		name string
		resp *mcp.Response
		err  error
	}
	ch := make(chan result, len(agentNames))

	var wg sync.WaitGroup
	for _, name := range agentNames {
		wg.Add(1)
		go func(agentName string) {
			defer wg.Done()
			agentCtx := map[string]interface{}{}
			if ac, ok := req.RepoConfig[agentName]; ok {
				if ac.ProviderID != "" {
					agentCtx["provider_id"] = ac.ProviderID
				}
				if ac.Model != "" {
					agentCtx["model"] = ac.Model
				}
			}
			resp, err := r.orchestrator.Dispatch(ctx, agentName, mcp.Request{
				Query:   prompt,
				Context: agentCtx,
			})
			ch <- result{name: agentName, resp: resp, err: err}
		}(name)
	}
	wg.Wait()
	close(ch)

	// Buffer all agent results
	type rawResult struct {
		name   string
		parsed *agentJSON
	}
	var rawResults []rawResult
	var combined ReviewResult

	for res := range ch {
		if res.err != nil {
			r.log.Error("Agent dispatch failed", "agent", res.name, "error", res.err)
			continue
		}
		if in, ok := res.resp.Metadata["input_tokens"].(int); ok {
			combined.InputTokens += in
		}
		if out, ok := res.resp.Metadata["output_tokens"].(int); ok {
			combined.OutputTokens += out
		}
		parsed, err := parseAgentResponse(res.resp.Content)
		if err != nil {
			r.log.Error("Failed to parse agent response", "agent", res.name, "error", err)
			continue
		}
		rawResults = append(rawResults, rawResult{name: res.name, parsed: parsed})
	}

	// Build consensus counts (only when threshold > 1)
	type lineKey struct {
		Path string
		Line int
	}
	var lineCount map[lineKey]int
	if req.ConsensusThreshold > 1 {
		lineCount = map[lineKey]int{}
		for _, raw := range rawResults {
			seen := map[lineKey]bool{}
			for _, c := range raw.parsed.Comments {
				k := lineKey{c.Path, c.Line}
				if !seen[k] {
					seen[k] = true
					lineCount[k]++
				}
			}
		}
	}

	// Merge agent results
	for _, raw := range rawResults {
		if combined.Summary == "" {
			combined.Summary = raw.parsed.Summary
		} else {
			combined.Summary += " | " + raw.parsed.Summary
		}
		for _, c := range raw.parsed.Comments {
			priority := normalisePriority(c.Priority, c.Severity)
			// Apply consensus filter to p2/p3 comments
			if lineCount != nil && (priority == "p2" || priority == "p3") {
				k := lineKey{c.Path, c.Line}
				if lineCount[k] < req.ConsensusThreshold {
					continue
				}
			}
			combined.Comments = append(combined.Comments, github.ReviewComment{
				Path:     c.Path,
				Line:     c.Line,
				Side:     sideOrDefault(c.Side),
				Body:     fmt.Sprintf("%s %s", priorityLabel(priority), c.Body),
				Severity: priorityToSeverity(priority),
				Priority: priority,
			})
		}
	}

	combined.Score = computeScore(combined.Comments)
	span.SetAttributes(
		attribute.Int("review.comments", len(combined.Comments)),
		attribute.Int("review.score", combined.Score),
		attribute.Int("review.input_tokens", combined.InputTokens),
		attribute.Int("review.output_tokens", combined.OutputTokens),
	)
	r.log.Info("AI review complete", "comments", len(combined.Comments), "score", combined.Score)
	return &combined, nil
}

func (r *reviewerImpl) Explain(ctx context.Context, commentBody, filePath string) (string, error) {
	prompt := fmt.Sprintf(`You are a senior engineer who flagged a code issue during review.

The finding:
%s

File: %s

Provide a detailed explanation covering:
1. Why this specific code pattern is a problem
2. Potential impact (security, correctness, performance, maintainability)
3. A concrete example of how to fix it

Be specific and practical.`, commentBody, filePath)

	resp, err := r.orchestrator.Dispatch(ctx, "code-review", mcp.Request{Query: prompt})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// StripJSONFence removes a leading/trailing markdown code fence (``` or ```json)
// that LLMs often wrap JSON in despite instructions not to.
func StripJSONFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}
	if nl := strings.IndexByte(content, '\n'); nl != -1 {
		content = content[nl+1:] // drop opening ``` / ```json line
	}
	if end := strings.LastIndex(content, "```"); end != -1 {
		content = content[:end] // drop closing fence
	}
	return strings.TrimSpace(content)
}

func parseAgentResponse(content string) (*agentJSON, error) {
	var out agentJSON
	if err := json.Unmarshal([]byte(StripJSONFence(content)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func computeScore(comments []github.ReviewComment) int {
	score := 100
	for _, c := range comments {
		switch c.Priority {
		case "p0":
			score -= 25
		case "p1":
			score -= 15
		case "p2":
			score -= 5
		case "p3":
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// normalisePriority returns a canonical p0–p3 value.
// If the LLM returned a legacy severity instead, it maps up.
func normalisePriority(priority, severity string) string {
	switch priority {
	case "p0", "p1", "p2", "p3":
		return priority
	}
	// fallback: map old severity field
	switch severity {
	case "error":
		return "p1"
	case "warning":
		return "p2"
	default:
		return "p3"
	}
}

func priorityLabel(p string) string {
	switch p {
	case "p0":
		return "🔴 **[P0 - Critical]**"
	case "p1":
		return "🟠 **[P1 - High]**"
	case "p2":
		return "🟡 **[P2 - Medium]**"
	default:
		return "🟢 **[P3 - Low]**"
	}
}

func priorityToSeverity(p string) string {
	switch p {
	case "p0", "p1":
		return "error"
	case "p2":
		return "warning"
	default:
		return "info"
	}
}

func sideOrDefault(side string) string {
	if side == "LEFT" || side == "RIGHT" {
		return side
	}
	return "RIGHT"
}

func buildRAGQuery(req AnalysisRequest) string {
	files := make([]string, 0, len(req.Diff))
	for _, f := range req.Diff {
		files = append(files, f.Filename)
	}
	return fmt.Sprintf("PR: %s\nFiles: %s\nDescription: %s",
		req.Title, strings.Join(files, ", "), req.Body)
}

func formatRAGContext(docs []rag.Document) string {
	var sb strings.Builder
	for i, d := range docs {
		kind, _ := d.Metadata["kind"].(string)
		if kind == "fix" {
			fmt.Fprintf(&sb, "%d. [Known fix] %s\n", i+1, d.Content)
		} else {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, d.Content)
		}
	}
	return sb.String()
}

func formatDiff(files []github.FileDiff) string {
	var sb strings.Builder
	for _, f := range files {
		fmt.Fprintf(&sb, "--- %s (%s +%d -%d)\n", f.Filename, f.Status, f.Additions, f.Deletions)
		if f.Patch != "" {
			sb.WriteString(f.Patch)
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}
