package controlplane

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- P3-1: capability registry ---

func TestCapabilityRegistryBuiltinIssueFix(t *testing.T) {
	r := NewCapabilityRegistry()
	c, err := r.Get("issue-fix")
	require.NoError(t, err)
	assert.Equal(t, CapStable, c.Status)
	assert.Equal(t, "controlplane-core", c.ProviderID)
	assert.NotEmpty(t, c.UserValue)
	// Lane is the flagship 5-stage definition, gated at review+merge.
	assert.Len(t, c.Lane, 5)
	assert.Equal(t, "feasibility", c.Lane[0].ID)
	assert.Equal(t, "merge", c.Lane[4].ID)
	// review and merge stages are gated.
	reviewGate := false
	mergeGate := false
	for _, s := range c.Lane {
		if s.ID == "review" {
			reviewGate = s.Gate
		}
		if s.ID == "merge" {
			mergeGate = s.Gate
		}
	}
	assert.True(t, reviewGate, "review stage must be gated")
	assert.True(t, mergeGate, "merge stage must be gated")
}

func TestCapabilityRegistryRegisterRequiresContract(t *testing.T) {
	r := NewCapabilityRegistry()
	// Missing ID rejected.
	assert.Error(t, r.Register(Capability{UserValue: "x"}))
	// Missing caller-outcome contract rejected.
	assert.Error(t, r.Register(Capability{ID: "thing"}))
	// Valid registration with default provider.
	require.NoError(t, r.Register(Capability{ID: "content-ops", Title: "Content Ops", UserValue: "run a content lane"}))
	got, err := r.Get("content-ops")
	require.NoError(t, err)
	assert.Equal(t, "controlplane-core", got.ProviderID, "default provider assigned")
}

func TestCapabilityRegistryListStableOrder(t *testing.T) {
	r := NewCapabilityRegistry()
	require.NoError(t, r.Register(Capability{ID: "alpha", UserValue: "a"}))
	require.NoError(t, r.Register(Capability{ID: "zebra", UserValue: "z"}))
	list := r.List()
	ids := make([]string, len(list))
	for i, c := range list {
		ids[i] = c.ID
	}
	// Sorted by ID, stable for projection.
	sorted := append([]string{}, ids...)
	// in-place sort to compare
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] > sorted[i] {
			t.Fatalf("list not sorted: %v", ids)
		}
	}
}

func TestCapabilityMissing(t *testing.T) {
	r := NewCapabilityRegistry()
	_, err := r.Get("nope")
	assert.ErrorIs(t, err, ErrCapabilityNotFound)
}

// --- P3-2: reward memory authority classes ---

func TestRewardPrecedenceOrder(t *testing.T) {
	assert.Less(t, (RewardRecord{Class: AuthorityHardPolicy}).PrecedenceOrder(),
		(RewardRecord{Class: AuthorityRunBoundReward}).PrecedenceOrder(),
		"hard policy outranks run-bound reward")
	assert.Less(t, (RewardRecord{Class: AuthoritySoftPreference}).PrecedenceOrder(),
		(RewardRecord{Class: AuthorityRunBoundReward}).PrecedenceOrder())
}

func TestSelectByPrecedenceStrongestFirst(t *testing.T) {
	scope := DecisionScope{Kind: ScopeWrite, ScopeKey: "main.go"}
	records := []RewardRecord{
		{Class: AuthorityRunBoundReward, Scope: scope, Lifecycle: LifecycleActive, Content: "evidence only"},
		{Class: AuthoritySoftPreference, Scope: scope, Lifecycle: LifecycleActive, Content: "prefer X"},
		{Class: AuthorityHardPolicy, Scope: scope, Lifecycle: LifecycleActive, Content: "must not push directly"},
	}
	got := SelectByPrecedence(records, scope)
	require.Len(t, got, 3)
	assert.Equal(t, AuthorityHardPolicy, got[0].Class, "hard policy first")
	assert.Equal(t, AuthoritySoftPreference, got[1].Class)
	assert.Equal(t, AuthorityRunBoundReward, got[2].Class, "run-bound reward last")
}

func TestSelectByPrecedenceScopeFilteringAndInactive(t *testing.T) {
	scope := DecisionScope{Kind: ScopeWrite, ScopeKey: "main.go"}
	other := DecisionScope{Kind: ScopeWrite, ScopeKey: "other.go"}
	records := []RewardRecord{
		{Class: AuthorityHardPolicy, Scope: scope, Lifecycle: LifecycleActive},
		{Class: AuthorityHardPolicy, Scope: other, Lifecycle: LifecycleActive},      // different scope -> excluded
		{Class: AuthoritySoftPreference, Scope: scope, Lifecycle: LifecycleRevoked}, // inactive -> excluded
		{Class: AuthorityRunBoundReward, Scope: scope, Lifecycle: LifecycleActive},
	}
	got := SelectByPrecedence(records, scope)
	require.Len(t, got, 2, "only active in-scope records")
	assert.Equal(t, AuthorityHardPolicy, got[0].Class)
}

func TestConfidenceNeverWidensScope(t *testing.T) {
	// A high-confidence soft preference is still advisory (low precedence).
	r := RewardRecord{Class: AuthoritySoftPreference, Confidence: ConfidenceHigh,
		Scope: DecisionScope{Kind: ScopeWrite, ScopeKey: "x"}, Lifecycle: LifecycleActive}
	assert.True(t, r.IsActive())
	// Confidence does not change the precedence rank.
	assert.Equal(t, authorityPrecedence[AuthoritySoftPreference], r.PrecedenceOrder())
}

// --- P3-3: privacy / public-private boundary ---

func TestRedactEvidenceStripsLocalRefs(t *testing.T) {
	e := Evidence{
		ID: "e1", Kind: "diff", Summary: "patched C:\\users\\alice\\secret.go",
		SourceRef: "file:///home/alice/.local/run.log",
	}
	out := RedactEvidence(e)
	assert.Equal(t, "e1", out.ID, "identity preserved")
	assert.Equal(t, "diff", out.Kind, "kind preserved for auditability")
	assert.Equal(t, "(redacted:local)", out.SourceRef, "local file ref redacted")
	assert.Contains(t, out.Summary, "redacted", "local path in summary scrubbed")
}

func TestRedactEvidenceKeepsPublicSafe(t *testing.T) {
	e := Evidence{ID: "e2", Kind: "test_pass", Summary: "go test ./... green",
		SourceRef: "github.com/owner/repo/pull/42"}
	out := RedactEvidence(e)
	assert.Equal(t, e.SourceRef, out.SourceRef, "public ref kept")
	assert.Equal(t, e.Summary, out.Summary, "clean summary kept")
}

func TestEnforceVisibility(t *testing.T) {
	assert.True(t, EnforceVisibility(VisibilityPublic))
	assert.False(t, EnforceVisibility(VisibilityPrivate))
	assert.False(t, EnforceVisibility(""), "empty defaults to private (safe default)")
}

func TestRedactEvidenceSlice(t *testing.T) {
	in := []Evidence{
		{ID: "a", SourceRef: "file:///x"},
		{ID: "b", SourceRef: "github.com/o/r/pull/1"},
	}
	out := RedactEvidenceSlice(in)
	assert.Equal(t, "(redacted:local)", out[0].SourceRef)
	assert.Equal(t, "github.com/o/r/pull/1", out[1].SourceRef)
}
