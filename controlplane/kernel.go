package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Decision is the result of ShouldRun: a pure-state-machine classification of
// what this turn must do. It never calls a model (loopx no_api policy); the
// runtime acts on it. Quota is applied LAST here — it is never a second
// permission system layered over the gates.
type Decision struct {
	ShouldRun          bool            `json:"should_run"`
	State              ComputeState    `json:"state"`
	Route              TurnRoute       `json:"route"`
	Mode               Mode            `json:"mode"`
	Notify             NotifyPolicy    `json:"notify"`
	Scheduler          SchedulerAction `json:"scheduler"`
	CurrentTodoID      string          `json:"current_todo_id,omitempty"`
	Spent              int             `json:"spent"`
	Allowed            int             `json:"allowed"`
	Reason             string          `json:"reason,omitempty"`
	GateID             string          `json:"gate_id,omitempty"`
	Question           string          `json:"question,omitempty"`
	FallbackAuthorized bool            `json:"fallback_authorized,omitempty"`
	Fallback           *FallbackPolicy `json:"fallback,omitempty"`
	// TurnToken is minted when ShouldRunTurn authorizes a turn (#3). SpendSlot
	// can require this token to prove the turn was authorized by the control
	// plane, not bypassed. Empty when token enforcement is off or ShouldRun was
	// used instead of ShouldRunTurn.
	TurnToken string `json:"turn_token,omitempty"`
}

// TickResult is what Tick returns: the decision plus any claim it made. Tick
// does NOT execute work; the ReAct runtime does. After execution the runtime
// calls Writeback then SpendSlot separately.
type TickResult struct {
	Decision      *Decision `json:"decision"`
	ClaimedTodoID string    `json:"claimed_todo_id,omitempty"`
}

// ErrAlreadyClaimed is returned by Claim when another agent owns the todo.
var ErrAlreadyClaimed = errors.New("controlplane: todo already claimed by another agent")

// ErrTodoNotClaimable is returned by Claim when the todo is not open.
var ErrTodoNotClaimable = errors.New("controlplane: todo not in a claimable state")

// ErrNoValidatedWriteback is returned by SpendSlot when no unspent, validated
// delivery exists for the turn. Spend is only legal after validated writeback.
var ErrNoValidatedWriteback = errors.New("controlplane: no validated writeback to spend against")

// ErrAlreadySpent is returned by SpendSlot when the turn's delivery was
// already accounted (idempotent second spend rejected).
var ErrAlreadySpent = errors.New("controlplane: turn delivery already spent")

// ErrNoShouldRunTicket is returned by SpendSlot when ticket enforcement is on
// and the turn was never authorized by ShouldRunTurn/Tick (#3). This prevents a
// runtime from spending quota/gates without first asking the control plane.
var ErrNoShouldRunTicket = errors.New("controlplane: turn not authorized by ShouldRun (no ticket)")

// ErrTicketConsumed is returned when the turn's ticket was already used by a
// prior spend (one ticket per turn).
var ErrTicketConsumed = errors.New("controlplane: turn ticket already consumed")

// ErrStaleTicket is returned by SpendSlot when ticket enforcement is on and the
// turn's eligibility has changed since the ticket was minted (#3 round-3): a
// gate opened, the goal was paused, or quota was exhausted mid-turn. A ticket
// is an authorization snapshot, not a perpetual permit; spend requires the
// turn to STILL be eligible at spend time (LoopX "fresh should_run").
var ErrStaleTicket = errors.New("controlplane: turn no longer eligible since authorization")

// ErrClaimOwnerMismatch is returned by Writeback when the caller's AgentID does
// not match the todo's ClaimedBy (#3b): only the agent that claimed a todo may
// write back its outcome.
var ErrClaimOwnerMismatch = errors.New("controlplane: writeback caller does not own the todo claim")

// ErrLaneGateRequired is returned by Writeback when a todo sits at a gated
// capability lane stage (e.g. issue-fix "review"/"merge") and no approved
// UserGate covers it (#3c). This makes "no auto-merge without approved review"
// an enforced contract, not just capability boundary prose.
var ErrLaneGateRequired = errors.New("controlplane: gated lane stage requires an approved user gate")

// Kernel is the control-plane facade: it wires the Goal/Todo/Gate stores, the
// SpendLog, and the decision-lineage Ledger, and exposes the operators
// ShouldRun, Claim, Writeback, SpendSlot, OpenGate, ResolveGate, and the Tick
// orchestrator. The Kernel owns no durable state of its own — deliveries,
// tickets, rewards, and all entities live in their stores (#4 round-4).
type Kernel struct {
	goals         GoalStore
	todos         TodoStore
	gates         GateStore
	leases        LeaseStore
	spend         SpendLog
	ledger        Ledger
	capabilities  *CapabilityRegistry
	tickets       TicketStore
	rewards       RewardStore
	deliveries    DeliveryStore
	txStarter     func(ctx context.Context, fn func(context.Context) error) error
	enforceTicket bool
}

// NewKernel wires a Kernel over the given stores. Any nil store is replaced
// with an empty in-memory implementation so callers can construct with nil.
// The Kernel always allocates in-memory Gate/Lease stores and a Ledger; pass
// your own via WithGateStore / WithLeaseStore / WithLedger after construction.
func NewKernel(g GoalStore, t TodoStore, s SpendLog) *Kernel {
	if g == nil {
		g = NewMemoryGoalStore()
	}
	if t == nil {
		t = NewMemoryTodoStore()
	}
	if s == nil {
		s = NewMemorySpendLog()
	}
	return &Kernel{
		goals:        g,
		todos:        t,
		gates:        NewMemoryGateStore(),
		leases:       NewMemoryLeaseStore(),
		spend:        s,
		ledger:       NewMemoryLedger(),
		capabilities: NewCapabilityRegistry(),
		tickets:      NewMemoryTicketStore(),
		rewards:      NewMemoryRewardStore(),
		deliveries:   NewMemoryDeliveryStore(),
	}
}

// WithTicketEnforcement turns on #3 spend authorization: SpendSlot then
// requires a fresh ShouldRunTurn ticket for the turn, proving the control plane
// authorized the turn before any quota is spent. Off by default (advisory) for
// backward compatibility; turn on in production to make governance mandatory.
func (k *Kernel) WithTicketEnforcement() *Kernel {
	k.enforceTicket = true
	return k
}

// WithTicketStore replaces the in-process ticket store. Inject a SQLTicketStore
// (or any shared backend) so ticket enforcement works across multiple processes
// sharing one DB — without this, enforceTicket is only correct single-process.
func (k *Kernel) WithTicketStore(ts TicketStore) *Kernel {
	if ts != nil {
		k.tickets = ts
	}
	return k
}

// WithTxStarter wires a transaction wrapper (typically SQLStores.RunInTx) so
// multi-step Kernel mutations become atomic (#4 round-3).
func (k *Kernel) WithTxStarter(ts func(ctx context.Context, fn func(context.Context) error) error) *Kernel {
	k.txStarter = ts
	return k
}

// WithRewardStore wires a reward-memory store (#3b round-4). When set,
// Writeback auto-records run_bound_reward evidence and ShouldRun applies active
// hard_policy records as vetoes, so the 5-class authority model carries real
// data and influences decisions.
func (k *Kernel) WithRewardStore(rs RewardStore) *Kernel {
	if rs != nil {
		k.rewards = rs
	}
	return k
}

// WithDeliveryStore replaces the in-process delivery store so Writeback→Spend
// composes across processes (#4 round-4). Inject a SQLDeliveryStore (shared DB)
// alongside the SQL ticket store for a fully multi-process control plane.
func (k *Kernel) WithDeliveryStore(ds DeliveryStore) *Kernel {
	if ds != nil {
		k.deliveries = ds
	}
	return k
}

// RewardStore exposes the reward store for integrations.
func (k *Kernel) RewardStore() RewardStore { return k.rewards }

// RecordReward adds a classified reward record to the goal's memory. Use this
// to record hard_policy constraints / soft_preference advisories that should
// influence future ShouldRun decisions (run_bound_reward is auto-recorded by
// Writeback and need not be added manually).
func (k *Kernel) RecordReward(ctx context.Context, goalID string, r RewardRecord) error {
	if k.rewards == nil {
		return nil
	}
	if r.Lifecycle == "" {
		r.Lifecycle = LifecycleActive
	}
	if r.Confidence == "" {
		r.Confidence = ConfidenceLow
	}
	return k.rewards.Add(ctx, goalID, r)
}

// laneStageGated reports whether todo t sits at a gated stage of goal's
// capability lane (e.g. issue-fix "review"/"merge"). Returns false if the goal
// has no capability, the capability has no such stage, or the stage is not
// gated (#3c round-4).
func (k *Kernel) laneStageGated(g *Goal, t *Todo) bool {
	if g == nil || g.CapabilityID == "" || t.StageID == "" {
		return false
	}
	c, err := k.capabilities.Get(g.CapabilityID)
	if err != nil {
		return false
	}
	for _, s := range c.Lane {
		if s.ID == t.StageID {
			return s.Gate
		}
	}
	return false
}

// hasApprovedGate reports whether an APPROVED UserGate covers the todo: at
// least one gate bound to the todo with Outcome.Decision == approve, and no
// OPEN gate on the todo. "No gate at all" is NOT approval — the capability
// lane contract requires an explicit approval (#3c).
func (k *Kernel) hasApprovedGate(ctx context.Context, goalID, todoID string) bool {
	all, _ := k.gates.List(ctx, goalID)
	approved, open := false, false
	for _, gt := range all {
		if gt.TodoID != todoID {
			continue
		}
		if !gt.IsResolved() {
			open = true
		} else if gt.Outcome != nil && gt.Outcome.Decision == DecisionApprove {
			approved = true
		}
	}
	return approved && !open
}

// runTx runs fn inside a transaction if a txStarter is wired, else runs fn
// directly (best-effort, single-process semantics).
func (k *Kernel) runTx(ctx context.Context, fn func(context.Context) error) error {
	if k.txStarter == nil {
		return fn(ctx)
	}
	return k.txStarter(ctx, fn)
}

// WithGateStore replaces the Kernel's gate store. Enables tests or apps to
// inject a shared/redis-backed store.
func (k *Kernel) WithGateStore(gs GateStore) *Kernel {
	if gs != nil {
		k.gates = gs
	}
	return k
}

// WithLeaseStore replaces the Kernel's lease store. Inject a messagebus.
// CoordBus-backed adapter to get cross-process hard leases.
func (k *Kernel) WithLeaseStore(ls LeaseStore) *Kernel {
	if ls != nil {
		k.leases = ls
	}
	return k
}

// WithLedger replaces the Kernel's ledger. Enables a cross-process
// messagebus-backed ledger for shared decision lineage.
func (k *Kernel) WithLedger(l Ledger) *Kernel {
	if l != nil {
		k.ledger = l
	}
	return k
}

// Ledger exposes the decision-lineage ledger for read-only inspection (e.g.
// by an operator review board). Returns nil if none is wired.
func (k *Kernel) Ledger() Ledger { return k.ledger }

// GoalStore exposes the goal store so integrations (e.g. a gateway HTTP API)
// can manage durable goals through the Kernel facade without holding a
// separate reference.
func (k *Kernel) GoalStore() GoalStore { return k.goals }

// TodoStore exposes the todo store for integrations.
func (k *Kernel) TodoStore() TodoStore { return k.todos }

// GateStore exposes the gate store for integrations.
func (k *Kernel) GateStore() GateStore { return k.gates }

// SpendLog exposes the spend log for integrations.
func (k *Kernel) SpendLog() SpendLog { return k.spend }

// ReapTickets removes consumed (and stale) tickets older than olderThan, bounding
// the ticket store's growth (#5). Safe to call periodically from a background
// sweeper; errors are non-fatal (reap is best-effort housekeeping).
func (k *Kernel) ReapTickets(ctx context.Context, olderThan time.Duration) error {
	return k.tickets.Reap(ctx, olderThan)
}

// CompactHistory folds the goal's ledger events older than keepLastN into a
// single "history_compacted" marker, bounding cp_events growth (#5). Aligned
// with LoopX run_compaction. The recent window (keepLastN) stays intact for
// ReviewPacket / Kanban.
func (k *Kernel) CompactHistory(ctx context.Context, goalID string, keepLastN int) error {
	if k.ledger == nil {
		return nil
	}
	return k.ledger.Compact(ctx, goalID, keepLastN)
}

// CapabilityRegistry exposes the catalog of built-in + extension capabilities
// (LoopX BUILTIN_CAPABILITIES). Integrations can Register more or read the list
// to render an operator catalog.
func (k *Kernel) CapabilityRegistry() *CapabilityRegistry { return k.capabilities }

// WithCapabilityRegistry replaces the Kernel's capability catalog.
func (k *Kernel) WithCapabilityRegistry(r *CapabilityRegistry) *Kernel {
	if r != nil {
		k.capabilities = r
	}
	return k
}

// record appends an event to the ledger if one is wired; it is best-effort
// and never fails the calling operator (lineage is observability, not control).
func (k *Kernel) record(ctx context.Context, e Event) {
	if k.ledger == nil || e.GoalID == "" {
		return
	}
	_, _ = k.ledger.Append(ctx, e)
}

// containsString reports whether s is in list.
func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ShouldRun is the core decision machine. Precedence (quota applied LAST):
//  1. health (goal exists, not terminal)
//  2. paused (goal-level hard pause or compute<=0 -> terminal contract, all auto perms false)
//  3. operator gate (unresolved user gate -> block delivery; with fallback -> authorize one scoped fallback)
//  4. evidence/focus waits (later phases; pass-through)
//  5. compute quota (the only state that yields should_run=true besides recovery)
//
// ponytail: gate resolution is exact-scope only (DecisionScope.Covers). The
// resolution-matrix rows for "gate does not cover action" and granularity
// widening land with the projection work in P2.
// ShouldRun is the read-only decision (no ticket minted). Use ShouldRunTurn
// when the caller intends to spend against this turn and ticket enforcement is
// on. ShouldRun is appropriate for inspection surfaces (review board, etc.).
func (k *Kernel) ShouldRun(ctx context.Context, goalID, agentID string) (*Decision, error) {
	return k.shouldRun(ctx, goalID, agentID, "")
}

// ShouldRunTurn is the authorizing variant of ShouldRun: it computes the same
// decision AND, when the turn is eligible, mints a one-shot ticket bound to
// (goalID, turnID). SpendSlot consumes that ticket when enforcement is on (#3),
// so a runtime cannot spend without first asking the control plane. The minted
// token is returned in Decision.TurnToken.
func (k *Kernel) ShouldRunTurn(ctx context.Context, goalID, agentID, turnID string) (*Decision, error) {
	return k.shouldRun(ctx, goalID, agentID, turnID)
}

func (k *Kernel) shouldRun(ctx context.Context, goalID, agentID, turnID string) (*Decision, error) {
	g, err := k.goals.Get(ctx, goalID)
	if err != nil {
		return &Decision{
			ShouldRun: false, State: ComputeBlockedHealth, Route: RouteContractError,
			Mode: ModeQuotaThrottled, Notify: DontNotify,
			Scheduler: HintStopUntilExplicitResume, Reason: "goal not found",
		}, nil
	}
	if g.State.IsTerminal() {
		return &Decision{
			ShouldRun: false, State: ComputeBlockedHealth, Route: RouteBlocked,
			Mode: ModeMonitorQuietSkip, Notify: DontNotify,
			Scheduler: HintStopUntilExplicitResume,
			Reason:    "goal terminal: " + string(g.State),
		}, nil
	}
	if g.State == GoalPaused || g.Quota.Compute <= 0 {
		return &Decision{
			ShouldRun: false, State: ComputePaused, Route: RouteBlocked,
			Mode: ModeQuotaThrottled, Notify: DontNotify,
			Scheduler: HintStopUntilExplicitResume,
			Reason:    "goal paused",
		}, nil
	}

	// Operator gate: an unresolved gate blocks delivery. A gate with a declared
	// fallback authorizes exactly that scoped fallback (should_run stays false
	// for the gated lane, but FallbackAuthorized lets the runtime proceed once).
	if unresolved, _ := k.gates.ListUnresolved(ctx, goalID); len(unresolved) > 0 {
		gt := unresolved[0]
		dec := &Decision{
			ShouldRun: false, State: ComputeOperatorGate, Route: RouteUserActionRequired,
			Mode: ModeUserGate, Notify: Notify,
			Scheduler: HintBackoffWaitingForUser,
			GateID:    gt.GateID, Question: gt.Question,
			Reason: "unresolved user gate",
		}
		if gt.Fallback != nil {
			dec.Mode = ModeScopedUserGateFallback
			dec.FallbackAuthorized = true
			dec.Fallback = gt.Fallback
		}
		return dec, nil
	}

	// Policy veto (#3b round-4): an active hard_policy reward record for this
	// goal blocks delivery. hard_policy is a constraint inside a verified scope
	// (set via RecordReward); unlike a user gate it needs no human answer — the
	// operator must revoke/supersede the policy. This makes reward memory an
	// enforced constraint, not just a type definition.
	if k.rewards != nil {
		if recs, _ := k.rewards.List(ctx, goalID); len(recs) > 0 {
			goalScope := DecisionScope{Kind: ScopeDirection, Granularity: GranularityGoal, ScopeKey: goalID}
			for _, r := range SelectByPrecedence(recs, goalScope) {
				if r.Class == AuthorityHardPolicy {
					return &Decision{
						ShouldRun: false, State: ComputePolicyBlocked, Route: RouteBlocked,
						Mode: ModePolicyBlocked, Notify: Notify,
						Scheduler: HintStopUntilExplicitResume,
						Reason:    "blocked by hard_policy: " + r.Content,
					}, nil
				}
			}
		}
	}

	window := time.Duration(g.Quota.WindowHours * float64(time.Hour))
	spent, _ := k.spend.SpentInWindow(ctx, goalID, window)
	allowed := g.Quota.AllowedSlots()
	if allowed > 0 && spent >= allowed {
		return &Decision{
			ShouldRun: false, State: ComputeThrottled, Route: RouteWait,
			Mode: ModeQuotaThrottled, Notify: DontNotify,
			Scheduler: HintBackoffUntilMaterialTransition,
			Spent:     spent, Allowed: allowed,
			Reason: "quota exhausted in rolling window",
		}, nil
	}
	return &Decision{
		ShouldRun: true, State: ComputeEligible, Route: RouteReadyForHost,
		Mode: ModeBoundedDelivery, Notify: DontNotify,
		Scheduler: HintRunNow, CurrentTodoID: g.CurrentTodoID,
		Spent: spent, Allowed: allowed,
		TurnToken: k.mintTicket(ctx, goalID, turnID),
	}, nil
}

// mintTicket records a one-shot authorization for (goalID, turnID) via the
// ticket store and returns an opaque token. Idempotent per turn. Empty turnID
// mints nothing (read-only inspection).
func (k *Kernel) mintTicket(ctx context.Context, goalID, turnID string) string {
	if turnID == "" {
		return ""
	}
	tok, _ := k.tickets.Mint(ctx, goalID, turnID)
	return tok
}

// Claim records soft ownership of an open todo by agentID. Hard leases over
// CoordBus.Lock land in P2; P0 uses the soft ClaimedBy field only.
func (k *Kernel) Claim(ctx context.Context, goalID, todoID, agentID string) error {
	t, err := k.todos.Get(ctx, goalID, todoID)
	if err != nil {
		return err
	}
	if t.State != TodoOpen {
		return ErrTodoNotClaimable
	}
	if t.ClaimedBy != "" && t.ClaimedBy != agentID {
		return ErrAlreadyClaimed
	}
	t.ClaimedBy = agentID
	return k.todos.Upsert(ctx, t)
}

// OpenGate registers a concrete, scoped user gate. Until resolved, ShouldRun
// reports ComputeOperatorGate and blocks delivery on the gated lane. If the
// gate declares a Fallback whose scope is not covered by the gate's own scope,
// the store rejects it (a fallback may never widen authority). Returns
// ErrGateAlreadyResolved if the gate id already exists and is resolved.
func (k *Kernel) OpenGate(ctx context.Context, gate UserGate) error {
	if gate.GateID == "" {
		gate.GateID = uuid.NewString()
	}
	if existing, err := k.gates.Get(ctx, gate.GoalID, gate.GateID); err == nil && existing.IsResolved() {
		return ErrGateAlreadyResolved
	}
	if err := k.gates.Upsert(ctx, gate); err != nil {
		return err
	}
	k.record(ctx, Event{
		Kind: EventDecision, Type: "gate_opened",
		GoalID: gate.GoalID, GateID: gate.GateID, TodoID: gate.TodoID,
		Detail: map[string]any{"question": gate.Question, "scope": gate.Scope},
	})
	return nil
}

// ResolveGate answers an open gate. approve unblocks the gated lane; reject/
// cancel leave it blocked (the runtime decides what to do). Resolving an
// already-resolved gate is an error. The resolution is recorded in the ledger.
func (k *Kernel) ResolveGate(ctx context.Context, goalID, gateID string, outcome GateOutcome) error {
	g, err := k.gates.Get(ctx, goalID, gateID)
	if err != nil {
		return err
	}
	if g.IsResolved() {
		return ErrGateAlreadyResolved
	}
	// Authority: if the gate names resolvers, only they may answer (#5).
	if len(g.Resolvers) > 0 && !containsString(g.Resolvers, outcome.By) {
		return ErrUnauthorizedResolver
	}
	g.Outcome = &outcome
	g.ResolvedAt = time.Now().UTC()
	if err := k.gates.Upsert(ctx, g); err != nil {
		return err
	}
	k.record(ctx, Event{
		Kind: EventDecision, Type: "gate_resolved",
		GoalID: goalID, GateID: gateID,
		Outcome: string(outcome.Decision),
		Detail:  map[string]any{"by": outcome.By, "note": outcome.Note},
	})
	return nil
}

// AcquireLease takes a hard, TTL-bounded lease on one todo for owner. This is
// the contention primitive for peer agents: the unit is (GoalID, TodoID), so
// different todos under one goal run in parallel. ttl<=0 uses DefaultLeaseTTL.
// Returns ErrLeaseHeld if another owner holds a valid lease.
func (k *Kernel) AcquireLease(ctx context.Context, goalID, todoID, owner string, ttl time.Duration) (*Lease, error) {
	l, err := k.leases.Acquire(ctx, goalID, todoID, owner, ttl)
	if err != nil {
		return nil, err
	}
	k.record(ctx, Event{
		Kind: EventDecision, Type: "lease_acquired",
		GoalID: goalID, TodoID: todoID,
		Detail: map[string]any{"owner": owner, "expires_at": l.ExpiresAt},
	})
	return l, nil
}

// ReleaseLease drops the caller's lease on a todo. Releasing a missing lease
// is a no-op; releasing another owner's lease returns ErrLeaseOwnerMismatch.
func (k *Kernel) ReleaseLease(ctx context.Context, goalID, todoID, owner string) error {
	if err := k.leases.Release(ctx, goalID, todoID, owner); err != nil {
		return err
	}
	k.record(ctx, Event{
		Kind: EventDecision, Type: "lease_released",
		GoalID: goalID, TodoID: todoID, Detail: map[string]any{"owner": owner},
	})
	return nil
}

// InspectLease returns the current lease for a todo, or ErrLeaseNotFound.
func (k *Kernel) InspectLease(ctx context.Context, goalID, todoID string) (*Lease, error) {
	return k.leases.Inspect(ctx, goalID, todoID)
}

// Writeback is the only input that may change durable todo state. It validates
// the evidence invariant, applies the legal todo transition, and — for
// accountable outcomes — records a pending delivery that SpendSlot consumes.
// It returns the delivery id (== the turn id) so the caller can spend against
// it. Non-accountable (failed) outcomes record no delivery and change nothing.
func (k *Kernel) Writeback(ctx context.Context, wb ValidatedWriteback) (deliveryID string, err error) {
	if err := wb.Validate(); err != nil {
		return "", err
	}
	t, err := k.todos.Get(ctx, wb.GoalID, wb.TodoID)
	if err != nil {
		return "", err
	}
	// Claim ownership (#3b): if the todo is claimed, only the claiming agent
	// may write back its outcome. This stops a peer from completing a todo it
	// never claimed.
	if t.ClaimedBy != "" && wb.AgentID != "" && t.ClaimedBy != wb.AgentID {
		return "", ErrClaimOwnerMismatch
	}
	evIDs := make([]string, 0, len(wb.Evidence))
	for _, e := range wb.Evidence {
		evIDs = append(evIDs, e.ID)
	}
	t.EvidenceIDs = append(t.EvidenceIDs, evIDs...)
	t.Evidence = append(t.Evidence, wb.Evidence...)

	if wb.Outcome.Status == OutcomeCompletion {
		if !LegalTodoTransition(t.State, TodoDone) {
			return "", ErrTodoNotClaimable
		}
		// Capability lane gate enforcement (#3c round-4): if this todo is at a
		// gated stage of its goal's capability lane, the completion requires an
		// APPROVED UserGate covering the todo. Turns "no auto-merge without
		// approved review" from prose into a check.
		if goal, gerr := k.goals.Get(ctx, wb.GoalID); gerr == nil && k.laneStageGated(goal, t) {
			if !k.hasApprovedGate(ctx, wb.GoalID, wb.TodoID) {
				return "", ErrLaneGateRequired
			}
		}
		t.State = TodoDone
	}
	if err := k.todos.Upsert(ctx, t); err != nil {
		return "", err
	}
	k.record(ctx, Event{
		Kind: EventWork, Type: "validated_writeback",
		GoalID: wb.GoalID, TodoID: wb.TodoID, TurnID: wb.TurnID,
		Outcome: string(wb.Outcome.Status),
		Detail: map[string]any{
			"decision_source": wb.DecisionSource,
			"primary_cause":   wb.PrimaryCause,
			"evidence_count":  len(wb.Evidence),
		},
	})
	// #3b: auto-record a run_bound_reward (evidence about this one outcome). It
	// is the weakest authority class — used as evidence, never as policy — and
	// seeds the reward memory without an explicit RecordReward call.
	if k.rewards != nil {
		_ = k.rewards.Add(ctx, wb.GoalID, RewardRecord{
			Class: AuthorityRunBoundReward, Lifecycle: LifecycleActive,
			Source:  wb.TurnID,
			Scope:   DecisionScope{Kind: ScopeOther, Granularity: GranularityAction, ScopeKey: wb.TodoID},
			Content: wb.PrimaryCause,
		})
	}

	if !wb.Outcome.Status.IsAccountable() {
		return "", nil
	}
	if wb.TurnID == "" {
		wb.TurnID = uuid.NewString()
	}
	// Record the delivery via the shared DeliveryStore (#4 round-4): idempotent
	// per turn and — with a SQL backend — visible to SpendSlot in any process.
	if err := k.deliveries.Record(ctx, Delivery{
		GoalID: wb.GoalID, TurnID: wb.TurnID, TodoID: wb.TodoID,
		Outcome: wb.Outcome.Status, Slots: 1,
	}); err != nil {
		return "", err
	}
	return wb.TurnID, nil
}

// SpendSlot accounts compute consumed by a validated turn. It is dry-run by
// default; Execute appends one SpendEvent. Spend is ONLY legal after a
// validated, unspent writeback for the same turn; quiet skips, preflight
// SpendSlot accounts compute consumed by a validated turn. Dry-run reports the
// projected rolling-window total; Execute appends one SpendEvent. Spend is ONLY
// legal after a validated, unspent writeback for the same turn.
//
// Concurrency (#2 round-3, #4 round-4): there is NO Kernel-wide lock — the
// DeliveryStore.MarkSpent CAS serializes concurrent spends of one turn, and the
// TicketStore CAS does the same when enforcement is on. All DB I/O happens
// without any lock, so latency never blocks other Kernel operations.
//
// With WithTicketEnforcement on (#3), an execute-spend additionally requires a
// fresh ShouldRunTurn ticket for the turn; a read-only eligibility probe rejects
// stale authorizations (gate opened / paused / quota drained since mint).
func (k *Kernel) SpendSlot(ctx context.Context, goalID, turnID string, opts SpendOpts) (int, error) {
	// 1. Load the delivery (shared store; cross-process visible, #4).
	del, err := k.deliveries.Get(ctx, goalID, turnID)
	if err != nil {
		return 0, ErrNoValidatedWriteback
	}
	if del.Spent {
		return 0, ErrAlreadySpent
	}

	// 2. Resolve the quota window + dry-run projection.
	window := time.Hour
	if g, err := k.goals.Get(ctx, goalID); err == nil && g.Quota.WindowHours > 0 {
		window = time.Duration(g.Quota.WindowHours * float64(time.Hour))
	}
	if !opts.Execute {
		spent, _ := k.spend.SpentInWindow(ctx, goalID, window)
		return spent + del.Slots, nil
	}

	// 3. Ticket consume (DB CAS) — serializes same-turn spends when on.
	//    #3 round-3: a ticket is an eligibility SNAPSHOT from ShouldRunTurn. A
	//    gate may have opened, the goal may have paused, or quota may have been
	//    drained since mint time. Re-probe eligibility; reject if it changed.
	if k.enforceTicket {
		if opts.Token == "" {
			return 0, ErrTicketTokenMismatch // token required when enforcement is on
		}
		// Read-only eligibility probe (turnID="" so it does NOT mint/re-mint a
		// ticket — the spend must rely on the ticket minted by ShouldRunTurn,
		// not mint its own). shouldRun with empty turnID mints nothing.
		probe, err := k.shouldRun(ctx, goalID, "", "")
		if err != nil {
			return 0, err
		}
		if !probe.ShouldRun {
			return 0, fmt.Errorf("%w: state=%s", ErrStaleTicket, probe.State)
		}
		if err := k.tickets.Consume(ctx, goalID, turnID, opts.Token); err != nil {
			return 0, err
		}
	}

	// 4. Atomically mark the delivery spent (CAS in the shared store).
	marked, err := k.deliveries.MarkSpent(ctx, goalID, turnID)
	if err != nil {
		return 0, err
	}
	if !marked {
		return 0, ErrAlreadySpent
	}

	// 5. Append the spend event + record lineage.
	ev := SpendEvent{
		GoalID: goalID, TurnID: turnID, Slots: del.Slots,
		Reason: opts.Reason, SpentAt: time.Now().UTC(),
	}
	total, err := k.spend.Append(ctx, ev)
	if err != nil {
		return 0, err
	}
	k.record(ctx, Event{
		Kind: EventAccounting, Type: "quota_slot_spent",
		GoalID: goalID, TurnID: turnID,
		Detail: map[string]any{"slots": del.Slots, "reason": opts.Reason},
	})
	return total, nil
}

// SpendOpts controls SpendSlot behavior. Execute=false is a dry-run that
// reports the projected rolling-window total without committing. Token is the
// TurnToken from a prior ShouldRunTurn; when ticket enforcement + token
// verification are on, SpendSlot verifies it matches the minted ticket (#3).
type SpendOpts struct {
	Execute bool
	Reason  string
	Token   string
}

// Tick is the per-heartbeat orchestrator. It computes the decision and, when
// the turn is eligible, best-effort claims the current todo. It does NOT
// execute work — the ReAct runtime executes after Tick and then calls
// Writeback + SpendSlot.
func (k *Kernel) Tick(ctx context.Context, goalID, agentID, turnID string) (*TickResult, error) {
	dec, err := k.ShouldRunTurn(ctx, goalID, agentID, turnID)
	if err != nil {
		return nil, err
	}
	res := &TickResult{Decision: dec}
	if !dec.ShouldRun {
		return res, nil
	}
	if dec.CurrentTodoID != "" {
		if cerr := k.Claim(ctx, goalID, dec.CurrentTodoID, agentID); cerr == nil {
			res.ClaimedTodoID = dec.CurrentTodoID
		}
	}
	return res, nil
}
