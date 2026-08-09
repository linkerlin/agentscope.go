package controlplane

import (
	"context"
	"sync"
	"time"
)

// Quota is the per-goal compute budget policy. Compute is a fraction in [0,1]
// of the rolling window the goal is allowed to consume. allowed_slots is
// derived: window_hours*60/slot_minutes*compute. This mirrors LoopX's quota
// schema. Quota is applied LAST in the ShouldRun precedence order, after
// health/gate/evidence/focus waits — it never becomes a second permission
// system.
type Quota struct {
	// Compute is the duty-cycle fraction in [0,1]. Compute<=0 means paused.
	Compute float64 `json:"compute"`
	// WindowHours is the rolling-window length for spend accounting.
	WindowHours float64 `json:"window_hours"`
	// SlotMinutes is the granularity of one accountable slot.
	SlotMinutes float64 `json:"slot_minutes"`
}

// AllowedSlots returns the maximum spendable slots within the rolling window.
// Returns 0 when compute is non-positive or the window/slot is degenerate.
func (q Quota) AllowedSlots() int {
	if q.Compute <= 0 || q.WindowHours <= 0 || q.SlotMinutes <= 0 {
		return 0
	}
	slots := q.WindowHours * 60.0 / q.SlotMinutes * q.Compute
	return int(slots + 0.5) // round to nearest
}

// DefaultQuota is a sane P0 default: 50% duty cycle, 1h window, 15m slots.
func DefaultQuota() Quota {
	return Quota{Compute: 0.5, WindowHours: 1.0, SlotMinutes: 15.0}
}

// SpendEvent is one accounting record: a slot was spent after checks allowed a
// validated turn. The event is NOT permission — it records compute consumed
// after the fact (loopx_quota_slot_spent).
type SpendEvent struct {
	GoalID  string    `json:"goal_id"`
	TurnID  string    `json:"turn_id"`
	Slots   int       `json:"slots"`
	Reason  string    `json:"reason,omitempty"`
	SpentAt time.Time `json:"spent_at"`
}

// SpendLog records SpendEvents and answers rolling-window spend counts. P0
// keeps an in-process slice; P2 upgrades to messagebus.LogAppend / Redis LIST
// for cross-process accounting.
//
// ponytail: in-memory spend log; no persistence. Upgrade path: replace this
// store with a RedisBus-backed LogAppend so multi-process workers share the
// same rolling-window budget.
type SpendLog interface {
	// Append records a spend event and returns the new rolling-window spend
	// total for the goal.
	Append(ctx context.Context, e SpendEvent) (int, error)
	// SpentInWindow returns the count of spend events for the goal within the
	// rolling window ending at now.
	SpentInWindow(ctx context.Context, goalID string, window time.Duration) (int, error)
}

// MemorySpendLog is a concurrency-safe in-process SpendLog.
type MemorySpendLog struct {
	mu   sync.Mutex
	logs map[string][]SpendEvent // goalID -> events
}

// NewMemorySpendLog returns an empty in-memory SpendLog.
func NewMemorySpendLog() *MemorySpendLog {
	return &MemorySpendLog{logs: make(map[string][]SpendEvent)}
}

// Append records the event (UTC) and returns the rolling-window spend total
// (default 1h). This matches SQLSpendLog.Append so both backends report the
// same windowed semantics (#4); ShouldRun and the gateway handler both rely on
// this being a windowed count, not an all-time count.
func (s *MemorySpendLog) Append(_ context.Context, e SpendEvent) (int, error) {
	if e.SpentAt.IsZero() {
		e.SpentAt = time.Now().UTC()
	}
	s.mu.Lock()
	s.logs[e.GoalID] = append(s.logs[e.GoalID], e)
	cutoff := time.Now().UTC().Add(-time.Hour)
	n := 0
	for _, ev := range s.logs[e.GoalID] {
		if !ev.SpentAt.Before(cutoff) {
			n++
		}
	}
	s.mu.Unlock()
	return n, nil
}

// SpentInWindow counts events for the goal within [now-window, now].
func (s *MemorySpendLog) SpentInWindow(_ context.Context, goalID string, window time.Duration) (int, error) {
	if window <= 0 {
		window = time.Hour
	}
	cutoff := time.Now().UTC().Add(-window)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.logs[goalID] {
		if !e.SpentAt.Before(cutoff) {
			n++
		}
	}
	return n, nil
}
