package controlplane

import "sync/atomic"

// MetricsSnapshot is a compact, race-free view of the control plane's runtime
// counters (#3 round-7). Operators can read it to see spend rate, gate load,
// and how often turns were blocked vs allowed — no external metrics system
// required; exporters (prometheus etc.) can poll it.
type MetricsSnapshot struct {
	ShouldRunEligible int64 `json:"should_run_eligible"`
	ShouldRunBlocked  int64 `json:"should_run_blocked"`
	SpendExecuted     int64 `json:"spend_executed"`
	GatesOpened       int64 `json:"gates_opened"`
	GatesResolved     int64 `json:"gates_resolved"`
	Writebacks        int64 `json:"writebacks"`
	Supersedes        int64 `json:"supersedes"`
	RewardsRecorded   int64 `json:"rewards_recorded"`
	RewardsRevoked    int64 `json:"rewards_revoked"`
}

// metrics holds the Kernel's atomic counters.
type metrics struct {
	shouldRunEligible atomic.Int64
	shouldRunBlocked  atomic.Int64
	spendExecuted     atomic.Int64
	gatesOpened       atomic.Int64
	gatesResolved     atomic.Int64
	writebacks        atomic.Int64
	supersedes        atomic.Int64
	rewardsRecorded   atomic.Int64
	rewardsRevoked    atomic.Int64
}

func newMetrics() *metrics { return &metrics{} }

// Snapshot returns a consistent-enough copy of the counters (each is atomic;
// the snapshot as a whole may straddle concurrent updates, which is fine for
// monitoring).
func (m *metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ShouldRunEligible: m.shouldRunEligible.Load(),
		ShouldRunBlocked:  m.shouldRunBlocked.Load(),
		SpendExecuted:     m.spendExecuted.Load(),
		GatesOpened:       m.gatesOpened.Load(),
		GatesResolved:     m.gatesResolved.Load(),
		Writebacks:        m.writebacks.Load(),
		Supersedes:        m.supersedes.Load(),
		RewardsRecorded:   m.rewardsRecorded.Load(),
		RewardsRevoked:    m.rewardsRevoked.Load(),
	}
}
