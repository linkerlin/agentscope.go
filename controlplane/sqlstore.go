package controlplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// txCtxKey carries a *sql.Tx through context so SQL store methods can enlist in
// a caller-started transaction (#4 round-3). Memory stores ignore it.
type txCtxKey struct{}

// WithTx returns ctx with tx attached. SQL store write methods enlisted in this
// tx use it instead of the bare *sql.DB; commit/rollback stays with the caller.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// txFromContext returns the tx attached to ctx, if any.
func txFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(*sql.Tx)
	return tx, ok
}

// execer is the common ExecContext surface of *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// queryer is the common query surface of *sql.DB and *sql.Tx.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SQL durability for the control plane. All five stores (Goal/Todo/Gate/Ledger/
// SpendLog) are backed by one *sql.DB. controlplane imports ONLY database/sql
// (stdlib); the driver (e.g. modernc.org/sqlite) is registered by the caller,
// so this package stays dependency-free at the source level. SQLite is the
// default target (zero-CGO via modernc); Postgres/MySQL work with minor DDL.
//
// This closes the biggest real-world gap from the design review: in-memory
// stores lost all goals on restart, defeating the "lifetime goal" invariant.
// Use NewSQLKernel(db) to wire a Kernel whose state survives process restarts.

// InitSchema creates the control-plane tables on db if they do not exist. Safe
// to call multiple times. Dialect is SQLite; the column types are generic
// enough for Postgres/MySQL with minor tweaks.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cp_goals (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL DEFAULT '',
			capability_id TEXT NOT NULL DEFAULT '',
			objective TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT '[]',
			authority TEXT NOT NULL DEFAULT '[]',
			state TEXT NOT NULL DEFAULT 'active',
			current_todo_id TEXT NOT NULL DEFAULT '',
			quota TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_goals_owner ON cp_goals(owner_user_id)`,
		`CREATE TABLE IF NOT EXISTS cp_todos (
			id TEXT NOT NULL,
			goal_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			task_class TEXT NOT NULL DEFAULT 'advancement_task',
			stage_id TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'open',
			claimed_by TEXT NOT NULL DEFAULT '',
			continuation TEXT NOT NULL DEFAULT '',
			"order" INTEGER NOT NULL DEFAULT 0,
			evidence_ids TEXT NOT NULL DEFAULT '[]',
			evidence TEXT NOT NULL DEFAULT '[]',
			supersedes TEXT NOT NULL DEFAULT '',
			superseded_by TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (goal_id, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_todos_owner ON cp_todos(owner_user_id)`,
		`CREATE TABLE IF NOT EXISTS cp_gates (
			gate_id TEXT NOT NULL,
			goal_id TEXT NOT NULL,
			todo_id TEXT NOT NULL DEFAULT '',
			question TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT '{}',
			fallback TEXT NOT NULL DEFAULT '',
			outcome TEXT NOT NULL DEFAULT '',
			resolvers TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			resolved_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (goal_id, gate_id)
		)`,
		`CREATE TABLE IF NOT EXISTS cp_events (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			todo_id TEXT NOT NULL DEFAULT '',
			turn_id TEXT NOT NULL DEFAULT '',
			gate_id TEXT NOT NULL DEFAULT '',
			outcome TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '{}',
			at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_events_goal ON cp_events(goal_id, seq)`,
		`CREATE TABLE IF NOT EXISTS cp_spend (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id TEXT NOT NULL,
			turn_id TEXT NOT NULL DEFAULT '',
			slots INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			spent_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_spend_goal ON cp_spend(goal_id, spent_at)`,
		`CREATE TABLE IF NOT EXISTS cp_tickets (
			goal_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			token TEXT NOT NULL,
			consumed INTEGER NOT NULL DEFAULT 0,
			minted_at TEXT NOT NULL,
			PRIMARY KEY (goal_id, turn_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_tickets_reap ON cp_tickets(consumed, minted_at)`,
		`CREATE TABLE IF NOT EXISTS cp_rewards (
			goal_id TEXT NOT NULL,
			class TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT '{}',
			authority TEXT NOT NULL DEFAULT '',
			confidence TEXT NOT NULL DEFAULT 'low',
			lifecycle TEXT NOT NULL DEFAULT 'active',
			content TEXT NOT NULL DEFAULT '',
			at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_rewards_goal ON cp_rewards(goal_id)`,
		`CREATE TABLE IF NOT EXISTS cp_deliveries (
			goal_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			todo_id TEXT NOT NULL DEFAULT '',
			outcome TEXT NOT NULL DEFAULT '',
			slots INTEGER NOT NULL DEFAULT 1,
			spent INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			PRIMARY KEY (goal_id, turn_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cp_deliveries_reap ON cp_deliveries(spent, created_at)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("controlplane: init schema: %w", err)
		}
	}
	return nil
}

// SQLStores bundles the five SQL-backed stores sharing one db. Build via
// NewSQLStores (which also runs InitSchema) and pass into NewKernel.
type SQLStores struct {
	Goals GoalStore
	Todos TodoStore
	Gates GateStore
	Ledger
	SpendLog
	Rewards    RewardStore
	Deliveries DeliveryStore
	db         *sql.DB
}

// NewSQLStores opens the control-plane schema on db and returns the five stores.
// The caller is responsible for driver registration (e.g. modernc.org/sqlite)
// and db lifecycle.
func NewSQLStores(db *sql.DB) (*SQLStores, error) {
	if err := InitSchema(db); err != nil {
		return nil, err
	}
	return &SQLStores{
		Goals:      NewSQLGoalStore(db),
		Todos:      NewSQLTodoStore(db),
		Gates:      NewSQLGateStore(db),
		Ledger:     NewSQLLedger(db),
		SpendLog:   NewSQLSpendLog(db),
		Rewards:    NewSQLRewardStore(db),
		Deliveries: NewSQLDeliveryStore(db),
		db:         db,
	}, nil
}

// RunInTx runs fn inside a single DB transaction with a tx-attached context.
// SQL store write methods enlisted via that context join the tx (#4 round-3);
// if fn returns an error the tx is rolled back and all writes are discarded.
// This makes multi-step Kernel mutations (e.g. SupersedeTodo) atomic.
func (s *SQLStores) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.db == nil {
		return fn(ctx)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	tctx := WithTx(ctx, tx)
	if err := fn(tctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// NewSQLKernel builds a Kernel whose Goal/Todo/Gate/Ledger/Spend/Ticket state
// is durable AND shared across processes. The capability registry remains
// in-memory (it is a static catalog, not runtime state). The TicketStore is
// SQL-backed so ticket enforcement (#3) composes with multi-process (#1).
func NewSQLKernel(db *sql.DB) (*Kernel, error) {
	stores, err := NewSQLStores(db)
	if err != nil {
		return nil, err
	}
	return NewKernel(stores.Goals, stores.Todos, stores.SpendLog).
		WithGateStore(stores.Gates).
		WithLedger(stores.Ledger).
		WithTicketStore(NewSQLTicketStore(db)).
		WithRewardStore(stores.Rewards).
		WithDeliveryStore(stores.Deliveries).
		WithTxStarter(stores.RunInTx), nil
}

// --- helpers ---

func encJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func decJSONInto(s string, v any) {
	if s == "" {
		return
	}
	_ = json.Unmarshal([]byte(s), v)
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func parseTS(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// --- SQLGoalStore ---

// SQLGoalStore is a GoalStore backed by *sql.DB. OwnerUserID scopes goals per
// tenant; an empty owner matches legacy/global goals (see #5 tenancy fix).
type SQLGoalStore struct {
	db *sql.DB
}

// NewSQLGoalStore returns a GoalStore over db. Assumes InitSchema has run.
func NewSQLGoalStore(db *sql.DB) *SQLGoalStore { return &SQLGoalStore{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLGoalStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) so reads inside a tx use the tx's
// connection and don't block on SQLite's db-level write lock (#4).
func (s *SQLGoalStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Get returns the goal, or ErrGoalNotFound.
func (s *SQLGoalStore) Get(ctx context.Context, id string) (*Goal, error) {
	row := s.qr(ctx).QueryRowContext(ctx,
		`SELECT id, owner_user_id, capability_id, objective, scope, authority, state, current_todo_id, quota, created_at, updated_at
		 FROM cp_goals WHERE id = ?`, id)
	g, err := scanGoal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalNotFound
	}
	return g, err
}

// Upsert inserts or replaces the goal by id.
func (s *SQLGoalStore) Upsert(ctx context.Context, g *Goal) error {
	if g == nil || g.ID == "" {
		return errors.New("controlplane: goal id required")
	}
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	_, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_goals (id, owner_user_id, capability_id, objective, scope, authority, state, current_todo_id, quota, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   owner_user_id=excluded.owner_user_id, capability_id=excluded.capability_id, objective=excluded.objective, scope=excluded.scope,
		   authority=excluded.authority, state=excluded.state, current_todo_id=excluded.current_todo_id,
		   quota=excluded.quota, updated_at=excluded.updated_at`,
		g.ID, g.OwnerUserID, g.CapabilityID, g.Objective, encJSON(g.Scope), encJSON(g.Authority),
		string(g.State), g.CurrentTodoID, encJSON(g.Quota), ts(g.CreatedAt), ts(g.UpdatedAt))
	return err
}

// Delete removes the goal. A missing id is not an error.
func (s *SQLGoalStore) Delete(ctx context.Context, id string) error {
	_, err := s.ex(ctx).ExecContext(ctx, `DELETE FROM cp_goals WHERE id = ?`, id)
	return err
}

// List returns all goals.
func (s *SQLGoalStore) List(ctx context.Context) ([]*Goal, error) {
	rows, err := s.qr(ctx).QueryContext(ctx,
		`SELECT id, owner_user_id, capability_id, objective, scope, authority, state, current_todo_id, quota, created_at, updated_at
		 FROM cp_goals ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Goal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGoal(sc scanner) (*Goal, error) {
	var g Goal
	var scope, authority, quota, state, created, updated string
	if err := sc.Scan(&g.ID, &g.OwnerUserID, &g.CapabilityID, &g.Objective, &scope, &authority,
		&state, &g.CurrentTodoID, &quota, &created, &updated); err != nil {
		return nil, err
	}
	g.State = GoalState(state)
	decJSONInto(scope, &g.Scope)
	decJSONInto(authority, &g.Authority)
	decJSONInto(quota, &g.Quota)
	g.CreatedAt = parseTS(created)
	g.UpdatedAt = parseTS(updated)
	return &g, nil
}

// --- SQLTodoStore ---

// SQLTodoStore is a TodoStore backed by *sql.DB.
type SQLTodoStore struct {
	db *sql.DB
}

// NewSQLTodoStore returns a TodoStore over db.
func NewSQLTodoStore(db *sql.DB) *SQLTodoStore { return &SQLTodoStore{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLTodoStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads (#4).
func (s *SQLTodoStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Get returns the todo, or ErrTodoNotFound.
func (s *SQLTodoStore) Get(ctx context.Context, goalID, todoID string) (*Todo, error) {
	row := s.qr(ctx).QueryRowContext(ctx,
		`SELECT id, goal_id, owner_user_id, description, task_class, stage_id, state, claimed_by, continuation,
		        "order", evidence_ids, evidence, supersedes, superseded_by, created_at, updated_at
		 FROM cp_todos WHERE goal_id = ? AND id = ?`, goalID, todoID)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTodoNotFound
	}
	return t, err
}

// List returns all todos for a goal.
func (s *SQLTodoStore) List(ctx context.Context, goalID string) ([]*Todo, error) {
	rows, err := s.qr(ctx).QueryContext(ctx,
		`SELECT id, goal_id, owner_user_id, description, task_class, stage_id, state, claimed_by, continuation,
		        "order", evidence_ids, evidence, supersedes, superseded_by, created_at, updated_at
		 FROM cp_todos WHERE goal_id = ? ORDER BY "order", id`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Upsert inserts or replaces the todo by (goal_id, id).
func (s *SQLTodoStore) Upsert(ctx context.Context, t *Todo) error {
	if t == nil || t.ID == "" || t.GoalID == "" {
		return errors.New("controlplane: todo id and goal_id required")
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_todos (id, goal_id, owner_user_id, description, task_class, stage_id, state, claimed_by, continuation, "order", evidence_ids, evidence, supersedes, superseded_by, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(goal_id, id) DO UPDATE SET
		   owner_user_id=excluded.owner_user_id, description=excluded.description, task_class=excluded.task_class,
		   stage_id=excluded.stage_id,
		   state=excluded.state, claimed_by=excluded.claimed_by, continuation=excluded.continuation, "order"=excluded."order",
		   evidence_ids=excluded.evidence_ids, evidence=excluded.evidence, supersedes=excluded.supersedes, superseded_by=excluded.superseded_by, updated_at=excluded.updated_at`,
		t.ID, t.GoalID, t.OwnerUserID, t.Description, string(t.TaskClass), t.StageID, string(t.State), t.ClaimedBy,
		string(t.Continuation), t.Order, encJSON(t.EvidenceIDs()), encJSON(t.Evidence), t.Supersedes, t.SupersededBy, ts(t.CreatedAt), ts(t.UpdatedAt))
	return err
}

// Delete removes a todo.
func (s *SQLTodoStore) Delete(ctx context.Context, goalID, todoID string) error {
	_, err := s.ex(ctx).ExecContext(ctx, `DELETE FROM cp_todos WHERE goal_id = ? AND id = ?`, goalID, todoID)
	return err
}

func scanTodo(sc scanner) (*Todo, error) {
	var t Todo
	var taskClass, state, cont, evIDs, evidence, created, updated string
	if err := sc.Scan(&t.ID, &t.GoalID, &t.OwnerUserID, &t.Description, &taskClass, &t.StageID, &state, &t.ClaimedBy,
		&cont, &t.Order, &evIDs, &evidence, &t.Supersedes, &t.SupersededBy, &created, &updated); err != nil {
		return nil, err
	}
	t.TaskClass = TaskClass(taskClass)
	t.State = TodoState(state)
	t.Continuation = ContinuationPolicy(cont)
	// Evidence is the single source of truth; the stored evidence_ids column is
	// legacy/derived and not re-populated (#5 round-5).
	decJSONInto(evidence, &t.Evidence)
	t.CreatedAt = parseTS(created)
	t.UpdatedAt = parseTS(updated)
	return &t, nil
}

// --- SQLGateStore ---

// SQLGateStore is a GateStore backed by *sql.DB.
type SQLGateStore struct {
	db *sql.DB
}

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLGateStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads.
func (s *SQLGateStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// NewSQLGateStore returns a GateStore over db.
func NewSQLGateStore(db *sql.DB) *SQLGateStore { return &SQLGateStore{db: db} }

// Upsert inserts or replaces a gate (with fallback-scope validation).
func (s *SQLGateStore) Upsert(ctx context.Context, g UserGate) error {
	if g.GateID == "" || g.GoalID == "" {
		return errors.New("controlplane: gate_id and goal_id required")
	}
	if g.Fallback != nil && !g.Scope.Covers(g.Fallback.Scope) {
		return ErrFallbackScopeMismatch
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	var outcomeJSON string
	if g.Outcome != nil {
		outcomeJSON = encJSON(g.Outcome)
	}
	_, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_gates (gate_id, goal_id, todo_id, question, scope, fallback, outcome, resolvers, created_at, resolved_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(goal_id, gate_id) DO UPDATE SET
		   todo_id=excluded.todo_id, question=excluded.question, scope=excluded.scope, fallback=excluded.fallback,
		   outcome=excluded.outcome, resolvers=excluded.resolvers, resolved_at=excluded.resolved_at`,
		g.GateID, g.GoalID, g.TodoID, g.Question, encJSON(g.Scope), encJSON(g.Fallback),
		outcomeJSON, encJSON(g.Resolvers), ts(g.CreatedAt), ts(g.ResolvedAt))
	return err
}

// Get returns the gate, or ErrGateNotFound.
func (s *SQLGateStore) Get(ctx context.Context, goalID, gateID string) (UserGate, error) {
	row := s.qr(ctx).QueryRowContext(ctx,
		`SELECT gate_id, goal_id, todo_id, question, scope, fallback, outcome, resolvers, created_at, resolved_at
		 FROM cp_gates WHERE goal_id = ? AND gate_id = ?`, goalID, gateID)
	g, err := scanGate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return UserGate{}, ErrGateNotFound
	}
	return g, err
}

// ListUnresolved returns gates for the goal whose outcome is empty.
func (s *SQLGateStore) ListUnresolved(ctx context.Context, goalID string) ([]UserGate, error) {
	return s.listWhere(ctx, goalID, true)
}

// List returns ALL gates for the goal (resolved + unresolved).
func (s *SQLGateStore) List(ctx context.Context, goalID string) ([]UserGate, error) {
	return s.listWhere(ctx, goalID, false)
}

func (s *SQLGateStore) listWhere(ctx context.Context, goalID string, unresolvedOnly bool) ([]UserGate, error) {
	q := `SELECT gate_id, goal_id, todo_id, question, scope, fallback, outcome, resolvers, created_at, resolved_at
	      FROM cp_gates WHERE goal_id = ?`
	if unresolvedOnly {
		q += ` AND outcome = ''`
	}
	q += ` ORDER BY created_at, gate_id`
	rows, err := s.qr(ctx).QueryContext(ctx, q, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserGate
	for rows.Next() {
		g, err := scanGate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func scanGate(sc scanner) (UserGate, error) {
	var g UserGate
	var scope, fallback, outcome, resolvers, created, resolved string
	if err := sc.Scan(&g.GateID, &g.GoalID, &g.TodoID, &g.Question, &scope, &fallback,
		&outcome, &resolvers, &created, &resolved); err != nil {
		return UserGate{}, err
	}
	decJSONInto(scope, &g.Scope)
	if fallback != "" && fallback != "null" {
		var fb FallbackPolicy
		decJSONInto(fallback, &fb)
		g.Fallback = &fb
	}
	if outcome != "" {
		var oc GateOutcome
		decJSONInto(outcome, &oc)
		g.Outcome = &oc
	}
	decJSONInto(resolvers, &g.Resolvers)
	g.CreatedAt = parseTS(created)
	g.ResolvedAt = parseTS(resolved)
	return g, nil
}

// --- SQLLedger ---

// SQLLedger is a Ledger backed by *sql.DB. Events get a monotonic per-DB seq;
// Read is forward-only, Len supplies the back-cursor for "recent" views.
type SQLLedger struct {
	db *sql.DB
}

// NewSQLLedger returns a Ledger over db.
func NewSQLLedger(db *sql.DB) *SQLLedger { return &SQLLedger{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (l *SQLLedger) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return l.db
}

// Append inserts the event and returns its seq.
func (l *SQLLedger) Append(ctx context.Context, e Event) (int64, error) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	res, err := l.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_events (goal_id, kind, type, todo_id, turn_id, gate_id, outcome, detail, at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		e.GoalID, string(e.Kind), e.Type, e.TodoID, e.TurnID, e.GateID, e.Outcome, encJSON(e.Detail), ts(e.At))
	if err != nil {
		return -1, err
	}
	return res.LastInsertId()
}

// Read returns up to limit events for the goal starting at cursor. cursor is
// 0-based for parity with MemoryLedger; the underlying AUTOINCREMENT seq is
// 1-based, so we bind cursor+1.
func (l *SQLLedger) Read(ctx context.Context, goalID string, cursor int64, limit int) ([]Event, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if cursor < 0 {
		cursor = 0
	}
	rows, err := l.db.QueryContext(ctx,
		`SELECT seq, kind, type, todo_id, turn_id, gate_id, outcome, detail, at
		 FROM cp_events WHERE goal_id = ? AND seq >= ? ORDER BY seq LIMIT ?`, goalID, cursor+1, limit)
	if err != nil {
		return nil, cursor, err
	}
	defer rows.Close()
	var out []Event
	next := cursor
	for rows.Next() {
		var e Event
		var seq int64
		var kind, detail, at string
		if err := rows.Scan(&seq, &kind, &e.Type, &e.TodoID, &e.TurnID, &e.GateID, &e.Outcome, &detail, &at); err != nil {
			return nil, cursor, err
		}
		e.Kind = EventKind(kind)
		decJSONInto(detail, &e.Detail)
		e.At = parseTS(at)
		e.Index = seq - 1 // map 1-based seq to 0-based index
		next = e.Index + 1
		out = append(out, e)
	}
	return out, next, nil
}

// Len returns the total event count for the goal.
func (l *SQLLedger) Len(ctx context.Context, goalID string) (int64, error) {
	var n int64
	err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cp_events WHERE goal_id = ?`, goalID).Scan(&n)
	return n, err
}

// Last returns up to the n most recent events for the goal, in chronological
// order. Implemented as ORDER BY seq DESC LIMIT n then reversed — correct even
// though seq is a global AUTOINCREMENT shared (and interleaved) across goals.
// (The Read cursor arithmetic cannot be used here: per-goal count != global seq.)
func (l *SQLLedger) Last(ctx context.Context, goalID string, n int) ([]Event, error) {
	if n <= 0 {
		return nil, nil
	}
	rows, err := l.db.QueryContext(ctx,
		`SELECT seq, kind, type, todo_id, turn_id, gate_id, outcome, detail, at
		 FROM cp_events WHERE goal_id = ? ORDER BY seq DESC LIMIT ?`, goalID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var desc []Event
	for rows.Next() {
		var e Event
		var seq int64
		var kind, detail, at string
		if err := rows.Scan(&seq, &kind, &e.Type, &e.TodoID, &e.TurnID, &e.GateID, &e.Outcome, &detail, &at); err != nil {
			return nil, err
		}
		e.Kind = EventKind(kind)
		decJSONInto(detail, &e.Detail)
		e.At = parseTS(at)
		e.Index = seq - 1
		desc = append(desc, e)
	}
	// reverse to chronological (oldest-first)
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

// Compact deletes events older than the most-recent keepLastN for the goal and
// inserts a single "history_compacted" marker recording how many were folded.
// The marker is appended AFTER the kept events chronologically (it records the
// compaction time), so subsequent Last views show it at the head.
//
// ponytail: single-statement transaction; if it ever needs to preserve
// pre-compaction audit detail, fold a summary into the marker's Detail JSON.
func (l *SQLLedger) Compact(ctx context.Context, goalID string, keepLastN int) error {
	if keepLastN < 0 {
		keepLastN = 0
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Find the seq threshold: the seq of the (keepLastN)-th newest event.
	var threshold sql.NullInt64
	if keepLastN > 0 {
		row := tx.QueryRowContext(ctx,
			`SELECT MIN(seq) FROM (
			   SELECT seq FROM cp_events WHERE goal_id = ? ORDER BY seq DESC LIMIT ?
			 )`, goalID, keepLastN)
		if err := row.Scan(&threshold); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM cp_events WHERE goal_id = ? AND (? IS NULL OR seq < ?)`,
		goalID, threshold, threshold)
	if err != nil {
		return err
	}
	folded, _ := res.RowsAffected()
	if folded > 0 {
		detail, _ := json.Marshal(map[string]any{"events_folded": folded, "kept": keepLastN})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO cp_events (goal_id, kind, type, detail, at) VALUES (?, ?, ?, ?, ?)`,
			goalID, string(EventDecision), "history_compacted", string(detail), ts(time.Now().UTC())); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- SQLSpendLog ---

// SQLSpendLog is a SpendLog backed by *sql.DB.
type SQLSpendLog struct {
	db *sql.DB
}

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLSpendLog) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads.
func (s *SQLSpendLog) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// NewSQLSpendLog returns a SpendLog over db.
func NewSQLSpendLog(db *sql.DB) *SQLSpendLog { return &SQLSpendLog{db: db} }

// Append records the spend and returns the ALL-TIME total (#4 round-6).
// Windowed counting is the Kernel's job via SpentInWindow.
func (s *SQLSpendLog) Append(ctx context.Context, e SpendEvent) (int, error) {
	if e.SpentAt.IsZero() {
		e.SpentAt = time.Now().UTC()
	}
	if _, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_spend (goal_id, turn_id, slots, reason, spent_at) VALUES (?,?,?,?,?)`,
		e.GoalID, e.TurnID, e.Slots, e.Reason, ts(e.SpentAt)); err != nil {
		return 0, err
	}
	var total int
	err := s.qr(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cp_spend WHERE goal_id = ?`, e.GoalID).Scan(&total)
	return total, err
}

// SpentInWindow counts spend rows for the goal within [now-window, now].
func (s *SQLSpendLog) SpentInWindow(ctx context.Context, goalID string, window time.Duration) (int, error) {
	if window <= 0 {
		window = time.Hour
	}
	cutoff := time.Now().UTC().Add(-window)
	var n int
	err := s.qr(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cp_spend WHERE goal_id = ? AND spent_at >= ?`, goalID, ts(cutoff)).Scan(&n)
	return n, err
}
