package controlplane

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/linkerlin/agentscope.go/internal/migration"
)

// pgTestDB mirrors the helper in service: $TEST_POSTGRES_DSN gates the suite.
func pgTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping Postgres migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open pgx: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Skipf("postgres unreachable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestPostgres_ControlPlaneMigrations verifies InitSchemaDialect applies the
// control-plane migrations (with the IDENTITY variant for cp_events/cp_spend)
// on Postgres, the auto-increment columns actually work, and re-running is
// idempotent.
func TestPostgres_ControlPlaneMigrations(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()

	if err := InitSchemaDialect(db, migration.DialectPostgres); err != nil {
		t.Fatalf("migrate controlplane on postgres: %v", err)
	}
	ids, err := migration.Applied(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	// The shared test database accumulates records from other suites' suites;
	// assert this suite's migration is recorded rather than an exact count.
	found := false
	for _, id := range ids {
		if id == cpMigrations[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q recorded, got %v", cpMigrations[0].ID, ids)
	}
	if err := InitSchemaDialect(db, migration.DialectPostgres); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}

	// The IDENTITY sequence columns must auto-assign on insert.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO cp_events (goal_id, kind, at) VALUES ($1, $2, $3);`,
		"g1", "test", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert cp_events: %v", err)
	}
	var seq int64
	if err := db.QueryRowContext(ctx, `SELECT seq FROM cp_events WHERE goal_id = $1;`, "g1").Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if seq <= 0 {
		t.Fatalf("expected identity seq to be assigned, got %d", seq)
	}

	// Every control-plane table must exist and be queryable.
	for _, table := range []string{"cp_goals", "cp_todos", "cp_gates", "cp_events", "cp_spend", "cp_tickets", "cp_rewards", "cp_deliveries"} {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+`;`).Scan(&n); err != nil {
			t.Fatalf("table %s unusable after migration: %v", table, err)
		}
	}
}
