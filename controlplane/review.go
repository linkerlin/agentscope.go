package controlplane

import (
	"context"
	"time"
)

// ReviewQuota is the quota snapshot shown on the operator board.
type ReviewQuota struct {
	Spent       int     `json:"spent"`
	Allowed     int     `json:"allowed"`
	Compute     float64 `json:"compute"`
	WindowHours float64 `json:"window_hours"`
	SlotMinutes float64 `json:"slot_minutes"`
}

// ReviewPacket is the read-only operator surface: the current objective, its
// open todos, pending gates, active reward policies, recent work, the decision
// lineage, and the quota snapshot. It is a PROJECTION — the stores remain the
// source of truth (LoopX "Agent-Native Kanban Is A Projection"). Mutating it
// changes nothing.
type ReviewPacket struct {
	Goal            *Goal          `json:"goal"`
	OpenTodos       []*Todo        `json:"open_todos"`
	PendingGates    []UserGate     `json:"pending_gates"`
	ActivePolicies  []RewardRecord `json:"active_policies"`
	RecentWork      []Event        `json:"recent_work"`
	DecisionLineage []Event        `json:"decision_lineage"`
	Quota           ReviewQuota    `json:"quota"`
}

// ReviewPacket builds the operator-facing projection for a goal. It never
// mutates state. A missing ledger or spend log degrades gracefully (empty
// fields), so the board stays useful mid-construction.
func (k *Kernel) ReviewPacket(ctx context.Context, goalID string) (*ReviewPacket, error) {
	g, err := k.goals.Get(ctx, goalID)
	if err != nil {
		return nil, err
	}
	pkt := &ReviewPacket{Goal: g}

	todos, _ := k.todos.List(ctx, goalID)
	for _, t := range todos {
		if !t.State.IsTerminal() {
			// Public/private boundary (#3a round-4): redact private source refs
			// from projected evidence before they reach an operator surface.
			t.Evidence = RedactEvidenceSlice(t.Evidence)
			pkt.OpenTodos = append(pkt.OpenTodos, t)
		}
	}

	pkt.PendingGates, _ = k.gates.ListUnresolved(ctx, goalID)

	// Surface active reward policies so the operator sees the constraints in
	// force (hard_policy vetoes, soft_preference advisories) — #3b round-4.
	if k.rewards != nil {
		pkt.ActivePolicies, _ = k.rewards.List(ctx, goalID)
	}

	recent := k.readRecent(ctx, goalID, 20)
	for _, e := range recent {
		if e.Kind == EventWork {
			pkt.RecentWork = append(pkt.RecentWork, e)
		}
	}
	pkt.DecisionLineage = recent

	window := time.Duration(g.Quota.WindowHours * float64(time.Hour))
	if window <= 0 {
		window = time.Hour
	}
	spent, _ := k.spend.SpentInWindow(ctx, goalID, window)
	pkt.Quota = ReviewQuota{
		Spent: spent, Allowed: g.Quota.AllowedSlots(),
		Compute: g.Quota.Compute, WindowHours: g.Quota.WindowHours,
		SlotMinutes: g.Quota.SlotMinutes,
	}
	return pkt, nil
}

// readRecent returns up to the last n events for the goal, in chronological
// order. It uses Ledger.Last (ORDER BY ... DESC LIMIT n under the hood) so it
// is correct even when a SQL backend's global AUTOINCREMENT seq interleaves
// events across goals. The earlier Len+Read cursor arithmetic was wrong under
// interleaving.
func (k *Kernel) readRecent(ctx context.Context, goalID string, n int) []Event {
	if k.ledger == nil || n <= 0 {
		return nil
	}
	evs, err := k.ledger.Last(ctx, goalID, n)
	if err != nil {
		return nil
	}
	return evs
}
