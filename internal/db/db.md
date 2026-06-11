# `internal/db/db.go` — Database connection & schema migration

This file is the database access layer's foundation. It exposes exactly two
exported functions:

| Function | Responsibility |
|----------|----------------|
| `Connect(dsn string) (*gorm.DB, error)` | Open a tuned, verified connection to Postgres and return a GORM handle. |
| `RunMigrations(dsn string) error` | Apply pending app migrations (golang-migrate, embedded SQL). The runtime migration path. |
| `VerifySchema(dsn string) error` | Confirm the DB is at the latest version and not dirty; used by the server to fail fast. |
| `AutoMigrate(db *gorm.DB) error` | Derive the schema from GORM models. **Not** a runtime path — used only to generate the baseline migration (see below). |

> **Migrations are versioned SQL** under `internal/db/migrations/`, embedded into
> the binary and applied by [golang-migrate](https://github.com/golang-migrate/migrate).
> They are an **explicit step** — the server verifies the schema on boot but does
> not migrate. River's queue tables are migrated separately by `rivermigrate`.
>
> 📖 **Full guide:** [`migrations/README.md`](migrations/README.md) — how the
> system works end to end and how to author a migration.

It targets a **direct PostgreSQL connection** (no connection pooler such as
PgBouncer in between). Other secrets-related helpers in this package (e.g.
`db.Decrypt`) live in *separate files* — this file is only about connecting and
migrating.

---

## Imports

```go
"github.com/jackc/pgx/v5"          // low-level Postgres driver + DSN parsing
"github.com/jackc/pgx/v5/stdlib"   // adapts pgx to Go's database/sql interface
"gorm.io/driver/postgres"          // GORM's Postgres driver
"gorm.io/gorm"                     // the ORM
"gorm.io/gorm/logger"              // GORM query logging (silenced here)
"pr-reviewer/internal/db/models"   // all the table model structs
```

The notable choice: rather than letting GORM open the connection from a DSN
string directly, the code opens a `*sql.DB` through **pgx** first and hands that
pre-configured connection to GORM. This gives explicit control over the pgx
query protocol and the connection pool.

---

## `Connect(dsn string) (*gorm.DB, error)`

Builds the database handle in four steps.

### 1. Parse the DSN with pgx (lines 17–20)

```go
pgxCfg, err := pgx.ParseConfig(dsn)
if err != nil {
    return nil, fmt.Errorf("db: parse config: %w", err)
}
```

Parses the connection string (`DATABASE_URL`) into a pgx config struct. Errors
are wrapped with `%w` so callers can unwrap the underlying cause.

### 2. Open via the standard library adapter (lines 21–23)

```go
// Direct connection to Postgres: use the default extended query protocol,
// which caches prepared statements for faster repeated queries.
sqlDB := stdlib.OpenDB(*pgxCfg)
```

`stdlib.OpenDB` turns the pgx config into a standard `*sql.DB`. Because no
`DefaultQueryExecMode` override is set, pgx uses its **default extended query
protocol**, which caches server-side prepared statements — the normal, faster
path for a direct Postgres connection.

> Historical note: this previously forced `QueryExecModeSimpleProtocol` to work
> around Neon's PgBouncer (transaction-pooling mode), which doesn't support
> prepared statements and would raise `cached plan must not change result type`
> after schema changes. That workaround was removed when the project moved to a
> direct connection. If you ever route through a transaction-mode pooler again,
> that error will return and the simple-protocol setting would be needed.

### 3. Tune the connection pool (lines 25–30)

```go
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(5)
sqlDB.SetConnMaxLifetime(time.Hour)
sqlDB.SetConnMaxIdleTime(30 * time.Minute)
```

- **`MaxOpenConns(25)`** — at most 25 concurrent connections to Postgres.
- **`MaxIdleConns(5)`** — keep up to 5 warm idle connections so most queries skip
  connection-setup latency.
- **`ConnMaxLifetime(1h)`** — recycle any connection older than an hour.
- **`ConnMaxIdleTime(30m)`** — close connections idle longer than 30 minutes.

> Sizing caveat: the server also runs a **separate** pgx pool for the River job
> queue (see `cmd/server/main.go`). The sum of both pools must stay under
> Postgres `max_connections` (default 100).

### 4. Hand off to GORM, then verify (lines 32–41)

```go
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
```

- Wraps the pre-built `*sql.DB` in GORM via `postgres.Config{Conn: sqlDB}`.
- **`LogMode(logger.Silent)`** disables GORM's built-in query logging — the
  project uses its own structured logger elsewhere.
- **`sqlDB.Ping()`** forces a real round-trip so a bad DSN or unreachable
  database fails *here*, at startup, rather than on the first query.

---

## `AutoMigrate(db *gorm.DB) error`

Brings the schema up to date. Called both by the server on startup
(`cmd/server/main.go`) and by the standalone `cmd/migrate` tool. It is
**idempotent** — safe to run repeatedly.

### 1. Enable the pgvector extension (lines 45–46)

```go
_ = db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
```

Creates the `vector` extension required by the `CodeEmbedding` model (the RAG
feature). The error is **deliberately ignored**: if the DB role lacks
permission or the extension isn't installed, migration still proceeds — only the
vector features are unavailable.

### 2. Migrate all model tables (lines 48–76)

A single `db.AutoMigrate(...)` call passes all 24 model structs. GORM creates
or alters tables and columns to match each struct. This is the **single source
of truth** for which tables exist:

```
Session, User, Installation, ProviderConfig, Repository, PullRequest,
Review, ReviewComment, WebhookDelivery, AssignmentRule, Assignment,
TeamMember, CodeEmbedding, SystemConfig, GithubAppConfig, BotComment,
BotReply, ProviderHealth, NotificationConfig, CommentFeedback,
AuditLog, APIToken, OIDCConfig, JiraConfig, SlackAppConfig
```

The ordering roughly follows dependency order (e.g. `User`/`Installation`
before `Repository` → `PullRequest` → `Review` → `ReviewComment`), though GORM
resolves most foreign-key relationships itself.

### 3. Build the HNSW vector index (lines 77–78)

```go
_ = db.Exec("CREATE INDEX IF NOT EXISTS code_embeddings_hnsw ON code_embeddings USING hnsw (embedding vector_cosine_ops)").Error
```

Creates an **HNSW** index for fast approximate cosine-similarity search over
embeddings — this is what makes RAG retrieval fast. Also error-ignored, because
HNSW requires pgvector ≥ 0.5.0; on older versions migration still succeeds and
queries fall back to slower exact search.

---

## Key behaviors & caveats

- **Built for a direct Postgres connection.** Extended protocol + longer idle
  times assume no transaction-mode pooler sits in front of the database. The
  actual "no pooler" guarantee depends on `DATABASE_URL` pointing at the direct
  host/port (typically `5432`), not a pooler endpoint.
- **Versioned migrations, applied explicitly.** Schema changes are SQL files in
  `internal/db/migrations/` (`NNNNNN_name.up.sql` / `.down.sql`), applied by
  golang-migrate. The server calls `VerifySchema` and **exits** if the DB is
  behind — it never migrates on boot. Run migrations via `make migrate`
  (`migrate up`) or the docker `migrate` service (server image with
  `MIGRATE_ONLY=true`).
- **Graceful vector degradation.** `EnsurePgVector` is best-effort at boot: the
  system runs even without pgvector, just without RAG. The `vector` extension
  itself is created by the baseline migration.
- **Fail-fast connections.** The explicit `Ping()` ensures configuration errors
  surface at startup.

---

## Migration workflow

**Add a schema change**

1. `make migrate-new name=add_widgets` — scaffolds the next `up`/`down` pair.
2. Write the forward SQL in `.up.sql` and its inverse in `.down.sql`.
3. Keep the GORM models in `models.go` in sync (they back queries and the
   baseline). `Models()` in `db.go` lists every model.
4. `make migrate` to apply, `make migrate-status` to check, `make migrate-down`
   to roll back one step.

**Regenerate the baseline** (rare — only if rebuilding `000001` from scratch):
run `AutoMigrate` against an empty database and `pg_dump --schema-only`, then
strip River objects (`river*`, `river_job_state`, psql `\restrict` lines). This
is how `000001_init.up.sql` was produced.

**CLI** (`cmd/migrate`): `up` (app + River), `down`, `status`, `force <v>`
(clear a dirty version after manual recovery).

> ⚠️ golang-migrate and `AutoMigrate` are now **independent** — a model change is
> not a schema change until you write a migration for it. The models remain the
> source of truth for the baseline and for query mapping only.

**Drift detection** (`make migrate-check`, and CI's `test` job on every PR):
`TestSchemaDrift` builds the schema two ways into throwaway databases — once via
the migrations, once via `AutoMigrate` from the models — and compares columns
and indexes. If you change a model without writing a matching migration (or vice
versa), it fails with the exact diff. This closes the gap the independence above
opens.
