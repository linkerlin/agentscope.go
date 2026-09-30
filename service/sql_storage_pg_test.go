package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

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

// TestPostgres_ConcurrentMigrations locks the 23.4 rule: concurrent replica
// startups serialise on the advisory lock and every migration is recorded
// exactly once — no replica errors out.
func TestPostgres_ConcurrentMigrations(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()

	// A dedicated migration list so the "exactly once" assertion holds even
	// on the shared test database.
	conc := []migration.Migration{{
		ID:   fmt.Sprintf("0001_conc_%d", time.Now().UnixNano()),
		Up:   []string{`SELECT 1;`},
		UpPG: []string{`SELECT 1;`},
	}}

	const replicas = 8
	errs := make(chan error, replicas)
	for i := 0; i < replicas; i++ {
		go func() { errs <- migration.Migrate(ctx, db, migration.DialectPostgres, conc) }()
	}
	for i := 0; i < replicas; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent postgres migrate: %v", err)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE id = $1;`, conc[0].ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration recorded %d times, want exactly 1", count)
	}
}

// TestPostgres_SQLStorageCRUDContract runs the real CRUD + transaction paths
// of SQLStorage against Postgres (23.4): the support claim now matches the
// tests — same semantics as the SQLite suite, including the upsert
// (ON CONFLICT) path and the cascade-delete transaction.
func TestPostgres_SQLStorageCRUDContract(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()

	if err := migration.Migrate(ctx, db, migration.DialectPostgres, migrations); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := &SQLStorage{db: db, dialect: migration.DialectPostgres}

	// User CRUD.
	if err := s.SaveUser(ctx, &User{ID: "pg-u1", Name: "Alice"}); err != nil {
		t.Fatalf("save user: %v", err)
	}
	got, err := s.GetUser(ctx, "pg-u1")
	if err != nil || got.Name != "Alice" {
		t.Fatalf("get user: %v %+v", err, got)
	}

	// Credential round trip (secret must survive persistence, 22.1).
	if err := s.SaveCredential(ctx, &Credential{ID: "pg-c1", UserID: "pg-u1", Provider: "openai", Encrypted: "enc-pg"}); err != nil {
		t.Fatalf("save credential: %v", err)
	}
	cred, err := s.GetCredential(ctx, "pg-c1")
	if err != nil || cred.Encrypted != "enc-pg" {
		t.Fatalf("credential round trip: %v %+v", err, cred)
	}

	// Upsert (ON CONFLICT) overwrites.
	if err := s.SaveUser(ctx, &User{ID: "pg-u1", Name: "Alice2"}); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	got, _ = s.GetUser(ctx, "pg-u1")
	if got.Name != "Alice2" {
		t.Fatalf("upsert did not overwrite: %+v", got)
	}

	// Session + message + cascade transaction.
	if err := s.SaveSession(ctx, &Session{ID: "pg-s1", UserID: "pg-u1", Title: "t"}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := s.UpsertMessage(ctx, &StoredMessage{ID: "pg-m1", SessionID: "pg-s1", Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("save message: %v", err)
	}
	if err := s.DeleteUser(ctx, "pg-u1"); err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	if _, err := s.GetSession(ctx, "pg-s1"); err == nil {
		t.Fatal("session survived cascade delete")
	}
	if _, err := s.GetCredential(ctx, "pg-c1"); err == nil {
		t.Fatal("credential survived cascade delete")
	}
	if _, err := s.GetMessage(ctx, "pg-m1"); err == nil {
		t.Fatal("message survived cascade delete")
	}
}

// TestPostgres_SQLStorageAPIKeyIndexAndSchedule locks the hash-index and
// schedule-session query paths on Postgres: the key_hash upsert/backfill
// column, rotation semantics, and the jsonb source_schedule_id extraction
// (no dedicated column — same expression family as the SQLite json_extract
// path).
func TestPostgres_SQLStorageAPIKeyIndexAndSchedule(t *testing.T) {
	db := pgTestDB(t)
	ctx := context.Background()

	if err := migration.Migrate(ctx, db, migration.DialectPostgres, migrations); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := &SQLStorage{db: db, dialect: migration.DialectPostgres}

	if err := s.SaveUser(ctx, &User{ID: "pg-iu1", Name: "Alice"}); err != nil {
		t.Fatalf("save user: %v", err)
	}
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	keyNew, _ := GenerateAPIKey()
	if err := s.SaveCredential(ctx, &Credential{ID: "pg-ic1", UserID: "pg-iu1", Provider: "api_key", Encrypted: HashAPIKey(key)}); err != nil {
		t.Fatalf("save credential: %v", err)
	}

	creds, err := s.FindCredentialsByHash(ctx, HashAPIKey(key))
	if err != nil || len(creds) != 1 || creds[0].ID != "pg-ic1" {
		t.Fatalf("find by hash: %v %+v", err, creds)
	}
	// Rotation clears the old hash entry.
	if err := s.SaveCredential(ctx, &Credential{ID: "pg-ic1", UserID: "pg-iu1", Provider: "api_key", Encrypted: HashAPIKey(keyNew)}); err != nil {
		t.Fatalf("rotate credential: %v", err)
	}
	creds, err = s.FindCredentialsByHash(ctx, HashAPIKey(key))
	if err != nil || len(creds) != 0 {
		t.Fatalf("old hash still indexed after rotation: %v %+v", err, creds)
	}
	creds, err = s.FindCredentialsByHash(ctx, HashAPIKey(keyNew))
	if err != nil || len(creds) != 1 || creds[0].ID != "pg-ic1" {
		t.Fatalf("new hash lookup: %v %+v", err, creds)
	}

	// Schedule sessions via the payload jsonb extraction.
	sessions := []*Session{
		{ID: "pg-ss1", UserID: "pg-iu1", SourceScheduleID: "pg-sch1"},
		{ID: "pg-ss2", UserID: "pg-iu1"},
		{ID: "pg-ss3", UserID: "pg-iu1", SourceScheduleID: "pg-sch2"},
	}
	for _, se := range sessions {
		if err := s.SaveSession(ctx, se); err != nil {
			t.Fatalf("save session %s: %v", se.ID, err)
		}
	}
	out, err := s.ListSessionsBySchedule(ctx, "pg-iu1", "pg-sch1")
	if err != nil {
		t.Fatalf("list sessions by schedule: %v", err)
	}
	if len(out) != 1 || out[0].ID != "pg-ss1" {
		t.Fatalf("expected [pg-ss1], got %+v", out)
	}

	// Cleanup (shared test database).
	_, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE id LIKE 'pg-ss%';`)
	_, _ = db.ExecContext(ctx, `DELETE FROM credentials WHERE id = 'pg-ic1';`)
	_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = 'pg-iu1';`)
}
