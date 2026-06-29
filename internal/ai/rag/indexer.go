package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/ai/embeddings"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
)

// Indexer embeds and persists code chunks and review comments for future RAG retrieval.
type Indexer struct {
	db       *gorm.DB
	embedder embeddings.Embedder
}

func NewIndexer(db *gorm.DB, embedder embeddings.Embedder) *Indexer {
	return &Indexer{db: db, embedder: embedder}
}

// EmbedderID returns the "<provider>/<model>" signature of the configured embedder.
func (idx *Indexer) EmbedderID() string { return idx.embedder.ID() }

// EmbedderDim returns the configured embedder's expected output dimension (0 if unknown).
func (idx *Indexer) EmbedderDim() int { return idx.embedder.Dim() }

// PurgeRepo deletes every embedding stored for a repo. Used when the embedding
// provider/model changes and the old vectors are no longer comparable.
func (idx *Indexer) PurgeRepo(ctx context.Context, repoID uint) error {
	return idx.db.WithContext(ctx).Exec("DELETE FROM code_embeddings WHERE repo_id = ?", repoID).Error
}

// validateDim rejects a vector whose dimension does not match the code_embeddings
// column. This is the guard that turns a silent pgvector insert failure (and the
// resulting empty/half-written index) into a clear, surfaced error.
func validateDim(vec []float32) error {
	if len(vec) != embeddings.EmbeddingDim {
		return fmt.Errorf("embedding dimension %d does not match required %d "+
			"(incompatible embedding model?)", len(vec), embeddings.EmbeddingDim)
	}
	return nil
}

// IndexComments embeds each review comment and stores it in code_embeddings.
func (idx *Indexer) IndexComments(ctx context.Context, repoID uint, comments []gh.ReviewComment) error {
	for _, c := range comments {
		text := fmt.Sprintf("[%s] %s", c.Path, c.Body)
		vec, err := idx.embedder.Embed(ctx, text)
		if err != nil {
			// Transient embed failure for a single comment — skip it, best effort.
			continue
		}
		if err := validateDim(vec); err != nil {
			return fmt.Errorf("index comments: %w", err)
		}
		if err := idx.db.WithContext(ctx).Exec(
			"INSERT INTO code_embeddings (repo_id, content, embedding, kind, path, created_at) "+
				"VALUES (?, ?, '"+formatVector(vec)+"'::vector, ?, ?, ?)",
			repoID, text, "comment", c.Path, time.Now(),
		).Error; err != nil {
			return fmt.Errorf("index comments: insert: %w", err)
		}
	}
	return nil
}

// IndexFile chunks the given file content and upserts each chunk into code_embeddings.
// Existing chunks for this repo+path are deleted first — but only after every new
// vector has been validated, so a dimension mismatch can never delete good data
// without replacing it.
func (idx *Indexer) IndexFile(ctx context.Context, repoID uint, path, content string) error {
	chunks := chunkText(content, 200, 20)
	if len(chunks) == 0 {
		return nil
	}
	vecs, err := idx.embedder.EmbedBatch(ctx, chunks)
	if err != nil {
		return err
	}
	// Validate ALL dimensions before any destructive write.
	for i := range vecs {
		if err := validateDim(vecs[i]); err != nil {
			return fmt.Errorf("index file %s: %w", path, err)
		}
	}
	if err := idx.db.WithContext(ctx).Exec(
		"DELETE FROM code_embeddings WHERE repo_id = ? AND path = ? AND kind = 'chunk'",
		repoID, path,
	).Error; err != nil {
		return fmt.Errorf("index file %s: delete: %w", path, err)
	}
	now := time.Now()
	for i, chunk := range chunks {
		if i >= len(vecs) {
			break
		}
		if err := idx.db.WithContext(ctx).Exec(
			"INSERT INTO code_embeddings (repo_id, content, embedding, kind, path, created_at) "+
				"VALUES (?, ?, '"+formatVector(vecs[i])+"'::vector, 'chunk', ?, ?)",
			repoID, chunk, path, now,
		).Error; err != nil {
			return fmt.Errorf("index file %s: insert: %w", path, err)
		}
	}
	return nil
}

// IndexFix stores a known-good resolution pattern when a bot comment is addressed by a developer.
func (idx *Indexer) IndexFix(ctx context.Context, repoID uint, path, commentBody string) error {
	text := fmt.Sprintf("[FIX applied in %s] %s", path, commentBody)
	vec, err := idx.embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	if err := validateDim(vec); err != nil {
		return fmt.Errorf("index fix: %w", err)
	}
	if err := idx.db.WithContext(ctx).Exec(
		"INSERT INTO code_embeddings (repo_id, content, embedding, kind, path, created_at) "+
			"VALUES (?, ?, '"+formatVector(vec)+"'::vector, 'fix', ?, ?)",
		repoID, text, path, time.Now(),
	).Error; err != nil {
		return fmt.Errorf("index fix: insert: %w", err)
	}
	return nil
}

// chunkText splits text into overlapping chunks of chunkLines lines with overlapLines overlap.
func chunkText(text string, chunkLines, overlapLines int) []string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	var chunks []string
	for start := 0; start < len(lines); start += chunkLines - overlapLines {
		end := min(start+chunkLines, len(lines))
		chunk := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(lines) {
			break
		}
	}
	return chunks
}
