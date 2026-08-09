package controlplane

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// EventKind is the class of a ledger event. LoopX reduces all run history to
// four classes: work, decision, accounting, and evidence. The ledger is
// append-only and is the decision-lineage source of truth (event_sourced_
// state_contract_v0).
type EventKind string

const (
	EventWork       EventKind = "work"
	EventDecision   EventKind = "decision"
	EventAccounting EventKind = "accounting"
	EventEvidence   EventKind = "evidence"
)

// Event is one entry in the per-goal run-history ledger.
type Event struct {
	Index   int64          `json:"index"`
	Kind    EventKind      `json:"kind"`
	Type    string         `json:"type"` // e.g. "quota_slot_spent", "gate_opened", "validated_writeback"
	GoalID  string         `json:"goal_id"`
	TodoID  string         `json:"todo_id,omitempty"`
	TurnID  string         `json:"turn_id,omitempty"`
	GateID  string         `json:"gate_id,omitempty"`
	Outcome string         `json:"outcome,omitempty"`
	Detail  map[string]any `json:"detail,omitempty"`
	At      time.Time      `json:"at"`
}

// DetailInt reads an integer-valued Detail key with backend-independent typing
// (#4 round-5): the Memory backend stores native ints while the SQL backend
// round-trips Detail through JSON, yielding float64. Consumers must use these
// accessors instead of reading Detail directly.
func (e Event) DetailInt(key string) (int, bool) {
	v, ok := e.Detail[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// DetailStr reads a string-valued Detail key ("" if absent or non-string).
func (e Event) DetailStr(key string) string {
	if v, ok := e.Detail[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Ledger is the append-only decision-lineage store, scoped per goal. P0/P1
// keep an in-process slice; P2 upgrades to messagebus.LogAppend / Redis LIST
// for cross-process replay and the public-safe timeline projection.
//
// ponytail: in-memory ledger; upgrade path: back Append/Read with
// messagebus.CoordBus.LogAppend so multi-process workers share one lineage.
type Ledger interface {
	Append(ctx context.Context, e Event) (int64, error)
	Read(ctx context.Context, goalID string, cursor int64, limit int) ([]Event, int64, error)
	// Len returns the total number of events for a goal.
	Len(ctx context.Context, goalID string) (int64, error)
	// Last returns up to the n most recent events for the goal, in chronological
	// order (oldest-first within the returned window). This is the correct way
	// to fetch "recent" history: Read's cursor arithmetic assumes contiguous
	// per-goal indices, which holds for the in-memory slice backend but NOT for
	// a SQL backend whose seq is a global AUTOINCREMENT shared across goals.
	// Last uses an explicit "ORDER BY ... DESC LIMIT n" so it is correct under
	// interleaving.
	Last(ctx context.Context, goalID string, n int) ([]Event, error)
	// Compact folds events older than keepLastN for the goal into a single
	// "compacted" summary event, bounding storage growth (#5). Aligned with
	// LoopX run_compaction.
	Compact(ctx context.Context, goalID string, keepLastN int) error
}

// MemoryLedger is a concurrency-safe in-process Ledger.
type MemoryLedger struct {
	mu   sync.Mutex
	logs map[string][]Event // goalID -> events
}

// NewMemoryLedger returns an empty in-memory Ledger.
func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{logs: make(map[string][]Event)}
}

// Append records the event with a monotonic per-goal index and UTC timestamp,
// returning the assigned index.
func (l *MemoryLedger) Append(_ context.Context, e Event) (int64, error) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	idx := int64(len(l.logs[e.GoalID]))
	e.Index = idx
	l.logs[e.GoalID] = append(l.logs[e.GoalID], e)
	return idx, nil
}

// Read returns up to limit events for the goal starting at cursor, plus the
// next cursor (== cursor + len(returned)). limit<=0 means a sane default.
func (l *MemoryLedger) Read(_ context.Context, goalID string, cursor int64, limit int) ([]Event, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	evs := l.logs[goalID]
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= int64(len(evs)) {
		return nil, cursor, nil
	}
	end := cursor + int64(limit)
	if end > int64(len(evs)) {
		end = int64(len(evs))
	}
	out := append([]Event(nil), evs[cursor:end]...)
	return out, end, nil
}

// Len returns the total event count for the goal.
func (l *MemoryLedger) Len(_ context.Context, goalID string) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int64(len(l.logs[goalID])), nil
}

// Last returns up to the n most recent events for the goal, chronological
// (oldest-first within the window).
func (l *MemoryLedger) Last(_ context.Context, goalID string, n int) ([]Event, error) {
	if n <= 0 {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	evs := l.logs[goalID]
	if len(evs) == 0 {
		return nil, nil
	}
	if n > len(evs) {
		n = len(evs)
	}
	out := append([]Event(nil), evs[len(evs)-n:]...)
	return out, nil
}

// Compact replaces the goal's event slice with its last keepLastN events plus a
// leading "compacted" summary event recording how many were folded.
func (l *MemoryLedger) Compact(_ context.Context, goalID string, keepLastN int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	evs := l.logs[goalID]
	if keepLastN < 0 {
		keepLastN = 0
	}
	if len(evs) <= keepLastN {
		return nil
	}
	folded := len(evs) - keepLastN
	compacted := Event{
		Kind: EventDecision, Type: "history_compacted",
		GoalID: goalID,
		Detail: map[string]any{"events_folded": folded, "kept": keepLastN},
		At:     time.Now().UTC(),
	}
	kept := append([]Event(nil), evs[len(evs)-keepLastN:]...)
	next := append([]Event{compacted}, kept...)
	l.logs[goalID] = next
	return nil
}
