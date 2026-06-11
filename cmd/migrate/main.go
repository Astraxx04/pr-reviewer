// migrate is a small CLI for managing the PR Reviewer database schema.
//
// Usage:
//
//	migrate up         – apply all pending app migrations + River queue migrations
//	migrate down       – roll back the most recent app migration
//	migrate status     – print the applied/latest migration version and table list
//	migrate force <v>  – set the migration version and clear the dirty flag (recovery)
//
// App migrations are versioned SQL files embedded from internal/db/migrations
// and applied by golang-migrate. River's queue tables are migrated separately
// by rivermigrate.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"pr-reviewer/internal/db"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fatalf("DATABASE_URL is not set\n")
	}

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		runUp(dsn)
	case "down":
		runDown(dsn)
	case "status", "version":
		runStatus(dsn)
	case "force":
		runForce(dsn)
	default:
		fatalf("unknown command %q — use 'up', 'down', 'status', or 'force'\n", cmd)
	}
}

func runUp(dsn string) {
	if err := db.RunMigrations(dsn); err != nil {
		fatalf("%v\n", err)
	}
	fmt.Println("App schema migrations applied.")

	// Apply River queue migrations (managed separately from app migrations).
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fatalf("pgxpool: %v\n", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		fatalf("river migrator: %v\n", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		fatalf("river migrate: %v\n", err)
	}
	fmt.Println("River queue migrations applied.")
}

func runDown(dsn string) {
	if err := db.MigrateDownOne(dsn); err != nil {
		fatalf("%v\n", err)
	}
	fmt.Println("Rolled back one app migration.")
}

func runForce(dsn string) {
	if len(os.Args) < 3 {
		fatalf("force requires a version: migrate force <version>\n")
	}
	v, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fatalf("invalid version %q: %v\n", os.Args[2], err)
	}
	if err := db.ForceMigrationVersion(dsn, v); err != nil {
		fatalf("%v\n", err)
	}
	fmt.Printf("Forced migration version to %d (dirty flag cleared).\n", v)
}

func runStatus(dsn string) {
	current, dirty, applied, latest, err := db.MigrateStatus(dsn)
	if err != nil {
		fatalf("%v\n", err)
	}
	if !applied {
		fmt.Printf("No migrations applied yet (latest available: %d). Run 'migrate up'.\n", latest)
		return
	}
	state := "clean"
	if dirty {
		state = "DIRTY — run 'migrate force' then 'migrate up'"
	}
	fmt.Printf("Migration version: %d / %d  (%s)\n", current, latest, state)
	if current < latest {
		fmt.Println("Schema is BEHIND — run 'migrate up'.")
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format, args...)
	os.Exit(1)
}
