package migration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate_AppliesAndIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	migrations := []Migration{
		{ID: "0001_base", Up: []string{
			`CREATE TABLE IF NOT EXISTS things (id TEXT PRIMARY KEY, payload TEXT NOT NULL);`,
			`CREATE INDEX IF NOT EXISTS idx_things_payload ON things(payload);`,
		}},
		{ID: "0002_extra", Up: []string{
			`ALTER TABLE things ADD COLUMN note TEXT;`,
		}},
	}
	if err := Migrate(ctx, db, DialectSQLite, migrations); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	ids, err := Applied(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "0001_base" || ids[1] != "0002_extra" {
		t.Fatalf("unexpected applied list: %v", ids)
	}

	// Re-running must be a no-op and must not duplicate records.
	if err := Migrate(ctx, db, DialectSQLite, migrations); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
	ids, _ = Applied(ctx, db)
	if len(ids) != 2 {
		t.Fatalf("idempotent rerun changed records: %v", ids)
	}

	// The migrated schema must actually be usable.
	if _, err := db.ExecContext(ctx, `INSERT INTO things (id, payload, note) VALUES ('a', 'p', 'n');`); err != nil {
		t.Fatalf("insert after migrate: %v", err)
	}
}

func TestMigrate_FailedStepRollsBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	migrations := []Migration{
		{ID: "0001_good", Up: []string{`CREATE TABLE IF NOT EXISTS keep (id TEXT PRIMARY KEY);`}},
		{ID: "0002_bad", Up: []string{
			`CREATE TABLE IF NOT EXISTS partial (id TEXT PRIMARY KEY);`,
			`THIS IS NOT SQL;`,
		}},
	}
	err := Migrate(ctx, db, DialectSQLite, migrations)
	if err == nil {
		t.Fatal("expected migration failure")
	}
	if !strings.Contains(err.Error(), "0002_bad") {
		t.Fatalf("error should name the failing migration: %v", err)
	}

	// The good step stays recorded; the bad step rolled back completely —
	// neither its table nor its schema_migrations record may exist.
	ids, _ := Applied(ctx, db)
	if len(ids) != 1 || ids[0] != "0001_good" {
		t.Fatalf("unexpected applied list after failure: %v", ids)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='partial';`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("failed migration's table should have been rolled back")
	}
}

func TestMigrate_RejectsDuplicateIDs(t *testing.T) {
	db := openTestDB(t)

	err := Migrate(context.Background(), db, DialectSQLite, []Migration{
		{ID: "0001_x", Up: []string{`SELECT 1;`}},
		{ID: "0001_x", Up: []string{`SELECT 2;`}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate ID rejection, got %v", err)
	}
}
