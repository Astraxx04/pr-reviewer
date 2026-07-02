package db

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	pgxCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	// Direct connection to Postgres: use the default extended query protocol,
	// which caches prepared statements for faster repeated queries.
	sqlDB := stdlib.OpenDB(*pgxCfg)

	// Keep a pool of idle connections so requests don't pay connection
	// setup cost on every query.
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("db: failed to connect: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}
	return db, nil
}

// EnsurePgVector attempts to enable the pgvector extension and reports the result.
// When installed is false, the vector(1536) column on CodeEmbedding cannot be
// created, so AutoMigrate will fail and the RAG features are unavailable. err
// carries the reason the extension could not be enabled (e.g. not installed on
// the server, or insufficient privileges).
func EnsurePgVector(db *gorm.DB) (installed bool, version string, err error) {
	if e := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; e != nil {
		err = e
	}
	var v string
	if e := db.Raw("SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&v).Error; e != nil && err == nil {
		err = e
	}
	if v == "" {
		return false, "", err
	}
	return true, v, nil
}

// Models returns every GORM model managed by the application, in migration
// order. It is the single source of truth for AutoMigrate, which is used to
// generate the baseline migration (see internal/db/migrations).
func Models() []any {
	return []any{
		&models.Session{},
		&models.User{},
		&models.Installation{},
		&models.ProviderConfig{},
		&models.Repository{},
		&models.PullRequest{},
		&models.Review{},
		&models.ReviewComment{},
		&models.WebhookDelivery{},
		&models.AssignmentRule{},
		&models.Assignment{},
		&models.Invite{},
		&models.RepoAccess{},
		&models.CodeEmbedding{},
		&models.SystemConfig{},
		&models.GithubAppConfig{},
		&models.BotComment{},
		&models.BotReply{},
		&models.ProviderHealth{},
		&models.NotificationConfig{},
		&models.CommentFeedback{},
		&models.AuditLog{},
		&models.APIToken{},
		&models.OIDCConfig{},
		&models.JiraConfig{},
		&models.SlackAppConfig{},
	}
}

// AutoMigrate derives the schema directly from the GORM models. It is NOT the
// runtime migration path — the application uses versioned SQL migrations
// (RunMigrations). AutoMigrate exists to generate the baseline migration: run
// it against an empty database and pg_dump the result. See internal/db/db.md.
func AutoMigrate(db *gorm.DB) error {
	// pgvector extension — required for CodeEmbedding. Non-fatal if not available.
	_ = db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error

	if err := db.AutoMigrate(Models()...); err != nil {
		return err
	}
	// HNSW index for fast vector similarity (requires pgvector >= 0.5.0; non-fatal if unsupported).
	_ = db.Exec("CREATE INDEX IF NOT EXISTS code_embeddings_hnsw ON code_embeddings USING hnsw (embedding vector_cosine_ops)").Error
	// Partial unique index: one pending invite per email.
	_ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS invites_pending_email_uniq
		ON invites (email)
		WHERE accepted_at IS NULL`).Error
	return nil
}
