package embeddings

import "context"

// EmbeddingDim is the fixed vector dimension of the code_embeddings column
// (vector(1536)). An embedder that produces a different dimension cannot be
// stored without a schema migration, so the indexer rejects it.
const EmbeddingDim = 1536

// Embedder defines the interface for generating embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dim reports the output dimension the configured model is expected to
	// produce, or 0 if unknown. Used to fail fast on an incompatible provider
	// before any rows are written; the actual output length is still validated.
	Dim() int
	// ID is a stable "<provider>/<model>" signature used to detect when a repo's
	// embedding provider or model has changed and its index must be rebuilt.
	ID() string
}

// knownDims maps embedding model names to their output dimension. Models not
// listed report 0 (unknown) and are validated by actual output length instead.
var knownDims = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
	"nomic-embed-text":       768,
	"mxbai-embed-large":      1024,
	"all-minilm":             384,
}

// dimForModel returns the known output dimension for a model, or 0 if unknown.
func dimForModel(model string) int { return knownDims[model] }
