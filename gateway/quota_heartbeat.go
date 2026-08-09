package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/controlplane"
)

// QuotaHeartbeat implements the LoopX "timer wakes, quota decides" contract
// (#5 round-6): a scheduler only invokes Tick; ShouldRunTurn decides whether
// the turn may run at all. When should_run=false the tick is a quiet skip
// (no spend, no work). When eligible, the minted TurnToken is passed to
// RunTurn so the bounded turn can write back and spend against it.
//
// This is the operational loop the control plane was designed for — before it
// existed, only manual callers could drive the ticket-enforced flow.
type QuotaHeartbeat struct {
	Kernel  *controlplane.Kernel
	GoalID  string
	AgentID string
	// RunTurn executes one bounded turn: do work, Writeback with evidence,
	// then SpendSlot with the token. Nil means the heartbeat only reports the
	// decision (inspection mode).
	RunTurn func(ctx context.Context, token string) error
}

// Tick runs one heartbeat. turnID must be stable per scheduled tick so the
// ticket idempotency (one token per turn) holds across retries.
func (h *QuotaHeartbeat) Tick(ctx context.Context, turnID string) (*controlplane.Decision, error) {
	dec, err := h.Kernel.ShouldRunTurn(ctx, h.GoalID, h.AgentID, turnID)
	if err != nil {
		return nil, err
	}
	if !dec.ShouldRun {
		return dec, nil // quiet skip: no work, no spend
	}
	if h.RunTurn != nil {
		if err := h.RunTurn(ctx, dec.TurnToken); err != nil {
			return dec, err
		}
	}
	return dec, nil
}
