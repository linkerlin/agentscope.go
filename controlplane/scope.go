package controlplane

// DecisionScope replaces global gate booleans with a scoped decision. A gate
// is never a vague "waiting for owner": it names a kind, a granularity, and a
// concrete key. This is the core LoopX anti-pattern fix (interaction-pattern-
// catalog decision_scope_v0). Vague scope is a projection bug, never inferred
// from prose.
type DecisionScope struct {
	Kind        ScopeKind        `json:"kind"`
	Granularity ScopeGranularity `json:"granularity"`
	ScopeKey    string           `json:"scope_key"`
}

// ScopeKind is what kind of authority the gate covers.
type ScopeKind string

const (
	ScopePrivateRead ScopeKind = "private_read"
	ScopeWrite       ScopeKind = "write_scope"
	ScopeResource    ScopeKind = "resource"
	ScopeProduction  ScopeKind = "production"
	ScopePublicClaim ScopeKind = "public_claim"
	ScopeDirection   ScopeKind = "direction"
	ScopeOther       ScopeKind = "other"
)

// ScopeGranularity is how wide the decision reaches.
type ScopeGranularity string

const (
	GranularityAction  ScopeGranularity = "action"
	GranularityLane    ScopeGranularity = "lane"
	GranularityGoal    ScopeGranularity = "goal"
	GranularityProject ScopeGranularity = "project"
	GranularityGlobal  ScopeGranularity = "global"
)

// Covers reports whether gate scope g authorizes an action with required scope
// r. A gate covers an action only when both Kind and ScopeKey match (or the
// gate is strictly wider by Granularity). P1 uses exact Kind+ScopeKey match;
// granularity-based widening lands with the projection work in P2.
//
// ponytail: exact match only; granularity widening is a P2 refinement once
// projections need it. The rule "gate does not cover action -> execute
// normally" still holds because uncovered actions never reach this check.
func (g DecisionScope) Covers(r DecisionScope) bool {
	return g.Kind == r.Kind && g.ScopeKey == r.ScopeKey
}

// DecisionOutcome is the resolution of a user gate.
type DecisionOutcome string

const (
	DecisionApprove DecisionOutcome = "approve"
	DecisionReject  DecisionOutcome = "reject"
	DecisionCancel  DecisionOutcome = "cancel"
)
