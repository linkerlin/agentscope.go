// Package controlplane implements the long-running agent governance layer
// inspired by LoopX's state kernel. It is a control plane: it decides whether
// a turn should run, owns the durable lifetime goal as source of truth, and
// accounts for validated writeback. It does NOT execute work — the ReAct
// runtime in package agent does that.
//
// P0 ships the spine: Goal kernel + ShouldRun decision + ValidatedWriteback +
// idempotent SpendSlot. User gates, peer claims/leases, projections and
// domain capabilities are deferred to P1-P3.
//
// ponytail: flat package for P0; split into goal/, todo/, quota/ subpackages
// when any bounded context exceeds ~500 lines.
package controlplane

// GoalState is the lifecycle state of a durable lifetime objective.
type GoalState string

const (
	// GoalActive is the normal operating state.
	GoalActive GoalState = "active"
	// GoalPaused is a goal-level hard pause: ShouldRun returns a terminal
	// contract with every automatic permission false.
	GoalPaused GoalState = "paused"
	// GoalCompleted is a terminal success state.
	GoalCompleted GoalState = "completed"
	// GoalAbandoned is a terminal cancellation state.
	GoalAbandoned GoalState = "abandoned"
)

// IsTerminal reports whether s is a terminal goal state (no outgoing transitions).
func (s GoalState) IsTerminal() bool { return s == GoalCompleted || s == GoalAbandoned }

// TodoState is the lifecycle state of a typed todo within a goal.
type TodoState string

const (
	// TodoOpen is an uncompleted, actionable todo.
	TodoOpen TodoState = "open"
	// TodoDone is a terminal completed todo (requires validated evidence).
	TodoDone TodoState = "done"
	// TodoBlocked is a non-terminal todo waiting on a gate or dependency.
	TodoBlocked TodoState = "blocked"
	// TodoDeferred is a terminal retired todo (will not be done).
	TodoDeferred TodoState = "deferred"
)

// IsTerminal reports whether s is a terminal todo state.
func (s TodoState) IsTerminal() bool { return s == TodoDone || s == TodoDeferred }

// TaskClass routes a todo to a lane and drives gate resolution.
type TaskClass string

const (
	// TaskAdvancement advances the objective (primary delivery lane).
	TaskAdvancement TaskClass = "advancement_task"
	// TaskMonitor is a recurring watch; non-gating, quota-neutral polls.
	TaskMonitor TaskClass = "continuous_monitor"
	// TaskUserGate blocks delivery until an owner answers a concrete question.
	TaskUserGate TaskClass = "user_gate"
	// TaskUserAction is a non-blocking visible reminder to the operator.
	TaskUserAction TaskClass = "user_action"
	// TaskBlocker records an external blocker.
	TaskBlocker TaskClass = "blocker"
)

// IsGating reports whether the task class blocks agent delivery. Monitor and
// user-action todos are explicitly non-gating.
func (c TaskClass) IsGating() bool { return c == TaskUserGate || c == TaskBlocker }

// ContinuationPolicy decides successor ownership after a peer completes a todo.
type ContinuationPolicy string

const (
	// ContinuationIndependent leaves the successor unclaimed unless a peer
	// explicitly claims it.
	ContinuationIndependent ContinuationPolicy = "independent_handoff"
	// ContinuationSameAgent keeps the successor with the completing peer.
	ContinuationSameAgent ContinuationPolicy = "same_agent_non_delivery"
)

// ComputeState is the precedence-ordered eligibility state for ShouldRun.
// Order matters: only ComputeEligible yields should_run=true (plus
// outcome-floor recovery, deferred to a later phase). ComputePaused is a
// goal-level hard pause.
type ComputeState string

const (
	// ComputeBlockedHealth: registry/goal health gate failed.
	ComputeBlockedHealth ComputeState = "blocked_health"
	// ComputeOperatorGate: a user gate blocks delivery (P1).
	ComputeOperatorGate ComputeState = "operator_gate"
	// ComputeFocusWait: waiting on focus/assignment (later phase).
	ComputeFocusWait ComputeState = "focus_wait"
	// ComputeEligible: the only state that yields should_run=true.
	ComputeEligible ComputeState = "eligible"
	// ComputeWaiting: awaiting fresh evidence (later phase).
	ComputeWaiting ComputeState = "waiting"
	// ComputePolicyBlocked: an active hard_policy reward record vetoes this goal (#3b).
	ComputePolicyBlocked ComputeState = "policy_blocked"
	// ComputeThrottled: quota budget exhausted in the rolling window.
	ComputeThrottled ComputeState = "throttled"
	// ComputePaused: goal-level hard pause (compute<=0 or explicit pause).
	ComputePaused ComputeState = "paused"
)

// Mode is the interaction contract mode (loopx_interaction_contract_v0).
// It classifies what this turn must do: deliver, ask, wait, self-repair, or
// stay quiet.
type Mode string

const (
	ModeBoundedDelivery             Mode = "bounded_delivery"
	ModeUserGate                    Mode = "user_gate"
	ModeScopedUserGateFallback      Mode = "scoped_user_gate_fallback"
	ModeUserTodoBlockerPush         Mode = "user_todo_blocker_push"
	ModeSuccessorReplanRequired     Mode = "successor_replan_required"
	ModeExternalEvidenceObservation Mode = "external_evidence_observation"
	ModeMonitorQuietSkip            Mode = "monitor_quiet_skip"
	ModeAutonomousReplan            Mode = "autonomous_replan"
	ModeOutcomeFloorRecovery        Mode = "outcome_floor_recovery"
	ModeQuotaThrottled              Mode = "quota_throttled"
	// ModePolicyBlocked: an active hard_policy reward record vetoes the turn (#3b).
	ModePolicyBlocked Mode = "policy_blocked"
)

// NotifyPolicy is the user-channel notification decision.
type NotifyPolicy string

const (
	// Notify informs the user channel.
	Notify NotifyPolicy = "NOTIFY"
	// DontNotify stays quiet on the user channel.
	DontNotify NotifyPolicy = "DONT_NOTIFY"
)

// TurnRoute is the pre-execution routing decision for a turn.
type TurnRoute string

const (
	RouteReadyForHost       TurnRoute = "ready_for_host"
	RouteRepairRequired     TurnRoute = "repair_required"
	RouteReplanRequired     TurnRoute = "replan_required"
	RouteUserActionRequired TurnRoute = "user_action_required"
	RouteWait               TurnRoute = "wait"
	RouteBlocked            TurnRoute = "blocked"
	RouteContractError      TurnRoute = "contract_error"
)

// TurnResultKind is the post-execution result classification.
// Note: "deliver" splits into validated_progress vs validated_completion;
// "stay quiet" is a Mode behavior (ModeMonitorQuietSkip), not a result kind.
type TurnResultKind string

const (
	ResultValidatedProgress   TurnResultKind = "validated_progress"
	ResultValidatedCompletion TurnResultKind = "validated_completion"
	ResultRepairRequired      TurnResultKind = "repair_required"
	ResultReplanRequired      TurnResultKind = "replan_required"
	ResultUserActionRequired  TurnResultKind = "user_action_required"
	ResultWait                TurnResultKind = "wait"
	ResultHostFailure         TurnResultKind = "host_failure"
	ResultValidationFailed    TurnResultKind = "validation_failed"
	ResultWritebackFailed     TurnResultKind = "writeback_failed"
	ResultQuotaSpendFailed    TurnResultKind = "quota_spend_failed"
)

// SchedulerAction is the cross-runtime wait policy (scheduler_hint_v0). It is
// a wait policy, NOT a permission.
type SchedulerAction string

const (
	HintRunNow                         SchedulerAction = "run_now"
	HintBackoffWaitingForUser          SchedulerAction = "backoff_waiting_for_user"
	HintBackoffUntilReassigned         SchedulerAction = "backoff_until_reassigned"
	HintBackoffUntilMaterialTransition SchedulerAction = "backoff_until_material_transition"
	HintBackoffUntilFreshEvidence      SchedulerAction = "backoff_until_fresh_evidence"
	HintStopUntilExplicitResume        SchedulerAction = "stop_until_explicit_resume"
)
