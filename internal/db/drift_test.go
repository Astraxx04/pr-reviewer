//go:build integration

package db

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestSchemaDrift guards the one gap that adopting versioned SQL migrations
// opens: a model can change in models.go without anyone writing a matching
// migration. It builds the schema two independent ways into throwaway
// databases — once by applying the migrations (RunMigrations) and once
// straight from the GORM models (AutoMigrate) — then compares the resulting
// tables, columns, and indexes. A mismatch means the migrations and the models
// have drifted; write a migration to reconcile them.
//
// Requires DATABASE_URL in URL form pointing at a Postgres the test user can
// CREATE DATABASE on (the CI postgres service qualifies). Tagged `integration`.
func TestSchemaDrift(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		t.Skip("DATABASE_URL is not in URL form; drift check needs URL form to derive sibling DB names")
	}

	const migratedDB = "drift_migrated"
	const automigrateDB = "drift_automigrate"

	admin, err := openSQL(dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer admin.Close()

	// Fresh throwaway databases for each build path.
	for _, name := range []string{migratedDB, automigrateDB} {
		if _, err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Fatalf("drop %s: %v", name, err)
		}
		if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range []string{migratedDB, automigrateDB} {
			_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
		}
	})

	migratedDSN := withDatabase(t, dsn, migratedDB)
	automigrateDSN := withDatabase(t, dsn, automigrateDB)

	// Build path 1: the migrations.
	if err := RunMigrations(migratedDSN); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// Build path 2: AutoMigrate straight from the models.
	gormDB, err := Connect(automigrateDSN)
	if err != nil {
		t.Fatalf("connect automigrate db: %v", err)
	}
	if err := AutoMigrate(gormDB); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if sqlDB, err := gormDB.DB(); err == nil {
		_ = sqlDB.Close()
	}

	migratedFP := fingerprint(t, migratedDSN)
	automigrateFP := fingerprint(t, automigrateDSN)

	if migratedFP != automigrateFP {
		t.Fatalf("schema drift detected between migrations and models.\n"+
			"A model changed without a matching migration (or vice versa).\n"+
			"Run `make migrate-new name=...` and write the SQL to reconcile.\n\n%s",
			diff(migratedFP, automigrateFP))
	}
}

// withDatabase returns dsn with its database name swapped for db.
func withDatabase(t *testing.T, dsn, db string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	u.Path = "/" + db
	return u.String()
}

// fingerprint returns a stable, sorted textual description of the schema:
// every column (name, type, nullability, default) and every index, excluding
// River's tables and golang-migrate's bookkeeping table.
func fingerprint(t *testing.T, dsn string) string {
	t.Helper()
	conn, err := openSQL(dsn)
	if err != nil {
		t.Fatalf("open %s: %v", dsn, err)
	}
	defer conn.Close()

	var lines []string

	colRows, err := conn.Query(`
		SELECT table_name, column_name, udt_name, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name NOT LIKE 'river%'
		  AND table_name <> 'schema_migrations'`)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	for colRows.Next() {
		var table, col, udt, nullable, def string
		if err := colRows.Scan(&table, &col, &udt, &nullable, &def); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		lines = append(lines, fmt.Sprintf("col %s.%s type=%s null=%s default=%s", table, col, udt, nullable, def))
	}
	if err := colRows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}
	_ = colRows.Close()

	idxRows, err := conn.Query(`
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename NOT LIKE 'river%'
		  AND tablename <> 'schema_migrations'`)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	for idxRows.Next() {
		var name, def string
		if err := idxRows.Scan(&name, &def); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		lines = append(lines, "idx "+def)
	}
	if err := idxRows.Err(); err != nil {
		t.Fatalf("iterate indexes: %v", err)
	}
	_ = idxRows.Close()

	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// diff returns the lines unique to each side, to make a drift failure readable.
func diff(migrated, automigrate string) string {
	inAuto := map[string]bool{}
	for _, l := range strings.Split(automigrate, "\n") {
		inAuto[l] = true
	}
	inMig := map[string]bool{}
	for _, l := range strings.Split(migrated, "\n") {
		inMig[l] = true
	}

	var onlyMigrated, onlyModels []string
	for l := range inMig {
		if !inAuto[l] {
			onlyMigrated = append(onlyMigrated, l)
		}
	}
	for l := range inAuto {
		if !inMig[l] {
			onlyModels = append(onlyModels, l)
		}
	}
	sort.Strings(onlyMigrated)
	sort.Strings(onlyModels)

	var b strings.Builder
	b.WriteString("In migrations but NOT in models (AutoMigrate):\n")
	if len(onlyMigrated) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, l := range onlyMigrated {
		b.WriteString("  - " + l + "\n")
	}
	b.WriteString("In models (AutoMigrate) but NOT in migrations:\n")
	if len(onlyModels) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, l := range onlyModels {
		b.WriteString("  + " + l + "\n")
	}
	return b.String()
}
