package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// migrationsFS holds the versioned SQL migrations applied by golang-migrate.
// River's queue tables are NOT here — they are migrated separately by
// rivermigrate (see cmd/migrate and cmd/server).
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// openSQL opens a dedicated *sql.DB for migration work. Callers own the
// returned handle (closed via migrate.Migrate.Close), so it never touches the
// application's GORM connection pool.
func openSQL(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}
	return stdlib.OpenDB(*cfg), nil
}

// newMigrator builds a golang-migrate instance over the embedded SQL files.
// Closing the returned *migrate.Migrate also closes sqlDB.
func newMigrator(sqlDB *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("db: load embedded migrations: %w", err)
	}
	driver, err := migratepg.WithInstance(sqlDB, &migratepg.Config{})
	if err != nil {
		return nil, fmt.Errorf("db: migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("db: migrate init: %w", err)
	}
	return m, nil
}

// LatestMigrationVersion reports the highest version embedded in the binary —
// the version the database should be at after a successful migrate up.
func LatestMigrationVersion() (uint, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return 0, err
	}
	defer func() { _ = src.Close() }()

	last, err := src.First()
	if err != nil {
		return 0, fmt.Errorf("db: no embedded migrations found: %w", err)
	}
	for {
		next, err := src.Next(last)
		if err != nil {
			break // os.ErrNotExist marks the end of the chain
		}
		last = next
	}
	return last, nil
}

// RunMigrations applies all pending application migrations. It is a no-op when
// the schema is already current. River migrations are handled separately.
func RunMigrations(dsn string) error {
	sqlDB, err := openSQL(dsn)
	if err != nil {
		return err
	}
	m, err := newMigrator(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: apply migrations: %w", err)
	}
	return nil
}

// VerifySchema confirms the database is migrated to the latest embedded version
// and is not in a dirty state. It applies nothing — migrations are an explicit
// step (migrate up) — and returns an actionable error when the DB is behind so
// the server fails fast at startup instead of running against a stale schema.
func VerifySchema(dsn string) error {
	sqlDB, err := openSQL(dsn)
	if err != nil {
		return err
	}
	m, err := newMigrator(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	defer func() { _, _ = m.Close() }()

	current, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return errors.New("db: no migrations applied — run 'migrate up'")
	}
	if err != nil {
		return fmt.Errorf("db: read schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("db: schema is dirty at version %d — fix the DB, then 'migrate force %d' and 'migrate up'", current, current)
	}
	latest, err := LatestMigrationVersion()
	if err != nil {
		return err
	}
	if current < latest {
		return fmt.Errorf("db: schema is behind (at %d, want %d) — run 'migrate up'", current, latest)
	}
	return nil
}

// MigrateStatus returns the current applied version, whether it is dirty, and
// the latest embedded version. A current of 0 with applied=false means no
// migrations have been applied yet.
func MigrateStatus(dsn string) (current uint, dirty, applied bool, latest uint, err error) {
	latest, err = LatestMigrationVersion()
	if err != nil {
		return 0, false, false, 0, err
	}
	sqlDB, err := openSQL(dsn)
	if err != nil {
		return 0, false, false, latest, err
	}
	m, err := newMigrator(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return 0, false, false, latest, err
	}
	defer func() { _, _ = m.Close() }()

	current, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, false, latest, nil
	}
	if err != nil {
		return 0, false, false, latest, err
	}
	return current, dirty, true, latest, nil
}

// MigrateDownOne rolls back the most recently applied migration.
func MigrateDownOne(dsn string) error {
	sqlDB, err := openSQL(dsn)
	if err != nil {
		return err
	}
	m, err := newMigrator(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: roll back one migration: %w", err)
	}
	return nil
}

// ForceMigrationVersion sets the schema version without running migrations and
// clears the dirty flag. Use only to recover from a failed migration after
// manually reconciling the database.
func ForceMigrationVersion(dsn string, version int) error {
	sqlDB, err := openSQL(dsn)
	if err != nil {
		return err
	}
	m, err := newMigrator(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("db: force version %d: %w", version, err)
	}
	return nil
}
