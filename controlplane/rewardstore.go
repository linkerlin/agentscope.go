package controlplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RewardStore persists classified reward-memory records per goal (#3b round-4).
// This makes the 5-class authority model (reward.go) actually carry data and
// influence decisions, instead of being dead contract code.
type RewardStore interface {
	Add(ctx context.Context, goalID string, r RewardRecord) error
	List(ctx context.Context, goalID string) ([]RewardRecord, error)
	// Revoke deactivates one record by ID (Lifecycle -> revoked). Without it a
	// misconfigured hard_policy veto permanently blocks a goal — the one-way
	// policy trap. ReapAll only removes INACTIVE records, so revoke is the only
	// way to stop an active policy.
	Revoke(ctx context.Context, goalID, recordID string) error
	// Reap removes inactive (revoked/expired/superseded) records older than
	// olderThan, bounding cp_rewards growth (#2 round-5). run_bound_reward
	// records are auto-added on every Writeback, so without reaping the table
	// grows without bound.
	Reap(ctx context.Context, olderThan time.Duration) error
}

// --- MemoryRewardStore ---

// MemoryRewardStore is a concurrency-safe in-process RewardStore.
type MemoryRewardStore struct {
	mu sync.Mutex
	m  map[string][]RewardRecord
}

// NewMemoryRewardStore returns an empty in-process RewardStore.
func NewMemoryRewardStore() *MemoryRewardStore {
	return &MemoryRewardStore{m: make(map[string][]RewardRecord)}
}

// Add appends a record for the goal (assigns an ID if missing, sets CreatedAt).
func (s *MemoryRewardStore) Add(_ context.Context, goalID string, r RewardRecord) error {
	if goalID == "" {
		return errors.New("controlplane: reward goal_id required")
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[goalID] = append(s.m[goalID], r)
	return nil
}

// Revoke deactivates one record by ID.
func (s *MemoryRewardStore) Revoke(_ context.Context, goalID, recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.m[goalID] {
		if s.m[goalID][i].ID == recordID {
			s.m[goalID][i].Lifecycle = LifecycleRevoked
			return nil
		}
	}
	return ErrRewardNotFound
}

// ErrRewardNotFound is returned by Revoke for an unknown record ID.
var ErrRewardNotFound = errors.New("controlplane: reward record not found")

// List returns all records for the goal (caller may filter by IsActive /
// SelectByPrecedence). Ordered by insertion then class precedence for stability.
func (s *MemoryRewardStore) List(_ context.Context, goalID string) ([]RewardRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]RewardRecord(nil), s.m[goalID]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].PrecedenceOrder() < out[j].PrecedenceOrder() })
	return out, nil
}

// Reap removes inactive records (revoked/expired/superseded) older than
// olderThan. Active records are never removed — they still exert authority.
func (s *MemoryRewardStore) Reap(_ context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan)
	s.mu.Lock()
	defer s.mu.Unlock()
	for goalID, recs := range s.m {
		kept := make([]RewardRecord, 0, len(recs))
		for _, r := range recs {
			if !r.IsActive() && r.CreatedAt.Before(cutoff) {
				continue // reaped
			}
			kept = append(kept, r)
		}
		s.m[goalID] = kept
	}
	return nil
}

// --- SQLRewardStore ---

// SQLRewardStore is a RewardStore backed by *sql.DB.
type SQLRewardStore struct {
	db *sql.DB
}

// NewSQLRewardStore returns a RewardStore over db (assumes InitSchema ran).
func NewSQLRewardStore(db *sql.DB) *SQLRewardStore { return &SQLRewardStore{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLRewardStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads.
func (s *SQLRewardStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Add inserts a reward record row (assigns an ID if missing, uses its CreatedAt
// or now).
func (s *SQLRewardStore) Add(ctx context.Context, goalID string, r RewardRecord) error {
	if goalID == "" {
		return errors.New("controlplane: reward goal_id required")
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	scope, _ := json.Marshal(r.Scope)
	_, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_rewards (id, goal_id, class, source, scope, authority, confidence, lifecycle, content, at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, goalID, string(r.Class), r.Source, string(scope), r.Authority,
		string(r.Confidence), string(r.Lifecycle), r.Content, ts(r.CreatedAt))
	return err
}

// Revoke deactivates one record by ID.
func (s *SQLRewardStore) Revoke(ctx context.Context, goalID, recordID string) error {
	res, err := s.ex(ctx).ExecContext(ctx,
		`UPDATE cp_rewards SET lifecycle = 'revoked' WHERE goal_id = ? AND id = ?`, goalID, recordID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRewardNotFound
	}
	return nil
}

// Reap removes inactive records older than olderThan.
func (s *SQLRewardStore) Reap(ctx context.Context, olderThan time.Duration) error {
	cutoff := ts(time.Now().UTC().Add(-olderThan))
	_, err := s.ex(ctx).ExecContext(ctx,
		`DELETE FROM cp_rewards WHERE lifecycle != 'active' AND at < ?`, cutoff)
	return err
}

// List returns all reward records for the goal, precedence-ordered. CreatedAt
// is read back so age-based reaping behaves identically across backends.
func (s *SQLRewardStore) List(ctx context.Context, goalID string) ([]RewardRecord, error) {
	rows, err := s.qr(ctx).QueryContext(ctx,
		`SELECT id, class, source, scope, authority, confidence, lifecycle, content, at FROM cp_rewards WHERE goal_id = ?`,
		goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RewardRecord
	for rows.Next() {
		var r RewardRecord
		var class, scope, conf, life, at string
		if err := rows.Scan(&r.ID, &class, &r.Source, &scope, &r.Authority, &conf, &life, &r.Content, &at); err != nil {
			return nil, err
		}
		r.Class = AuthorityClass(class)
		r.Confidence = Confidence(conf)
		r.Lifecycle = LifecycleState(life)
		r.CreatedAt = parseTS(at)
		decJSONInto(scope, &r.Scope)
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PrecedenceOrder() < out[j].PrecedenceOrder() })
	return out, nil
}
