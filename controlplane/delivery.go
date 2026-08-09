package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// Delivery is one accountable writeback awaiting its single spend: the causal
// link between "a validated turn happened" (Writeback) and "compute was spent"
// (SpendSlot).
type Delivery struct {
	GoalID    string        `json:"goal_id"`
	TurnID    string        `json:"turn_id"`
	TodoID    string        `json:"todo_id"`
	Outcome   OutcomeStatus `json:"outcome"`
	Slots     int           `json:"slots"`
	Spent     bool          `json:"spent"`
	CreatedAt time.Time     `json:"created_at"`
}

// DeliveryStore persists deliveries so Writeback and SpendSlot compose across
// processes (#4 round-4). The ticket store already crosses processes; the
// delivery store must too, or a worker that writes back in process A cannot
// spend in process B. MarkSpent is an atomic CAS so concurrent spends of one
// turn serialize without a Kernel-wide lock.
type DeliveryStore interface {
	Record(ctx context.Context, d Delivery) error
	Get(ctx context.Context, goalID, turnID string) (*Delivery, error)
	// MarkSpent atomically flips spent=1; returns false if already spent.
	MarkSpent(ctx context.Context, goalID, turnID string) (bool, error)
	// Reap removes spent deliveries older than olderThan (#4 growth bound).
	Reap(ctx context.Context, olderThan time.Duration) error
}

// ErrDeliveryNotFound is returned by Get for an unknown (goalID, turnID).
var ErrDeliveryNotFound = errors.New("controlplane: delivery not found")

// --- MemoryDeliveryStore (single-process) ---

// MemoryDeliveryStore is a concurrency-safe in-process DeliveryStore.
type MemoryDeliveryStore struct {
	mu sync.Mutex
	m  map[string]map[string]*Delivery // goalID -> turnID -> delivery
}

// NewMemoryDeliveryStore returns an empty in-process DeliveryStore.
func NewMemoryDeliveryStore() *MemoryDeliveryStore {
	return &MemoryDeliveryStore{m: make(map[string]map[string]*Delivery)}
}

// Record creates the delivery; idempotent per (goalID, turnID).
func (s *MemoryDeliveryStore) Record(_ context.Context, d Delivery) error {
	if d.GoalID == "" || d.TurnID == "" {
		return errors.New("controlplane: delivery goal_id and turn_id required")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Slots <= 0 {
		d.Slots = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[d.GoalID] == nil {
		s.m[d.GoalID] = make(map[string]*Delivery)
	}
	if _, ok := s.m[d.GoalID][d.TurnID]; ok {
		return nil // idempotent
	}
	cp := d
	s.m[d.GoalID][d.TurnID] = &cp
	return nil
}

// Get returns the delivery or ErrDeliveryNotFound.
func (s *MemoryDeliveryStore) Get(_ context.Context, goalID, turnID string) (*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[goalID][turnID]
	if !ok {
		return nil, ErrDeliveryNotFound
	}
	cp := *d
	return &cp, nil
}

// MarkSpent atomically marks the delivery spent; returns false if already so.
func (s *MemoryDeliveryStore) MarkSpent(_ context.Context, goalID, turnID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[goalID][turnID]
	if !ok {
		return false, ErrDeliveryNotFound
	}
	if d.Spent {
		return false, nil
	}
	d.Spent = true
	return true, nil
}

// Reap removes spent deliveries older than olderThan.
func (s *MemoryDeliveryStore) Reap(_ context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan)
	s.mu.Lock()
	defer s.mu.Unlock()
	for goalID, gm := range s.m {
		for turnID, d := range gm {
			if d.Spent && d.CreatedAt.Before(cutoff) {
				delete(gm, turnID)
			}
		}
		if len(gm) == 0 {
			delete(s.m, goalID)
		}
	}
	return nil
}

// --- SQLDeliveryStore (multi-process) ---

// SQLDeliveryStore is a DeliveryStore backed by *sql.DB; multiple processes
// sharing one DB share one delivery pool.
type SQLDeliveryStore struct {
	db *sql.DB
}

// NewSQLDeliveryStore returns a DeliveryStore over db (assumes InitSchema ran).
func NewSQLDeliveryStore(db *sql.DB) *SQLDeliveryStore { return &SQLDeliveryStore{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLDeliveryStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads.
func (s *SQLDeliveryStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Record inserts the delivery; idempotent via ON CONFLICT DO NOTHING.
func (s *SQLDeliveryStore) Record(ctx context.Context, d Delivery) error {
	if d.GoalID == "" || d.TurnID == "" {
		return errors.New("controlplane: delivery goal_id and turn_id required")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Slots <= 0 {
		d.Slots = 1
	}
	_, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_deliveries (goal_id, turn_id, todo_id, outcome, slots, spent, created_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?)
		 ON CONFLICT(goal_id, turn_id) DO NOTHING`,
		d.GoalID, d.TurnID, d.TodoID, string(d.Outcome), d.Slots, ts(d.CreatedAt))
	return err
}

// Get returns the delivery or ErrDeliveryNotFound.
func (s *SQLDeliveryStore) Get(ctx context.Context, goalID, turnID string) (*Delivery, error) {
	var d Delivery
	var outcome, created string
	var spent int
	err := s.qr(ctx).QueryRowContext(ctx,
		`SELECT goal_id, turn_id, todo_id, outcome, slots, spent, created_at
		 FROM cp_deliveries WHERE goal_id = ? AND turn_id = ?`, goalID, turnID).
		Scan(&d.GoalID, &d.TurnID, &d.TodoID, &outcome, &d.Slots, &spent, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeliveryNotFound
	}
	if err != nil {
		return nil, err
	}
	d.Outcome = OutcomeStatus(outcome)
	d.Spent = spent == 1
	d.CreatedAt = parseTS(created)
	return &d, nil
}

// MarkSpent atomically flips spent via a CAS UPDATE; false if already spent.
func (s *SQLDeliveryStore) MarkSpent(ctx context.Context, goalID, turnID string) (bool, error) {
	res, err := s.ex(ctx).ExecContext(ctx,
		`UPDATE cp_deliveries SET spent = 1 WHERE goal_id = ? AND turn_id = ? AND spent = 0`,
		goalID, turnID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return true, nil
	}
	// Either not found or already spent — distinguish for a clean error.
	if _, gerr := s.Get(ctx, goalID, turnID); gerr == ErrDeliveryNotFound {
		return false, ErrDeliveryNotFound
	}
	return false, nil
}

// Reap removes spent deliveries older than olderThan.
func (s *SQLDeliveryStore) Reap(ctx context.Context, olderThan time.Duration) error {
	cutoff := ts(time.Now().UTC().Add(-olderThan))
	_, err := s.ex(ctx).ExecContext(ctx,
		`DELETE FROM cp_deliveries WHERE spent = 1 AND created_at < ?`, cutoff)
	return err
}
