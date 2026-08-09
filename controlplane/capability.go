package controlplane

import (
	"errors"
	"sort"
	"sync"
)

// CapStatus is the lifecycle status of a capability.
type CapStatus string

const (
	// CapStable is a supported, production-usable capability.
	CapStable CapStatus = "stable"
	// CapExperimental is opt-in and may change; default-off.
	CapExperimental CapStatus = "experimental"
	// CapCompatibilityFacade is a migration facade (do not use for new work).
	CapCompatibilityFacade CapStatus = "compatibility-facade"
)

// CapCommand is one operator command a capability exposes. Each command names
// its purpose and its write_boundary (what it may mutate), so an operator can
// reason about authority without reading the implementation.
type CapCommand struct {
	Name          string `json:"name"`
	Purpose       string `json:"purpose"`
	WriteBoundary string `json:"write_boundary,omitempty"`
}

// LaneStage is one ordered stage of a capability's delivery lane. LoopX's
// issue-fix lane is feasibility -> patch -> checks -> review -> merge. The lane
// is DATA a projection renders; it does not add stages to the Kernel's own
// todo lifecycle (LoopX: capabilities may project domain lanes without adding
// those labels to the Kernel lifecycle).
type LaneStage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Gate  bool   `json:"gate,omitempty"` // does this stage require a user gate?
	Order int    `json:"order"`
}

// Capability is a caller-facing product contract: what the control plane can
// do for an operator, named by caller OUTCOME (not delivery mechanism). This
// mirrors LoopX's BUILTIN_CAPABILITIES catalog: a capability is what LoopX can
// do; an extension is a delivery unit. The two are orthogonal axes sharing one
// registry. A connector/provider/sink is usually an extension provider, NOT a
// capability, unless it has an independently useful caller contract.
//
// ponytail: P3 ships the contract layer (the catalog + lane definitions as
// data). Per-provider execution (real PR/CI providers) is deferred — the lane
// is a projection the operator reads, not an executed pipeline here.
type Capability struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Status     CapStatus    `json:"status"`
	UserValue  string       `json:"user_value"`  // outcome the caller gets
	ProviderID string       `json:"provider_id"` // "controlplane-core" or extension id
	Commands   []CapCommand `json:"commands,omitempty"`
	Lane       []LaneStage  `json:"lane,omitempty"`
	Protocols  []string     `json:"protocols,omitempty"` // implemented schema versions
	Boundaries []string     `json:"boundaries,omitempty"`
}

// CapabilityRegistry holds built-in and extension capabilities, indexed by ID.
// Registration is explicit — a directory does not become a capability by
// location. LoopX discipline: capability vs extension are orthogonal axes; the
// registry is the single source of "what can the control plane do".
type CapabilityRegistry struct {
	mu  sync.RWMutex
	all map[string]Capability
}

// NewCapabilityRegistry returns a registry seeded with the built-in
// capabilities (issue-fix to start). More builtins can be added in future
// without changing call sites.
func NewCapabilityRegistry() *CapabilityRegistry {
	r := &CapabilityRegistry{all: make(map[string]Capability)}
	for _, c := range BuiltinCapabilities() {
		r.all[c.ID] = c
	}
	return r
}

// Register adds or replaces a capability by ID. Returns an error if the
// capability lacks the required caller-outcome contract (ID + UserValue).
func (r *CapabilityRegistry) Register(c Capability) error {
	if c.ID == "" {
		return errors.New("controlplane: capability id required")
	}
	if c.UserValue == "" {
		return errors.New("controlplane: capability user_value (caller outcome) required")
	}
	if c.ProviderID == "" {
		c.ProviderID = "controlplane-core"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all[c.ID] = c
	return nil
}

// Get returns the capability, or ErrCapabilityNotFound.
func (r *CapabilityRegistry) Get(id string) (Capability, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.all[id]
	if !ok {
		return Capability{}, ErrCapabilityNotFound
	}
	return c, nil
}

// List returns all capabilities ordered by ID (stable for projection).
func (r *CapabilityRegistry) List() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Capability, 0, len(r.all))
	for _, c := range r.all {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ErrCapabilityNotFound is returned for unknown capability ids.
var ErrCapabilityNotFound = errors.New("controlplane: capability not found")

// BuiltinCapabilities returns the seed catalog. issue-fix is the flagship:
// a repeatable lane for fixing an open issue end to end, with the user gate at
// the review/merge boundary. Named by caller outcome ("issue-fix"), not by
// mechanism ("github-connector").
func BuiltinCapabilities() []Capability {
	return []Capability{
		{
			ID:         "issue-fix",
			Title:      "Issue Fix",
			Status:     CapStable,
			UserValue:  "Resolve an open issue end to end: reproduce, patch, verify, and surface a reviewable change.",
			ProviderID: "controlplane-core",
			Lane: []LaneStage{
				{ID: "feasibility", Label: "Feasibility", Order: 10},
				{ID: "patch", Label: "Patch", Order: 20},
				{ID: "checks", Label: "Checks", Order: 30},
				{ID: "review", Label: "Review", Gate: true, Order: 40},
				{ID: "merge", Label: "Merge", Gate: true, Order: 50},
			},
			Commands: []CapCommand{
				{Name: "start", Purpose: "Open a goal+todos for the issue lane", WriteBoundary: "goals, todos"},
				{Name: "advance", Purpose: "Move the lane to the next stage", WriteBoundary: "todo state, evidence"},
				{Name: "request-review", Purpose: "Open a user gate at the review boundary", WriteBoundary: "gates"},
			},
			Protocols: []string{"issue_fix_workflow_plan_packet_v0"},
			Boundaries: []string{
				"merge is gated: no auto-merge without an approved review gate",
				"production writes stay with the human; this capability never publishes on the user's behalf",
			},
		},
	}
}
