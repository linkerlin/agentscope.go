package controlplane

import (
	"errors"
	"time"
)

// OutcomeStatus is the high-level result of a bounded turn. Only validated
// outcomes may change durable todo state or spend quota.
type OutcomeStatus string

const (
	// OutcomeProgress: the todo advanced but is not complete.
	OutcomeProgress OutcomeStatus = "progress"
	// OutcomeCompletion: the todo is fully done (transitions to TodoDone).
	OutcomeCompletion OutcomeStatus = "completion"
	// OutcomeFailed: the bounded attempt failed (no state change, no spend).
	OutcomeFailed OutcomeStatus = "failed"
)

// IsAccountable reports whether this outcome authorizes a quota spend. Only
// progress and completion are accountable; failed attempts do not spend.
func (s OutcomeStatus) IsAccountable() bool {
	return s == OutcomeProgress || s == OutcomeCompletion
}

// Outcome is the measured result of a turn. Field shape mirrors
// evolver.Outcome so the two can later share one canonical type.
type Outcome struct {
	Status OutcomeStatus `json:"status"`
	Score  float64       `json:"score,omitempty"`
}

// Evidence is one piece of validated, public-safe proof that a turn produced a
// real transition. Raw logs, trajectories, and verifier tails must be reduced
// to compact form before becoming evidence (LoopX evidence lifecycle).
type Evidence struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // e.g. "test_pass", "diff", "check_run"
	Summary    string    `json:"summary"`
	SourceRef  string    `json:"source_ref,omitempty"` // pointer to public-safe artifact
	ProducedAt time.Time `json:"produced_at"`
}

// ValidatedWriteback is the canonical "accepted writeback" object: the only
// input that may change durable todo state or authorize a quota spend. An
// observation is NOT a transition; a provider receipt is NOT accepted progress
// until evidence is validated and the Kernel commits the state change.
//
// Field design abstracts evolver.SolidifyRequest (DecisionSource / PrimaryCause
// / ContributingFactors / HumanIntervention) so evolver can later call this
// instead of carrying its own duplicate lineage shape.
type ValidatedWriteback struct {
	TodoID              string     `json:"todo_id"`
	GoalID              string     `json:"goal_id"`
	TurnID              string     `json:"turn_id"`
	AgentID             string     `json:"agent_id,omitempty"` // caller; checked vs todo.ClaimedBy (#3b)
	Outcome             Outcome    `json:"outcome"`
	DecisionSource      string     `json:"decision_source,omitempty"`
	PrimaryCause        string     `json:"primary_cause,omitempty"`
	ContributingFactors []string   `json:"contributing_factors,omitempty"`
	HumanIntervention   string     `json:"human_intervention,omitempty"`
	Evidence            []Evidence `json:"evidence"`
}

// Validate enforces the P0 invariants: accountable outcomes require at least
// one piece of evidence; IDs must be present. Returns nil if valid.
func (w ValidatedWriteback) Validate() error {
	if w.GoalID == "" || w.TodoID == "" {
		return errors.New("controlplane: writeback goal_id and todo_id required")
	}
	if w.Outcome.Status.IsAccountable() && len(w.Evidence) == 0 {
		return ErrEvidenceRequired
	}
	return nil
}

// ErrEvidenceRequired is returned when an accountable writeback carries no
// evidence. This is the core "validated writeback" invariant.
var ErrEvidenceRequired = errors.New("controlplane: accountable outcome requires evidence")
