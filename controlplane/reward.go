package controlplane

import (
	"sort"
	"time"
)

// AuthorityClass is the five-class reward-memory authority model (LoopX
// reward-memory-architecture-v0). Separating feedback evidence, policy content,
// and action authority is the point: a useful judgment must NOT silently become
// a universal profile or permissions the actor never held. Confidence never
// widens scope; policy content vs action authority are independent questions.
type AuthorityClass string

const (
	// AuthorityRunBoundReward is evidence about ONE outcome only. Append-only
	// overlay; the weakest authority — used as evidence, never as policy.
	AuthorityRunBoundReward AuthorityClass = "run_bound_reward"
	// AuthorityHardPolicy is a constraint/veto inside a verified scope.
	// Supersedable / revocable / expirable.
	AuthorityHardPolicy AuthorityClass = "hard_policy"
	// AuthoritySoftPreference is advisory ranking/rewrite only. Editable /
	// rejectable / retireable.
	AuthoritySoftPreference AuthorityClass = "soft_preference"
	// AuthorityProceduralExperience is advisory diagnosis/routing AFTER
	// current-artifact verification. Trajectories add-only; experiences
	// supersededable.
	AuthorityProceduralExperience AuthorityClass = "procedural_experience"
	// AuthorityWorkingContext is the current session only — no policy
	// authority. Bound to session/revision.
	AuthorityWorkingContext AuthorityClass = "working_context"
)

// authorityPrecedence is the guarded precedence order (reward-memory v0). Lower
// number = higher authority. Explicit action authority + privacy would rank
// above all (handled by the gate, not by records); run-bound reward ranks last.
var authorityPrecedence = map[AuthorityClass]int{
	AuthorityHardPolicy:           1,
	AuthorityWorkingContext:       2,
	AuthorityProceduralExperience: 3,
	AuthoritySoftPreference:       4,
	AuthorityRunBoundReward:       5,
}

// Confidence is the strength of a reward record. It NEVER widens authority or
// scope — a high-confidence soft preference is still advisory.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// LifecycleState is the durability state of a reward record.
type LifecycleState string

const (
	LifecycleActive     LifecycleState = "active"
	LifecycleSuperseded LifecycleState = "superseded"
	LifecycleRevoked    LifecycleState = "revoked"
	LifecycleExpired    LifecycleState = "expired"
)

// RewardRecord is one classified reward-memory entry. Every durable record
// names its source, scope, authority, confidence, and lifecycle. The authority
// class is independent of the policy content: inference may derive a gate
// policy, but cannot fabricate an operator-gate transition receipt.
type RewardRecord struct {
	Class      AuthorityClass `json:"class"`
	Source     string         `json:"source,omitempty"`
	Scope      DecisionScope  `json:"scope"`
	Authority  string         `json:"authority,omitempty"` // verified actor
	Confidence Confidence     `json:"confidence"`
	Lifecycle  LifecycleState `json:"lifecycle"`
	Content    string         `json:"content"`
	// CreatedAt enables age-based reaping of inactive records (#2 round-5).
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// IsActive reports whether the record may currently exert its (class-bounded) authority.
func (r RewardRecord) IsActive() bool { return r.Lifecycle == "" || r.Lifecycle == LifecycleActive }

// PrecedenceOrder returns the guarded-precedence rank of the record's class
// (lower = stronger). Unknown classes rank last.
func (r RewardRecord) PrecedenceOrder() int {
	if n, ok := authorityPrecedence[r.Class]; ok {
		return n
	}
	return 99
}

// SelectByPrecedence returns the active records for a scope, ordered strongest
// first by guarded precedence (hard_policy > working_context > procedural >
// soft_preference > run_bound_reward). This is the selector an agent uses when
// more than one record applies: the strongest class wins, but ALL active
// records remain visible as evidence — none is silently dropped.
//
// Scope matching (#3 round-5): for an ACTION-level scope, a record applies only
// if its Kind AND ScopeKey both match (Covers). For a GOAL-level scope (used by
// ShouldRun's policy check), Kind is NOT compared — an operator recording "no
// prod writes" with Kind=ScopeProduction must veto the goal even though the
// goal-level query has no single action kind. Only the ScopeKey is matched (and
// an empty record ScopeKey means "global, applies everywhere").
func SelectByPrecedence(records []RewardRecord, scope DecisionScope) []RewardRecord {
	out := make([]RewardRecord, 0, len(records))
	for _, r := range records {
		if !r.IsActive() {
			continue
		}
		if r.Scope.ScopeKey != "" {
			if scope.Granularity == GranularityGoal {
				// Goal-level: match ScopeKey only (kind-agnostic).
				if r.Scope.ScopeKey != scope.ScopeKey {
					continue
				}
			} else if !r.Scope.Covers(scope) {
				// Action-level: Kind AND ScopeKey must match.
				continue
			}
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PrecedenceOrder() < out[j].PrecedenceOrder() })
	return out
}
