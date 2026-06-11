package rag

import "context"

// Document represents a retrieved document.
type Document struct {
	Content  string
	Metadata map[string]interface{}
	Score    float32
}

// Retriever defines the interface for retrieving relevant context.
// repoID scopes results to a single repository (0 = global, no scope).
type Retriever interface {
	Retrieve(ctx context.Context, repoID uint, query string, topK int) ([]Document, error)
}
