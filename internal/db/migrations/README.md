# Database Migrations

This directory holds the application's database schema as **versioned SQL
migrations**, applied by [golang-migrate](https://github.com/golang-migrate/migrate).
This document explains how the system works end to end and how to add a change.

---

## TL;DR

```bash
make migrate-new name=add_widget_table   # scaffold 000002_add_widget_table.{up,down}.sql
# ... write the SQL in both files ...
# ... update the matching struct in internal/db/models/models.go ...
make migrate            # apply (app schema + River queue)
make migrate-status     # check current vs latest version
make migrate-check      # verify migrations and models haven't drifted (needs DATABASE_URL)
```

Golden rules:

1. **Never edit a migration that has been merged/applied.** Add a new one.
2. **Every `.up.sql` needs a working `.down.sql`** that reverses it.
3. **Keep `models.go` in sync** with every migration — the drift check enforces this.
4. Migrations are an **explicit step**. The server does *not* migrate on boot; it
   verifies the schema and exits if the DB is behind.

---

## Architecture

There are **two independent migration tracks**, applied together by `migrate up`:

| Track | Owns | Tooling | Tracking table |
|-------|------|---------|----------------|
| **App schema** | All application tables (`users`, `repositories`, …) | golang-migrate, SQL files in this dir | `schema_migrations` |
| **River queue** | `river_*` tables | `rivermigrate` (River's own migrator) | `river_migration` |

River manages its own schema across River version upgrades, so its tables are
**deliberately not** in these SQL files. `migrate up` runs both; everything else
below concerns the app-schema track.

### How the SQL gets into the binary

The files here are compiled into the binary via `go:embed` in
[`../migrate.go`](../migrate.go):

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS
```

There is no separate migrations directory to ship or mount — the running binary
already contains every migration. golang-migrate reads them through its `iofs`
source driver and applies them with its `postgres` database driver.

### The functions (`internal/db/migrate.go`)

| Function | What it does |
|----------|--------------|
| `RunMigrations(dsn)` | Apply all pending app migrations. No-op if already current. **The runtime path.** |
| `VerifySchema(dsn)` | Confirm the DB is at the latest version and not dirty. Returns an actionable error otherwise. Used by the server to fail fast. |
| `MigrateStatus(dsn)` | Return current version, dirty flag, whether anything is applied, and latest. |
| `MigrateDownOne(dsn)` | Roll back the most recent migration. |
| `ForceMigrationVersion(dsn, v)` | Set the version and clear the dirty flag (recovery only). |
| `LatestMigrationVersion()` | Highest version embedded in the binary. |

---

## File naming

```
NNNNNN_short_description.up.sql     # forward change
NNNNNN_short_description.down.sql   # exact reverse
```

- `NNNNNN` is a **zero-padded, sequential** 6-digit version (`000001`, `000002`, …).
  `make migrate-new` computes the next number automatically.
- The description is for humans; only the leading number is significant to the tool.
- `up` and `down` files **must come in pairs** with the same version and name.

Current baseline: [`000001_init.up.sql`](000001_init.up.sql) — the entire schema
as of adoption (see *Regenerating the baseline* below).

---

## How a migration is applied

When `migrate up` (or `RunMigrations`) runs, golang-migrate:

1. Reads the current version from the `schema_migrations` table (created on first
   run). A row is `(version bigint, dirty boolean)`.
2. Finds embedded migrations with a version greater than the current one.
3. For each, in order:
   - marks `schema_migrations` **dirty** at that version,
   - executes the `.up.sql` file,
   - on success, clears the dirty flag and advances the version.

**Each migration file runs as a single batch in one implicit transaction** — if
any statement fails, the whole file rolls back, but the version is left **dirty**
so the next run refuses to proceed until you intervene (see *Recovering from a
dirty state*).

> ⚠️ Because a migration runs inside a transaction, statements that **cannot run
> in a transaction** will fail: `CREATE INDEX CONCURRENTLY`, `CREATE DATABASE`,
> `VACUUM`, `ALTER TYPE … ADD VALUE` (older Postgres). Split those into their own
> migration or avoid them.

---

## Where migrations run

| Context | How |
|---------|-----|
| **Local dev** | `make migrate` |
| **Server boot** | Does **not** migrate. Calls `VerifySchema` and exits with `schema is behind … run 'migrate up'` if the DB isn't current. |
| **Docker Compose** | The one-shot `migrate` service (server image with `MIGRATE_ONLY=true`) runs migrations and exits; `app` waits for it via `depends_on: service_completed_successfully`. |
| **Production / deploy** | Run `migrate up` (or a `MIGRATE_ONLY=true` container) as an explicit step **before** rolling out the app. |
| **CI** | The `test` job runs the schema-drift check on every PR. |

Running migrations as a discrete step (rather than on boot) avoids races when
multiple app replicas start at once and makes rollouts predictable.

---

## Creating a migration — step by step

Suppose you want to add a `priority` column to `repositories`.

### 1. Scaffold the files

```bash
make migrate-new name=add_repo_priority
# created internal/db/migrations/000002_add_repo_priority.up.sql and .down.sql
```

### 2. Write the forward SQL (`.up.sql`)

```sql
ALTER TABLE repositories ADD COLUMN priority integer NOT NULL DEFAULT 0;
```

### 3. Write the reverse SQL (`.down.sql`)

```sql
ALTER TABLE repositories DROP COLUMN priority;
```

The `down` must leave the schema exactly as it was before the `up`. Test it:
`make migrate` then `make migrate-down` should round-trip cleanly.

### 4. Update the model

Edit the matching struct in [`../models/models.go`](../models/models.go) so the
ORM and the database agree:

```go
type Repository struct {
    // ...
    Priority int `gorm:"not null;default:0"`
}
```

The models back every query **and** are the source of truth for the drift check.
A migration without a model change (or vice versa) will fail CI.

### 5. Apply and verify

```bash
make migrate          # apply
make migrate-status   # should show "version: 2 / 2 (clean)"
make migrate-check    # migrations and models agree (no drift)
```

### 6. Commit

Commit the two `.sql` files **and** the `models.go` change together.

---

## Keeping models and migrations in sync (drift detection)

`AutoMigrate` (GORM) and the SQL migrations are **independent**: changing a struct
does not change the schema until you write a migration. To stop them silently
diverging, `TestSchemaDrift` ([`../drift_test.go`](../drift_test.go)):

1. Builds the schema two ways into throwaway databases — once via `RunMigrations`,
   once via `AutoMigrate` from the models.
2. Introspects both (columns: name, type, nullability, default; plus indexes).
3. Fails if they differ, printing exactly what's missing on each side.

Run it locally with `make migrate-check` (needs a `DATABASE_URL` the test user can
`CREATE DATABASE` on). CI runs it on every PR in the `test` job.

`AutoMigrate` itself is **not** a runtime path anymore — it exists only to back
the drift check and to regenerate the baseline.

---

## Command reference

### `cmd/migrate` CLI

| Command | Action |
|---------|--------|
| `migrate up` | Apply all pending app migrations **and** River queue migrations. |
| `migrate down` | Roll back the most recent app migration. |
| `migrate status` (or `version`) | Print applied vs latest version and dirty state. |
| `migrate force <v>` | Set the version and clear the dirty flag — recovery only. |

Reads `DATABASE_URL` from the environment (and `.env` via godotenv).

### Make targets

| Target | Wraps |
|--------|-------|
| `make migrate` | `migrate up` |
| `make migrate-down` | `migrate down` |
| `make migrate-status` | `migrate status` |
| `make migrate-check` | `TestSchemaDrift` (drift detection) |
| `make migrate-new name=<desc>` | Scaffold the next `up`/`down` pair |

---

## Recovering from a dirty state

If a migration fails partway, `schema_migrations.dirty` is left `true` and
`migrate up` / server boot will refuse to proceed:

```
db: schema is dirty at version N — fix the DB, then 'migrate force N' and 'migrate up'
```

To recover:

1. **Inspect the database** and finish or undo the partial change by hand so the
   actual schema matches a known version `M` (usually `N-1` if nothing applied, or
   `N` if the change actually landed).
2. `migrate force M` — sets the version to `M` and clears the dirty flag. This
   updates bookkeeping only; it does **not** run any SQL.
3. `migrate up` — re-apply forward from `M`.

Because a migration runs in a transaction, a clean failure usually rolls the SQL
back entirely, so forcing to the previous version and re-running (after fixing the
SQL) is the common case.

---

## Regenerating the baseline (rare)

`000001_init.up.sql` was generated from `AutoMigrate`, not hand-written. You only
need to redo this if you're rebuilding the baseline from scratch (you normally
**add** migrations instead). The process:

1. Start an empty Postgres with the `vector` extension available
   (`pgvector/pgvector:pg16`).
2. Point `AutoMigrate` at it (e.g. run the old AutoMigrate-based path, or a small
   throwaway program calling `db.AutoMigrate`).
3. `pg_dump --schema-only --no-owner --no-privileges --no-comments`.
4. Strip River objects (`river*` tables, the `river_job_state` type, the
   `river_job_state_in_bitmask` function) and psql meta-commands (`\restrict`,
   `\unrestrict`).
5. Save as `000001_init.up.sql`; write a `down` that drops the tables and the
   `vector` extension.

---

## See also

- [`../db.md`](../db.md) — connection layer and the migration design overview.
- [`../migrate.go`](../migrate.go) — the embedding and the migration functions.
- [`../../../cmd/migrate/main.go`](../../../cmd/migrate/main.go) — the CLI.
