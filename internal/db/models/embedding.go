package models

import "time"

// CodeEmbedding stores a vector embedding for a code review comment or diff chunk.
// Requires the pgvector Postgres extension (CREATE EXTENSION IF NOT EXISTS vector).
type CodeEmbedding struct {
	ID        uint      `gorm:"primarykey"`
	RepoID    uint      `gorm:"index;not null"`
	Content   string    `gorm:"not null"` // the text that was embedded
	Embedding string    `gorm:"type:vector(1536);not null"`
	Kind      string    `gorm:"not null"` // "comment" | "diff"
	Path      string    // source file path, if applicable
	CreatedAt time.Time
}
