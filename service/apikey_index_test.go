package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestAPIKeyHashIndexContract locks the same hash-index semantics on all
// three backends implementing APIKeyCredentialFinder: provider filtering,
// constant-time re-verification, rotation and deletion.
func TestAPIKeyHashIndexContract(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		runAPIKeyHashIndexContract(t, func(t *testing.T) Storage {
			return NewMemoryStorage()
		})
	})
	t.Run("sqlite", func(t *testing.T) {
		runAPIKeyHashIndexContract(t, func(t *testing.T) Storage {
			return newTestSQLStorage(t)
		})
	})
	t.Run("redis", func(t *testing.T) {
		runAPIKeyHashIndexContract(t, func(t *testing.T) Storage {
			s, mr := setupRedisStorage(t)
			t.Cleanup(func() { mr.Close() })
			return s
		})
	})
}

func runAPIKeyHashIndexContract(t *testing.T, mk func(t *testing.T) Storage) {
	t.Helper()
	ctx := context.Background()
	s := mk(t)
	finder, ok := s.(APIKeyCredentialFinder)
	if !ok {
		t.Fatalf("%T does not implement APIKeyCredentialFinder", s)
	}

	if err := s.SaveUser(ctx, &User{ID: "iu1", Name: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveUser(ctx, &User{ID: "iu2", Name: "Bob"}); err != nil {
		t.Fatal(err)
	}
	key1, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	key2, _ := GenerateAPIKey()
	key1new, _ := GenerateAPIKey()

	save := func(id, userID, provider string, key string) {
		t.Helper()
		enc := HashAPIKey(key)
		if provider != "api_key" {
			// Legacy-style non-hashed secret for other providers.
			enc = "enc-" + id
		}
		if err := s.SaveCredential(ctx, &Credential{ID: id, UserID: userID, Provider: provider, Encrypted: enc}); err != nil {
			t.Fatalf("save credential %s: %v", id, err)
		}
	}
	save("ic1", "iu1", "api_key", key1)
	save("ic2", "iu2", "api_key", key2)
	// Same hash as ic1 but a different provider: never indexed, never found.
	if err := s.SaveCredential(ctx, &Credential{ID: "ic3", UserID: "iu1", Provider: "openai", Encrypted: HashAPIKey(key1)}); err != nil {
		t.Fatal(err)
	}

	// Only api_key credentials are indexed.
	creds, err := finder.FindCredentialsByHash(ctx, HashAPIKey(key1))
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 1 || creds[0].ID != "ic1" {
		t.Fatalf("expected only ic1 for hash(key1), got %+v", creds)
	}

	// Authentication resolves through the index.
	u, err := FindUserByAPIKey(ctx, s, key1)
	if err != nil || u == nil || u.ID != "iu1" {
		t.Fatalf("key1: expected iu1, got %v (%v)", u, err)
	}
	u, err = FindUserByAPIKey(ctx, s, key2)
	if err != nil || u == nil || u.ID != "iu2" {
		t.Fatalf("key2: expected iu2, got %v (%v)", u, err)
	}
	unknown, _ := GenerateAPIKey()
	if _, err := FindUserByAPIKey(ctx, s, unknown); err == nil {
		t.Fatal("unknown key authenticated")
	}

	// Rotation: same credential ID, new key — old hash stops resolving and
	// the new one takes over.
	save("ic1", "iu1", "api_key", key1new)
	creds, err = finder.FindCredentialsByHash(ctx, HashAPIKey(key1))
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 0 {
		t.Fatalf("rotated-away hash still indexed: %+v", creds)
	}
	if _, err := FindUserByAPIKey(ctx, s, key1); err == nil {
		t.Fatal("old key still authenticates after rotation")
	}
	if u, err := FindUserByAPIKey(ctx, s, key1new); err != nil || u == nil || u.ID != "iu1" {
		t.Fatalf("new key: expected iu1, got %v (%v)", u, err)
	}

	// Deletion removes the index entry.
	if err := s.DeleteCredential(ctx, "ic2"); err != nil {
		t.Fatal(err)
	}
	creds, err = finder.FindCredentialsByHash(ctx, HashAPIKey(key2))
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 0 {
		t.Fatalf("deleted credential still indexed: %+v", creds)
	}
	if _, err := FindUserByAPIKey(ctx, s, key2); err == nil {
		t.Fatal("deleted key still authenticates")
	}
}

// scanOnlyStorage implements Storage (via embedding) but not
// APIKeyCredentialFinder, locking the linear-scan fallback path.
type scanOnlyStorage struct {
	Storage

	users map[string]*User
	creds map[string]*Credential
}

func (s *scanOnlyStorage) ListUsers(ctx context.Context) ([]*User, error) {
	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out, nil
}

func (s *scanOnlyStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error) {
	var out []*Credential
	for _, c := range s.creds {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *scanOnlyStorage) GetUser(ctx context.Context, id string) (*User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func TestFindUserByAPIKeyScanFallback(t *testing.T) {
	ctx := context.Background()
	key, _ := GenerateAPIKey()
	st := &scanOnlyStorage{
		users: map[string]*User{"su1": {ID: "su1", Name: "Alice"}},
		creds: map[string]*Credential{
			"sc1": {ID: "sc1", UserID: "su1", Provider: "api_key", Encrypted: HashAPIKey(key)},
		},
	}
	// scanOnlyStorage defines no FindCredentialsByHash method, so it cannot
	// satisfy APIKeyCredentialFinder and FindUserByAPIKey must take the
	// linear-scan path.
	u, err := FindUserByAPIKey(ctx, st, key)
	if err != nil || u == nil || u.ID != "su1" {
		t.Fatalf("scan fallback: expected su1, got %v (%v)", u, err)
	}
	other, _ := GenerateAPIKey()
	if _, err := FindUserByAPIKey(ctx, st, other); err == nil {
		t.Fatal("unknown key authenticated via scan")
	}
}

// TestSQLStorage_MigrationBackfillsAPIKeyHash upgrades a pre-index database
// (0001 schema, hashes only in the payload JSON) and locks that the backfill
// makes existing hashed api_key credentials resolve again — while legacy
// plaintext credentials stay un-indexed (fail closed).
func TestSQLStorage_MigrationBackfillsAPIKeyHash(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/legacy.db"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Pre-index credentials table (0001 shape — no key_hash column).
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS credentials (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		provider   TEXT,
		payload    TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`); err != nil {
		t.Fatal(err)
	}
	insert := func(id, userID, provider string, enc string) {
		t.Helper()
		row := credentialToPersist(&Credential{ID: id, UserID: userID, Provider: provider, Encrypted: enc})
		payload, _ := json.Marshal(row)
		_, err := db.Exec(`INSERT INTO credentials (id, user_id, provider, payload, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, id, userID, provider, string(payload), "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	key, _ := GenerateAPIKey()
	insert("lg1", "lu1", "api_key", HashAPIKey(key)) // hashed: must backfill
	insert("lg2", "lu1", "openai", "enc-openai")     // other provider: never indexed
	insert("lg3", "lu1", "api_key", "ask_plaintext") // legacy plaintext: fail closed
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening runs 0001 (idempotent) + 0002 (add column, backfill).
	s, err := NewSQLStorage(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen with migrations: %v", err)
	}
	defer s.Close()

	if err := s.SaveUser(ctx, &User{ID: "lu1", Name: "Alice"}); err != nil {
		t.Fatal(err)
	}
	creds, err := s.FindCredentialsByHash(ctx, HashAPIKey(key))
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 1 || creds[0].ID != "lg1" {
		t.Fatalf("backfill miss: got %+v", creds)
	}
	u, err := FindUserByAPIKey(ctx, s, key)
	if err != nil || u == nil || u.ID != "lu1" {
		t.Fatalf("pre-index key must authenticate after backfill: %v (%v)", u, err)
	}
	// Plaintext api_key rows stay un-indexed: the hash lookup finds nothing.
	creds, err = s.FindCredentialsByHash(ctx, "ask_plaintext")
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 0 {
		t.Fatalf("plaintext credential was indexed: %+v", creds)
	}
}

// TestSQLStorage_APIKeyHashQueryUsesIndex asserts the hash lookup actually
// uses idx_creds_key_hash (the whole point of the column).
func TestSQLStorage_APIKeyHashQueryUsesIndex(t *testing.T) {
	s := newTestSQLStorage(t)
	rows, err := s.DB().Query("EXPLAIN QUERY PLAN SELECT payload FROM credentials WHERE key_hash = 'sha256:x'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plans := []string{}
	for rows.Next() {
		var id, parent, notused, detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plans = append(plans, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plans, "\n")
	if !strings.Contains(joined, "USING INDEX idx_creds_key_hash") {
		t.Fatalf("hash lookup does not use the index; plan:\n%s", joined)
	}
}

// TestSQLStorage_ListSessionsBySchedule locks the source_schedule_id lookup:
// the field lives in the payload JSON (no dedicated column), so the query
// must extract it dialect-appropriately — the previous bare-column reference
// failed with "no such column" on every engine.
func TestSQLStorage_ListSessionsBySchedule(t *testing.T) {
	s := newTestSQLStorage(t)
	ctx := context.Background()
	seeds := []*Session{
		{ID: "ss1", UserID: "su1", SourceScheduleID: "sch1"},
		{ID: "ss2", UserID: "su1"},                           // no schedule
		{ID: "ss3", UserID: "su2", SourceScheduleID: "sch1"}, // other user
		{ID: "ss4", UserID: "su1", SourceScheduleID: "sch2"}, // other schedule
	}
	for _, se := range seeds {
		if err := s.SaveSession(ctx, se); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.ListSessionsBySchedule(ctx, "su1", "sch1")
	if err != nil {
		t.Fatalf("list sessions by schedule: %v", err)
	}
	if len(out) != 1 || out[0].ID != "ss1" {
		t.Fatalf("expected [ss1], got %+v", out)
	}
}
