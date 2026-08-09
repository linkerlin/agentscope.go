package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TicketStore persists ShouldRunTurn authorizations so they can be shared across
// processes. Without a shared store, ticket enforcement (#3) is incompatible
// with a multi-process SQL deployment (#1): the process that mints a ticket
// differs from the process that spends. The store is the cross-process bridge.
//
// #2 ships Memory (single-process) and SQL (multi-process) implementations.
type TicketStore interface {
	// Mint records a one-shot authorization for (goalID, turnID) and returns an
	// opaque token. Idempotent per turn: the same turnID returns the same token.
	Mint(ctx context.Context, goalID, turnID string) (string, error)
	// Consume atomically marks the turn's ticket consumed. It returns
	// ErrNoShouldRunTicket if the turn was never minted, or ErrTicketConsumed if
	// already used. The token parameter lets the caller prove it holds the
	// authorization (#3); implementations may verify it.
	Consume(ctx context.Context, goalID, turnID, token string) error
	// Reap removes consumed (and stale) tickets to bound growth (#5).
	Reap(ctx context.Context, olderThan time.Duration) error
}

// --- MemoryTicketStore (single-process; preserves pre-#2 behavior) ---

type memTicket struct {
	token    string
	consumed bool
	mintedAt time.Time
}

// MemoryTicketStore is a concurrency-safe in-process TicketStore.
type MemoryTicketStore struct {
	mu sync.Mutex
	m  map[string]map[string]*memTicket // goalID -> turnID -> ticket
}

// NewMemoryTicketStore returns an empty in-process TicketStore.
func NewMemoryTicketStore() *MemoryTicketStore {
	return &MemoryTicketStore{m: make(map[string]map[string]*memTicket)}
}

// Mint creates or returns the ticket for the turn.
func (s *MemoryTicketStore) Mint(_ context.Context, goalID, turnID string) (string, error) {
	if turnID == "" {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[goalID] == nil {
		s.m[goalID] = make(map[string]*memTicket)
	}
	t, ok := s.m[goalID][turnID]
	if !ok {
		t = &memTicket{token: uuid.NewString(), mintedAt: time.Now().UTC()}
		s.m[goalID][turnID] = t
	}
	return t.token, nil
}

// Consume verifies the turn was minted and not yet consumed, then marks it
// consumed. Token value is verified when non-empty (#3).
func (s *MemoryTicketStore) Consume(_ context.Context, goalID, turnID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gm := s.m[goalID]
	t, ok := gm[turnID]
	if !ok {
		return ErrNoShouldRunTicket
	}
	if t.consumed {
		return ErrTicketConsumed
	}
	if token != "" && t.token != token {
		return ErrTicketTokenMismatch
	}
	t.consumed = true
	return nil
}

// Reap deletes CONSUMED tickets older than olderThan. It must never remove an
// unconsumed ticket: an unspent turn authorization is still valid (the turn may
// be long-running and not have spent yet) and removing it would silently break
// that turn's SpendSlot. (#1 round-3: the prior predicate `consumed || older>0`
// degenerated to "any old ticket" and reaped live authorizations.)
func (s *MemoryTicketStore) Reap(_ context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan)
	s.mu.Lock()
	defer s.mu.Unlock()
	for goalID, gm := range s.m {
		for turnID, t := range gm {
			if t.consumed && t.mintedAt.Before(cutoff) {
				delete(gm, turnID)
			}
		}
		if len(gm) == 0 {
			delete(s.m, goalID)
		}
	}
	return nil
}

// ErrTicketTokenMismatch is returned by Consume when a presented token does not
// match the minted one (#3).
var ErrTicketTokenMismatch = errors.New("controlplane: turn token does not match")

// --- SQLTicketStore (multi-process; shares tickets across processes) ---

// SQLTicketStore is a TicketStore backed by *sql.DB. Multiple processes sharing
// one DB share one ticket pool, so a ShouldRunTurn on process A authorizes a
// SpendSlot on process B.
type SQLTicketStore struct {
	db *sql.DB
}

// NewSQLTicketStore returns a TicketStore over db (assumes InitSchema ran).
func NewSQLTicketStore(db *sql.DB) *SQLTicketStore { return &SQLTicketStore{db: db} }

// ex returns the execer to use for this ctx: a tx if one is attached, else db.
func (s *SQLTicketStore) ex(ctx context.Context) execer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// qr returns the queryer (tx or db) for tx-aware reads.
func (s *SQLTicketStore) qr(ctx context.Context) queryer {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Mint is idempotent: insert-on-conflict-ignore then read back the token.
func (s *SQLTicketStore) Mint(ctx context.Context, goalID, turnID string) (string, error) {
	if turnID == "" {
		return "", nil
	}
	tok := uuid.NewString()
	if _, err := s.ex(ctx).ExecContext(ctx,
		`INSERT INTO cp_tickets (goal_id, turn_id, token, consumed, minted_at)
		 VALUES (?, ?, ?, 0, ?)
		 ON CONFLICT(goal_id, turn_id) DO NOTHING`,
		goalID, turnID, tok, ts(time.Now().UTC())); err != nil {
		return "", err
	}
	var stored string
	if err := s.qr(ctx).QueryRowContext(ctx,
		`SELECT token FROM cp_tickets WHERE goal_id = ? AND turn_id = ?`, goalID, turnID).Scan(&stored); err != nil {
		return "", err
	}
	return stored, nil
}

// Consume atomically marks the ticket consumed via a CAS UPDATE. Token value is
// verified when non-empty (#3).
func (s *SQLTicketStore) Consume(ctx context.Context, goalID, turnID, token string) error {
	q := `UPDATE cp_tickets SET consumed = 1
	      WHERE goal_id = ? AND turn_id = ? AND consumed = 0`
	args := []any{goalID, turnID}
	if token != "" {
		q += ` AND token = ?`
		args = append(args, token)
	}
	res, err := s.ex(ctx).ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return nil
	}
	// Distinguish "never minted" from "already consumed" / "token mismatch".
	var consumed int
	var dbTok string
	err = s.qr(ctx).QueryRowContext(ctx,
		`SELECT consumed, token FROM cp_tickets WHERE goal_id = ? AND turn_id = ?`, goalID, turnID).
		Scan(&consumed, &dbTok)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoShouldRunTicket
	}
	if err != nil {
		return err
	}
	if consumed == 1 {
		return ErrTicketConsumed
	}
	if token != "" && dbTok != token {
		return ErrTicketTokenMismatch
	}
	return ErrNoShouldRunTicket
}

// Reap deletes consumed tickets older than olderThan.
func (s *SQLTicketStore) Reap(ctx context.Context, olderThan time.Duration) error {
	cutoff := ts(time.Now().UTC().Add(-olderThan))
	_, err := s.ex(ctx).ExecContext(ctx,
		`DELETE FROM cp_tickets WHERE consumed = 1 AND minted_at < ?`, cutoff)
	return err
}
