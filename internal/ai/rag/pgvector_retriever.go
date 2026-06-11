package rag

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"pr-reviewer/internal/ai/embeddings"
	"pr-reviewer/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

type PgvectorRetriever struct {
	db       *gorm.DB
	embedder embeddings.Embedder
}

func NewPgvectorRetriever(db *gorm.DB, embedder embeddings.Embedder) *PgvectorRetriever {
	return &PgvectorRetriever{db: db, embedder: embedder}
}

func (r *PgvectorRetriever) Retrieve(ctx context.Context, repoID uint, query string, topK int) ([]Document, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "rag.retrieve")
	defer span.End()
	span.SetAttributes(
		attribute.Int("rag.repo_id", int(repoID)),
		attribute.Int("rag.top_k", topK),
	)

	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("rag retrieve: embed: %w", err)
	}

	type row struct {
		Content  string
		Kind     string
		Path     string
		Distance float64
	}

	vecStr := formatVector(vec)

	// Use raw SQL: pgvector <=> operator computes cosine distance.
	// vecStr is safe (only digits, brackets, commas) — no injection risk.
	var rows []row
	baseSQL := fmt.Sprintf(
		"SELECT content, kind, path, (embedding <=> '%s'::vector) AS distance "+
			"FROM code_embeddings WHERE repo_id = ? ORDER BY distance LIMIT ?",
		vecStr,
	)
	if repoID == 0 {
		baseSQL = fmt.Sprintf(
			"SELECT content, kind, path, (embedding <=> '%s'::vector) AS distance "+
				"FROM code_embeddings ORDER BY distance LIMIT ?",
			vecStr,
		)
		r.db.WithContext(ctx).Raw(baseSQL, topK).Scan(&rows)
	} else {
		r.db.WithContext(ctx).Raw(baseSQL, repoID, topK).Scan(&rows)
	}

	docs := make([]Document, 0, len(rows))
	for _, row := range rows {
		docs = append(docs, Document{
			Content: row.Content,
			Score:   float32(1 - row.Distance), // convert distance to similarity
			Metadata: map[string]interface{}{
				"kind": row.Kind,
				"path": row.Path,
			},
		})
	}
	span.SetAttributes(attribute.Int("rag.docs_returned", len(docs)))
	return docs, nil
}

// formatVector renders []float32 as a pgvector literal: [0.1,0.2,...].
func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
