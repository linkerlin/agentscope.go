package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/linkerlin/agentscope.go/internal/migration"
)

// pgTestDB opens the Postgres test database from $TEST_POSTGRES_DSN and skips
// when unset — mirroring the $REDIS_URL convention of messagebus. Run a
// disposable server locally, e.g.:
//
//	docker run --rm -d -p 15432:5432 -e POSTGRES_PASSWORD=test \
//	  -e POSTGRES_DB=agentscope_mig postgres:16-alpine
//	TEST_POSTGRES_DSN='postgres://postgres:test@localhost:15432/agentscope_mig?sslmode=disable' \
//	  go test ./service/ ./controlplane/ -run Postgres -v
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

// TestPostgres_ServiceMigrations verifies the service schema migrations apply
// on Postgres and leave a working, versioned schema behind.
func TestPostgres_ServiceMigrations(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()

	// Fresh database: schema_migrations must not exist yet.
	if err := migration.Migrate(ctx, db, migration.DialectPostgres, migrations); err != nil {
		t.Fatalf("migrate on postgres: %v", err)
	}
	ids, err := migration.Applied(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	// The shared test database accumulates records from other suites too;
	// assert this suite's migration is recorded rather than an exact count.
	found := false
	for _, id := range ids {
		if id == migrations[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q recorded, got %v", migrations[0].ID, ids)
	}
	// Idempotent re-run.
	if err := migration.Migrate(ctx, db, migration.DialectPostgres, migrations); err != nil {
		t.Fatalf("idempotent migrate on postgres: %v", err)
	}

	// Verify every migrated table is present and queryable. SQLStorage's
	// query paths target SQLite placeholders; this test exercises the
	// Postgres schema the migrations produced, not those query paths.
	for _, table := range []string{"users", "sessions", "agents", "credentials", "messages", "snapshots", "schedules", "teams"} {
		var n int
		if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s;`, table)).Scan(&n); err != nil {
			t.Fatalf("table %s unusable after migration: %v", table, err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users;`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected fresh users table, got %d rows", n)
	}
}
