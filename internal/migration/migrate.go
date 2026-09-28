// Package migration provides the minimal versioned schema migration used by
// both service (SQLStorage) and controlplane (InitSchema). 16.5: schema
// changes go through ordered migrations recorded in a schema_migrations table
// instead of ad-hoc CREATE TABLE IF NOT EXISTS statements.
//
// The migrator is dialect-agnostic on purpose: statements must be valid on
// SQLite and Postgres unless a migration carries an explicit Postgres variant
// (UpPG), and each migration runs inside one transaction (both engines support
// DDL transactions; statements are additive DDL, no PRAGMA).
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Migration is one ordered schema step. Up holds the SQL statements executed
// in order within a single transaction, together with the ID record. UpPG, when
// non-nil, replaces Up on Postgres (e.g. AUTOINCREMENT → GENERATED AS
// IDENTITY); everything else must be written to be valid on both dialects.
type Migration struct {
	// ID is a stable, order-sensitive version string, e.g. "0001_initial".
	ID string
	Up []string
	// UpPG is the optional Postgres variant of Up. nil means Up is
	// Postgres-compatible as-is.
	UpPG []string
}

// Dialect names passed to Migrate. database/sql exposes no reliable driver
// name API, so callers state the dialect they opened the connection with.
const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
)

// Migrate applies every migration not yet recorded in schema_migrations, in
// the given order, using the given dialect ("sqlite" or "postgres") to pick
// per-dialect statements. It is idempotent: already-applied IDs are skipped,
// so calling it on every startup is the intended usage. Duplicate IDs within
// the list are rejected. Any other dialect is rejected outright (23.4):
// silently running SQLite statements against an unsupported engine would
// corrupt the schema halfway.
//
// Concurrent startups (23.4): two replicas migrating at once converge
// without error — Postgres serialises the check-and-apply window with a
// session advisory lock, and on both engines a migration whose schema_migrations
// insert loses a race is treated as applied (the winner did the work).
func Migrate(ctx context.Context, db *sql.DB, dialect string, migrations []Migration) error {
	if dialect != DialectSQLite && dialect != DialectPostgres {
		return fmt.Errorf("migration: unsupported dialect %q (supported: %q, %q)",
			dialect, DialectSQLite, DialectPostgres)
	}

	if dialect == DialectPostgres {
		unlock, err := lockPostgres(ctx, db)
		if err != nil {
			return fmt.Errorf("migration: advisory lock: %w", err)
		}
		defer unlock()
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		id         TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	);`); err != nil {
		return fmt.Errorf("migration: create schema_migrations: %w", err)
	}

	seen := make(map[string]bool, len(migrations))
	for _, m := range migrations {
		if m.ID == "" {
			return fmt.Errorf("migration: empty migration ID")
		}
		if seen[m.ID] {
			return fmt.Errorf("migration: duplicate ID %q", m.ID)
		}
		seen[m.ID] = true
	}

	applied := make(map[string]bool)
	rows, err := db.QueryContext(ctx, `SELECT id FROM schema_migrations;`)
	if err != nil {
		return fmt.Errorf("migration: load applied: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("migration: scan applied: %w", err)
		}
		applied[id] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("migration: iterate applied: %w", err)
	}
	rows.Close()

	for _, m := range migrations {
		if applied[m.ID] {
			continue
		}
		if err := applyOne(ctx, db, dialect, m); err != nil {
			if errors.Is(err, errMigrationRaced) {
				// Another replica applied this exact migration between our
				// load and our insert: re-check and skip when it is there.
				now, err := Applied(ctx, db)
				if err != nil {
					return err
				}
				found := false
				for _, id := range now {
					if id == m.ID {
						found = true
						break
					}
				}
				if found {
					continue
				}
			}
			return err
		}
	}
	return nil
}

// errMigrationRaced reports that the schema_migrations insert hit a unique
// conflict: a concurrent replica applied the same migration first.
var errMigrationRaced = errors.New("migration: applied concurrently by another replica")

// migrationLockKey is the advisory-lock key for the migration critical
// section (arbitrary constant, stable across replicas).
const migrationLockKey = 0x6167_6D69_6772 // "agmigr"

// lockPostgres takes a session-level advisory lock so concurrent replicas
// serialise their check-and-apply windows. The lock is held on a dedicated
// connection (pool connections change between calls) until the returned
// unlock runs.
func lockPostgres(ctx context.Context, db *sql.DB) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1);`, migrationLockKey); err != nil {
		conn.Close()
		return nil, err
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1);`, migrationLockKey)
		_ = conn.Close()
	}, nil
}

// statementsFor picks the dialect-appropriate statement list for a migration.
func statementsFor(m Migration, dialect string) []string {
	if dialect == DialectPostgres && m.UpPG != nil {
		return m.UpPG
	}
	return m.Up
}

// applyOne runs a single migration inside one transaction: statements first,
// then the schema_migrations record; any failure rolls the whole step back so
// a partially applied migration can never be recorded.
func applyOne(ctx context.Context, db *sql.DB, dialect string, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %s: begin tx: %w", m.ID, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	for i, stmt := range statementsFor(m, dialect) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %s: statement %d: %w", m.ID, i, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (id, applied_at) VALUES ($1, $2);`,
		m.ID, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		// A concurrent replica applied the same migration between our applied
		// load and this insert: the unique key on id fires. Report the race
		// so the caller can re-check and converge (23.4).
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s", errMigrationRaced, m.ID)
		}
		return fmt.Errorf("migration %s: record: %w", m.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %s: commit: %w", m.ID, err)
	}
	return nil
}

// Applied lists the recorded migration IDs in order of application. Used by
// tests and by callers that want to verify the schema version.
func Applied(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM schema_migrations ORDER BY applied_at, id;`)
	if err != nil {
		return nil, fmt.Errorf("migration: query applied: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("migration: scan applied: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// isUniqueViolation detects the primary-key conflict on schema_migrations
// across the two supported drivers (pgx error code 23505, SQLite "UNIQUE
// constraint failed").
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "UNIQUE constraint failed")
}
