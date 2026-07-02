package jobs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/ai/embeddings"
	"github.com/Astraxx04/pr-reviewer/internal/ai/rag"
	"github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// IndexRepoJobArgs is enqueued when a repo is enabled or on the weekly periodic schedule.
type IndexRepoJobArgs struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	RepoID uint   `json:"repo_id"`
	SHA    string `json:"sha,omitempty"` // empty = fetch default branch HEAD
}

func (IndexRepoJobArgs) Kind() string { return "index_repo" }

// IndexRepoWorker fetches the full repository file tree, chunks each code file,
// embeds the chunks, and upserts them into code_embeddings.
type IndexRepoWorker struct {
	river.WorkerDefaults[IndexRepoJobArgs]

	TokenCache    *gh.InstallationTokenCache
	DB            *gorm.DB
	Indexer       *rag.Indexer
	Log           *logger.Logger
	EncryptionKey string
}

// Timeout overrides River's 1-minute default. Indexing a whole repo makes one
// embedding API call per file, which routinely takes several minutes — the
// default would cancel the job mid-run (leaving files unindexed and the embedding
// signature unwritten).
func (w *IndexRepoWorker) Timeout(*river.Job[IndexRepoJobArgs]) time.Duration {
	return 30 * time.Minute
}

// codeExts is the set of file extensions worth indexing.
var codeExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rb": true, ".java": true, ".kt": true, ".rs": true,
	".c": true, ".cpp": true, ".h": true, ".cs": true, ".swift": true,
	".php": true, ".scala": true, ".ex": true, ".exs": true, ".sh": true,
	".yaml": true, ".yml": true, ".json": true, ".sql": true, ".md": true,
}

// maxIndexFileSize is the largest file (bytes) we will embed.
const maxIndexFileSize = 100 * 1024

func (w *IndexRepoWorker) Work(ctx context.Context, job *river.Job[IndexRepoJobArgs]) error {
	args := job.Args
	w.Log.Info("index repo job started", "owner", args.Owner, "repo", args.Repo)

	instClient, err := db.ResolveInstallationClient(ctx, w.DB, w.EncryptionKey, w.TokenCache)
	if err != nil {
		w.Log.Error("failed to resolve GitHub installation client", "error", err)
		w.setStatus(ctx, args.RepoID, "error")
		return err
	}

	w.setStatus(ctx, args.RepoID, "indexing")

	// Fail fast if the configured embedder produces a dimension the schema cannot
	// store. Without this, every insert would be rejected by pgvector and the repo
	// would be marked "indexed" with an empty index.
	if dim := w.Indexer.EmbedderDim(); dim != 0 && dim != embeddings.EmbeddingDim {
		w.setStatus(ctx, args.RepoID, "error")
		return fmt.Errorf("index repo: embedding model produces %d dims but schema requires %d — "+
			"configure a compatible embedding model", dim, embeddings.EmbeddingDim)
	}

	// If the repo was previously indexed with a different embedder, the stored
	// vectors are not comparable to new ones — purge them and re-index from scratch.
	sig := w.Indexer.EmbedderID()
	var existing models.Repository
	if err := w.DB.WithContext(ctx).Select("embedding_model").First(&existing, args.RepoID).Error; err == nil {
		if existing.EmbeddingModel != "" && existing.EmbeddingModel != sig {
			w.Log.Info("embedding provider changed; purging stale index",
				"repo_id", args.RepoID, "from", existing.EmbeddingModel, "to", sig)
			if err := w.Indexer.PurgeRepo(ctx, args.RepoID); err != nil {
				w.Log.Error("index repo: purge stale embeddings failed", "error", err)
			}
		}
	}

	sha := args.SHA
	if sha == "" {
		var err error
		sha, err = instClient.GetDefaultBranchSHA(ctx, args.Owner, args.Repo)
		if err != nil {
			w.setStatus(ctx, args.RepoID, "error")
			return fmt.Errorf("index repo: get default branch SHA: %w", err)
		}
	}

	entries, err := instClient.GetRepoTreeEntries(ctx, args.Owner, args.Repo, sha)
	if err != nil {
		w.setStatus(ctx, args.RepoID, "error")
		return fmt.Errorf("index repo: get tree: %w", err)
	}

	indexed, failed := 0, 0
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Path))
		if !codeExts[ext] || entry.Size > maxIndexFileSize {
			continue
		}
		content, err := instClient.GetFileContent(ctx, args.Owner, args.Repo, entry.Path)
		if err != nil {
			w.Log.Error("index repo: fetch file failed", "path", entry.Path, "error", err)
			continue
		}
		if err := w.Indexer.IndexFile(ctx, args.RepoID, entry.Path, content); err != nil {
			w.Log.Error("index repo: index file failed", "path", entry.Path, "error", err)
			failed++
			continue
		}
		indexed++
	}

	// If every candidate file failed to index, the index is empty — surface that
	// as an error rather than reporting a healthy "indexed" state.
	if indexed == 0 && failed > 0 {
		w.setStatus(ctx, args.RepoID, "error")
		return fmt.Errorf("index repo: all %d files failed to index", failed)
	}

	// Record the embedder signature so a later provider/model change is detected.
	if err := w.DB.WithContext(ctx).Model(&models.Repository{}).
		Where("id = ?", args.RepoID).
		Updates(map[string]any{"embedding_model": sig, "embedding_dim": embeddings.EmbeddingDim}).Error; err != nil {
		w.Log.Error("index repo: record embedding signature failed", "error", err)
	}

	w.Log.Info("index repo job complete", "files", indexed, "failed", failed, "owner", args.Owner, "repo", args.Repo)
	w.setStatus(ctx, args.RepoID, "indexed")
	return nil
}

func (w *IndexRepoWorker) setStatus(ctx context.Context, repoID uint, status string) {
	if w.DB == nil || repoID == 0 {
		return
	}
	w.DB.WithContext(ctx).Model(&models.Repository{}).
		Where("id = ?", repoID).Update("indexing_status", status)
}

// IndexAllReposJobArgs is enqueued by the weekly periodic job to re-index all enabled repos.
type IndexAllReposJobArgs struct{}

func (IndexAllReposJobArgs) Kind() string { return "index_all_repos" }

// IndexAllReposWorker fans out an IndexRepoJob for every enabled repository.
type IndexAllReposWorker struct {
	river.WorkerDefaults[IndexAllReposJobArgs]

	DB       *gorm.DB
	Enqueuer JobEnqueuer
	Log      *logger.Logger
}

func (w *IndexAllReposWorker) Work(ctx context.Context, job *river.Job[IndexAllReposJobArgs]) error {
	if w.DB == nil || w.Enqueuer == nil {
		return nil
	}
	var repos []models.Repository
	w.DB.WithContext(ctx).Where("enabled = true").Find(&repos)

	enqueued := 0
	for _, r := range repos {
		if _, err := w.Enqueuer.Insert(ctx, IndexRepoJobArgs{
			Owner:  r.Owner,
			Repo:   r.Name,
			RepoID: r.ID,
		}, nil); err != nil {
			w.Log.Error("index_all_repos: failed to enqueue", "repo", r.Name, "error", err)
			continue
		}
		enqueued++
	}
	w.Log.Info("index_all_repos: enqueued index jobs", "count", enqueued)
	return nil
}
