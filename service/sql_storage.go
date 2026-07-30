package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLStorage implements Storage using a SQL database (SQLite by default).
// Each entity is stored in its own table with indexed lookup columns plus a
// JSON payload column for the full record. Aligned with Python agentscope's
// AsyncSQLAlchemyStorage (#b49a26b9).
type SQLStorage struct {
	db *sql.DB
}

// NewSQLStorage opens (or creates) a SQLite database at dbPath and provisions
// the schema. Use ":memory:" for an ephemeral in-process database (great for tests).
func NewSQLStorage(ctx context.Context, dbPath string) (*SQLStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlstorage: open %q: %w", dbPath, err)
	}
	// Enable WAL for better concurrency (ignored for :memory:).
	_, _ = db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)
	s := &SQLStorage{db: db}
	if err := s.initSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *SQLStorage) Close() error { return s.db.Close() }

// DB exposes the underlying *sql.DB for advanced use (migrations, inspection).
func (s *SQLStorage) DB() *sql.DB { return s.db }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT,
	name       TEXT,
	api_key    TEXT,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	agent_id   TEXT,
	team_id    TEXT,
	source     TEXT,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_team ON sessions(team_id);

CREATE TABLE IF NOT EXISTS agents (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	name       TEXT,
	source     TEXT,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);

CREATE TABLE IF NOT EXISTS credentials (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	provider   TEXT,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_creds_user ON credentials(user_id);

CREATE TABLE IF NOT EXISTS messages (
	id         TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	payload    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_msgs_session ON messages(session_id, created_at);

CREATE TABLE IF NOT EXISTS snapshots (
	session_id TEXT PRIMARY KEY,
	reply_id   TEXT,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schedules (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	enabled    INTEGER NOT NULL DEFAULT 1,
	payload    TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sched_user ON schedules(user_id);

CREATE TABLE IF NOT EXISTS teams (
	id                TEXT PRIMARY KEY,
	user_id           TEXT NOT NULL,
	leader_session_id TEXT,
	payload           TEXT NOT NULL,
	created_at        TEXT NOT NULL,
	updated_at        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_teams_user ON teams(user_id);
CREATE INDEX IF NOT EXISTS idx_teams_leader ON teams(leader_session_id);
`

func (s *SQLStorage) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

// --- helpers ---

func marshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// upsert inserts or replaces a row by the given conflict column (default "id").
func (s *SQLStorage) upsert(ctx context.Context, table string, cols []string, vals []any) error {
	return s.upsertConflict(ctx, table, "id", cols, vals)
}

// upsertConflict inserts or replaces a row by the given conflict column.
func (s *SQLStorage) upsertConflict(ctx context.Context, table, conflictCol string, cols []string, vals []any) error {
	if len(cols) != len(vals) {
		return fmt.Errorf("sqlstorage: cols/vals length mismatch")
	}
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "?"
	}
	colList := ""
	for i, c := range cols {
		if i > 0 {
			colList += ", "
		}
		colList += c
	}
	phList := ""
	for i, p := range placeholders {
		if i > 0 {
			phList += ", "
		}
		phList += p
	}
	// SQLite ON CONFLICT DO UPDATE
	updateCols := ""
	for _, c := range cols {
		if c == conflictCol {
			continue
		}
		if updateCols != "" {
			updateCols += ", "
		}
		updateCols += fmt.Sprintf("%s = excluded.%s", c, c)
	}
	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s",
		table, colList, phList, conflictCol, updateCols,
	)
	_, err := s.db.ExecContext(ctx, q, vals...)
	return err
}

// --- Users ---

func (s *SQLStorage) SaveUser(ctx context.Context, user *User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	user.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, "users",
		[]string{"id", "email", "name", "api_key", "payload", "created_at", "updated_at"},
		[]any{user.ID, user.Email, user.Name, user.APIKey, marshalJSON(user), nowUTC2(user.CreatedAt), nowUTC2(user.UpdatedAt)})
}

func nowUTC2(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func (s *SQLStorage) GetUser(ctx context.Context, id string) (*User, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM users WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var u User
	if err := json.Unmarshal([]byte(payload), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLStorage) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM users WHERE email = ?", email).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found by email: %s", email)
	}
	if err != nil {
		return nil, err
	}
	var u User
	if err := json.Unmarshal([]byte(payload), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLStorage) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM users ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*User](rows)
}

func (s *SQLStorage) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Cascade: delete user's sessions → messages → snapshots, agents, credentials, schedules, teams
	sessionIDs, _ := queryIDs(tx, ctx, "SELECT id FROM sessions WHERE user_id = ?", id)
	for _, sid := range sessionIDs {
		_, _ = tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sid)
		_, _ = tx.ExecContext(ctx, "DELETE FROM snapshots WHERE session_id = ?", sid)
	}
	_, _ = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", id)
	_, _ = tx.ExecContext(ctx, "DELETE FROM agents WHERE user_id = ?", id)
	_, _ = tx.ExecContext(ctx, "DELETE FROM credentials WHERE user_id = ?", id)
	_, _ = tx.ExecContext(ctx, "DELETE FROM schedules WHERE user_id = ?", id)
	_, _ = tx.ExecContext(ctx, "DELETE FROM teams WHERE user_id = ?", id)
	_, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// --- Sessions ---

func (s *SQLStorage) SaveSession(ctx context.Context, sess *Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	sess.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, "sessions",
		[]string{"id", "user_id", "agent_id", "team_id", "source", "payload", "created_at", "updated_at"},
		[]any{sess.ID, sess.UserID, sess.AgentID, sess.TeamID, sess.Source, marshalJSON(sess), nowUTC2(sess.CreatedAt), nowUTC2(sess.UpdatedAt)})
}

func (s *SQLStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM sessions WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal([]byte(payload), &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *SQLStorage) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM sessions WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Session](rows)
}

func (s *SQLStorage) DeleteSession(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", id)
	_, _ = tx.ExecContext(ctx, "DELETE FROM snapshots WHERE session_id = ?", id)
	_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// --- Agent configs ---

func (s *SQLStorage) SaveAgentConfig(ctx context.Context, cfg *AgentConfig) error {
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now().UTC()
	}
	cfg.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, "agents",
		[]string{"id", "user_id", "name", "source", "payload", "created_at", "updated_at"},
		[]any{cfg.ID, cfg.UserID, cfg.Name, cfg.Source, marshalJSON(cfg), nowUTC2(cfg.CreatedAt), nowUTC2(cfg.UpdatedAt)})
}

func (s *SQLStorage) GetAgentConfig(ctx context.Context, id string) (*AgentConfig, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM agents WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent config not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var cfg AgentConfig
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *SQLStorage) ListAgentConfigsByUser(ctx context.Context, userID string) ([]*AgentConfig, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM agents WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*AgentConfig](rows)
}

func (s *SQLStorage) DeleteAgentConfig(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM agents WHERE id = ?", id)
	return err
}

// --- Credentials ---

func (s *SQLStorage) SaveCredential(ctx context.Context, cred *Credential) error {
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = time.Now().UTC()
	}
	cred.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, "credentials",
		[]string{"id", "user_id", "provider", "payload", "created_at", "updated_at"},
		[]any{cred.ID, cred.UserID, cred.Provider, marshalJSON(cred), nowUTC2(cred.CreatedAt), nowUTC2(cred.UpdatedAt)})
}

func (s *SQLStorage) GetCredential(ctx context.Context, id string) (*Credential, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM credentials WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var cred Credential
	if err := json.Unmarshal([]byte(payload), &cred); err != nil {
		return nil, err
	}
	return &cred, nil
}

func (s *SQLStorage) ListCredentialsByUser(ctx context.Context, userID string) ([]*Credential, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM credentials WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Credential](rows)
}

func (s *SQLStorage) DeleteCredential(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM credentials WHERE id = ?", id)
	return err
}

// --- Messages ---

func (s *SQLStorage) SaveMessage(ctx context.Context, msg *StoredMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	return s.upsert(ctx, "messages",
		[]string{"id", "session_id", "created_at", "payload"},
		[]any{msg.ID, msg.SessionID, nowUTC2(msg.CreatedAt), marshalJSON(msg)})
}

func (s *SQLStorage) UpsertMessage(ctx context.Context, msg *StoredMessage) error {
	return s.SaveMessage(ctx, msg)
}

func (s *SQLStorage) GetMessage(ctx context.Context, id string) (*StoredMessage, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM messages WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var msg StoredMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *SQLStorage) ListMessagesBySession(ctx context.Context, sessionID string, limit, offset int) ([]*StoredMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT payload FROM messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?",
		sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*StoredMessage](rows)
}

func (s *SQLStorage) DeleteMessagesBySession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sessionID)
	return err
}

// --- Snapshots ---

func (s *SQLStorage) SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error {
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}
	return s.upsertConflict(ctx, "snapshots", "session_id",
		[]string{"session_id", "reply_id", "payload", "created_at"},
		[]any{snap.SessionID, snap.ReplyID, marshalJSON(snap), nowUTC2(snap.CreatedAt)})
}

func (s *SQLStorage) GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM snapshots WHERE session_id = ?", sessionID).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot not found: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}
	var snap AgentSnapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *SQLStorage) DeleteSnapshot(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM snapshots WHERE session_id = ?", sessionID)
	return err
}

// --- Schedules ---

func (s *SQLStorage) SaveSchedule(ctx context.Context, sched *Schedule) error {
	if sched.CreatedAt.IsZero() {
		sched.CreatedAt = time.Now().UTC()
	}
	sched.UpdatedAt = time.Now().UTC()
	enabled := 0
	if sched.Enabled {
		enabled = 1
	}
	return s.upsert(ctx, "schedules",
		[]string{"id", "user_id", "enabled", "payload", "created_at", "updated_at"},
		[]any{sched.ID, sched.UserID, enabled, marshalJSON(sched), nowUTC2(sched.CreatedAt), nowUTC2(sched.UpdatedAt)})
}

func (s *SQLStorage) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM schedules WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var sched Schedule
	if err := json.Unmarshal([]byte(payload), &sched); err != nil {
		return nil, err
	}
	return &sched, nil
}

func (s *SQLStorage) ListSchedulesByUser(ctx context.Context, userID string) ([]*Schedule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM schedules WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Schedule](rows)
}

func (s *SQLStorage) ListAllSchedules(ctx context.Context) ([]*Schedule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM schedules WHERE enabled = 1 ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Schedule](rows)
}

func (s *SQLStorage) DeleteSchedule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM schedules WHERE id = ?", id)
	return err
}

func (s *SQLStorage) ListSessionsBySchedule(ctx context.Context, userID, scheduleID string) ([]*Session, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT payload FROM sessions WHERE user_id = ? AND source_schedule_id = ? ORDER BY created_at DESC",
		userID, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Session](rows)
}

// --- Teams ---

func (s *SQLStorage) SaveTeam(ctx context.Context, team *Team) error {
	if team.CreatedAt.IsZero() {
		team.CreatedAt = time.Now().UTC()
	}
	team.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, "teams",
		[]string{"id", "user_id", "leader_session_id", "payload", "created_at", "updated_at"},
		[]any{team.ID, team.UserID, team.LeaderSessionID, marshalJSON(team), nowUTC2(team.CreatedAt), nowUTC2(team.UpdatedAt)})
}

func (s *SQLStorage) GetTeam(ctx context.Context, id string) (*Team, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM teams WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("team not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var team Team
	if err := json.Unmarshal([]byte(payload), &team); err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *SQLStorage) ListTeamsByUser(ctx context.Context, userID string) ([]*Team, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM teams WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows[*Team](rows)
}

func (s *SQLStorage) DeleteTeam(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM teams WHERE id = ?", id)
	return err
}

func (s *SQLStorage) GetTeamByLeaderSession(ctx context.Context, sessionID string) (*Team, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM teams WHERE leader_session_id = ?", sessionID).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("team not found by leader session: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}
	var team Team
	if err := json.Unmarshal([]byte(payload), &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// --- generic helpers ---

func scanRows[T any](rows *sql.Rows) ([]T, error) {
	var out []T
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func queryIDs(tx *sql.Tx, ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
