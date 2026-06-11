package evolver

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Evolver is the primary client interface for GEP self-evolution capabilities.
// Implementations can back onto the evolver MCP server (via gateway MCP exposure
// or direct), evolver CLI, or in-memory for tests/demos.
//
// This provides Go-native access to evolver's advantages:
// Genes/Capsules as first-class evolution assets (vs ad-hoc skills),
// run/reflect/solidify pipeline, typed memory (remember/recall), meetings, ATP tasks.
type Evolver interface {
	// Gene/Capsule catalog
	ListGenes(ctx context.Context, category string) ([]Gene, error)
	UpsertGene(ctx context.Context, gene Gene) error
	DeleteGene(ctx context.Context, geneID string) error
	ListCapsules(ctx context.Context, limit int) ([]Capsule, error)

	// Core GEP loop (the heart of evolver advantage)
	Run(ctx context.Context, cfg RunConfig) (*RunResult, error)
	Reflect(ctx context.Context, req ReflectRequest) (*ReflectResult, error)
	Solidify(ctx context.Context, req SolidifyRequest) (*SolidifyResult, error)

	// Typed evolution memory (narrative + graph style via remember/recall)
	Remember(ctx context.Context, req RememberRequest) error
	Recall(ctx context.Context, req RecallRequest) ([]MemoryHit, error)

	// Structured multi-agent evolution meetings
	MeetingStart(ctx context.Context, req MeetingStartRequest) (*Meeting, error)
	MeetingStatus(ctx context.Context, meetingID string) (*MeetingStatus, error)
	// (full meeting control: proceed/human_input/finalize/playback can be added as needed)

	// Hub / ATP tasks
	FetchTasks(ctx context.Context, questions []any) ([]Task, error)
	ClaimTask(ctx context.Context, taskID string) error
	CompleteTask(ctx context.Context, taskID, assetID string) error

	// Ops
	Stats(ctx context.Context) (map[string]any, error)
	SafetyStatus(ctx context.Context) (map[string]any, error)
}

// RunConfig for evolver_run.
type RunConfig struct {
	Context              string
	Strategy             string // balanced|innovate|harden|repair-only
	DriftEnabled         bool
	ExplorationRate      float64
	CycleID              string
	SelectorMode         string // learning|rule|random
	UseHierarchicalBayes bool
}

// RunResult mirrors evolver_run response (signals + selected gene + GEP prompt).
type RunResult struct {
	Signals      []string
	SelectedGene *Gene
	GEPPrompt    string
	RunID        string
	SelectorMode string
}

// ReflectRequest for pre-solidify risk analysis.
type ReflectRequest struct {
	Context         string
	Gene            *Gene
	Signals         []string
	BlastRadius     *BlastRadius
	ProposedChanges string
	ModifiedFiles   []string
}

// ReflectResult contains risk assessment.
type ReflectResult struct {
	Approved         bool
	Risks            []string
	Suggestions      []string
	RecommendedScore float64
}

// SolidifyRequest matches evolver_solidify (core for persisting evolution).
type SolidifyRequest struct {
	Intent                  string
	Summary                 string
	Signals                 []string
	Gene                    *Gene
	Capsule                 *Capsule
	BlastRadius             *BlastRadius
	ModifiedFiles           []string
	GEPOutput               string
	DryRun                  bool
	DecisionSource          string
	PrimaryCause            string
	ContributingFactors     []string
	HumanIntervention       bool
	ManualInterventionCount int
	SelectorMode            string
	RunID                   string
	ReusedAssetID           string
	SourceType              string
}

// SolidifyResult reports outcome of solidify (event recorded, capsule stored etc).
type SolidifyResult struct {
	OK        bool
	EventID   string
	CapsuleID string
	GeneID    string
	DryRun    bool
	Message   string
}

// RememberRequest for typed memory store (gene/capsule/event).
type RememberRequest struct {
	Text       string
	Type       string // gene|capsule|event
	ID         string
	Importance float64
	Category   string
	Scope      string
	Metadata   map[string]any
}

// RecallRequest for hybrid search over evolution memory.
type RecallRequest struct {
	Query    string
	Limit    int
	Scope    string
	Category string
	MinScore float64
}

// MemoryHit is a recalled item with score.
type MemoryHit struct {
	ID       string
	Text     string
	Type     string
	Score    float64
	Source   string
	Metadata map[string]any
}

// MeetingStartRequest starts a structured evolution meeting.
type MeetingStartRequest struct {
	Type    string // research|code|debug|review|grokteam
	Task    string
	Context any
	Signals []string
	Options map[string]any // auto_human_on, max_rounds etc.
}

// Meeting and status simplified (real has stages, artifacts, playback).
type Meeting struct {
	ID   string
	Type string
	Task string
}

type MeetingStatus struct {
	ID         string
	Stage      string
	Progress   float64
	CanProceed bool
	Artifacts  []any
}

// MockEvolver is an in-memory implementation useful for tests, demos and
// when the real evolver server/MCP is not wired. Preloads representative genes.
type MockEvolver struct {
	mu    sync.RWMutex
	genes map[string]Gene
	caps  []Capsule
	mems  []MemoryHit
	runs  int
	solid int
}

func NewMockEvolver() *MockEvolver {
	m := &MockEvolver{
		genes: make(map[string]Gene),
	}
	// Seed from live MCP data + canonical examples (repair/optimize/innovate).
	seeds := []Gene{
		CreateGene(Gene{
			ID:           "gene_gep_repair_from_errors",
			Category:     CategoryRepair,
			SignalsMatch: []string{"error", "exception", "failed", "unstable", "log_error", "recurring_error"},
			Strategy: []string{
				"Extract structured signals from logs and user instructions",
				"Select an existing Gene by signals match (no improvisation)",
				"Estimate blast radius (files, lines) before editing",
				"Apply smallest reversible patch",
				"Validate using declared validation steps; rollback on failure",
				"Solidify knowledge: append EvolutionEvent, update Gene/Capsule store",
			},
			Preconditions: []string{"signals contains error-related indicators"},
			Constraints:   &GeneConstraints{MaxFiles: 20, ForbiddenPaths: []string{".git", "vendor", "node_modules"}},
			Validation:    []string{"go build ./...", "go test ./... -race"},
			Summary:       "Repair from errors using smallest safe patch + validate + solidify.",
		}),
		CreateGene(Gene{
			ID:           "gene_gep_optimize_prompt_and_assets",
			Category:     CategoryOptimize,
			SignalsMatch: []string{"protocol", "gep", "prompt", "audit", "reusable", "optimize"},
			Strategy: []string{
				"Extract signals and determine selection rationale via Selector JSON",
				"Prefer reusing existing Gene/Capsule; only create if no match exists",
				"Refactor prompt assembly to embed assets (genes, capsules, parent event)",
				"Reduce noise and ambiguity; enforce strict output schema",
			},
			Constraints: &GeneConstraints{MaxFiles: 20, ForbiddenPaths: []string{".git", "vendor", "node_modules"}},
		}),
		CreateGene(Gene{
			ID:           "gene_gep_innovate_from_opportunity",
			Category:     CategoryInnovate,
			SignalsMatch: []string{"user_feature_request", "user_improvement_suggestion", "perf_bottleneck", "capability_gap", "external_opportunity"},
			Strategy: []string{
				"Extract opportunity signals and identify the specific user need or system gap",
				"Search existing Genes and Capsules for partial matches (avoid reinventing)",
				"Design a minimal, testable implementation plan (prefer small increments)",
				"Estimate blast radius; innovate changes may touch more files but must stay within constraints",
				"Implement the change with clear validation criteria",
				"Solidify: record EvolutionEvent with intent=innovate, create new Gene if pattern is novel, create Capsule on success",
			},
			Preconditions: []string{"at least one opportunity signal is present", "no active log_error signals (stability first)"},
			Constraints:   &GeneConstraints{MaxFiles: 25, ForbiddenPaths: []string{".git", "vendor", "node_modules"}},
		}),
		CreateGene(Gene{
			ID:           "gene_gep_harden_security",
			Category:     CategoryOptimize,
			SignalsMatch: []string{"security", "injection", "xss", "validation", "sanitize", "harden"},
			Strategy: []string{
				"Identify all user-controlled inputs",
				"Apply strict input validation and sanitization",
				"Use parameterized queries for all SQL",
				"Enforce command whitelist for any shell execution",
			},
			Constraints: &GeneConstraints{MaxFiles: 15, ForbiddenPaths: []string{".git", "vendor"}},
		}),
	}
	for _, g := range seeds {
		m.genes[g.ID] = g
	}
	return m
}

func (m *MockEvolver) ListGenes(_ context.Context, category string) ([]Gene, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Gene{}
	for _, g := range m.genes {
		if category == "" || g.Category == category {
			out = append(out, g)
		}
	}
	return out, nil
}

func (m *MockEvolver) UpsertGene(_ context.Context, gene Gene) error {
	g := CreateGene(gene)
	if err := ValidateGene(g); err != nil {
		return err
	}
	m.mu.Lock()
	m.genes[g.ID] = g
	m.mu.Unlock()
	return nil
}

func (m *MockEvolver) DeleteGene(_ context.Context, geneID string) error {
	m.mu.Lock()
	delete(m.genes, geneID)
	m.mu.Unlock()
	return nil
}

func (m *MockEvolver) ListCapsules(_ context.Context, limit int) ([]Capsule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.caps) {
		limit = len(m.caps)
	}
	out := make([]Capsule, limit)
	copy(out, m.caps[:limit])
	return out, nil
}

func (m *MockEvolver) Run(_ context.Context, cfg RunConfig) (*RunResult, error) {
	m.mu.Lock()
	m.runs++
	m.mu.Unlock()

	// Very simplified selection: pick first matching category or first repair.
	cat := CategoryRepair
	if cfg.Strategy == "innovate" {
		cat = CategoryInnovate
	} else if cfg.Strategy == "harden" || cfg.Strategy == "repair-only" {
		cat = CategoryOptimize
	}
	genes, _ := m.ListGenes(context.Background(), cat)
	var sel *Gene
	if len(genes) > 0 {
		sel = &genes[0]
	} else {
		gg, _ := m.ListGenes(context.Background(), "")
		if len(gg) > 0 {
			sel = &gg[0]
		}
	}
	signals := []string{"demo_signal_from_context"}
	if sel != nil {
		signals = sel.SignalsMatch[:min(3, len(sel.SignalsMatch))]
	}
	prompt := fmt.Sprintf("GEP PROMPT (mock): context=%s strategy=%s gene=%s\nFollow the strategy steps exactly, estimate blast, then call solidify.", cfg.Context, cfg.Strategy, selID(sel))
	return &RunResult{
		Signals:      signals,
		SelectedGene: sel,
		GEPPrompt:    prompt,
		RunID:        fmt.Sprintf("run_%d", m.runs),
		SelectorMode: cfg.SelectorMode,
	}, nil
}

func selID(g *Gene) string {
	if g == nil {
		return "<none>"
	}
	return g.ID
}

func (m *MockEvolver) Reflect(_ context.Context, req ReflectRequest) (*ReflectResult, error) {
	risks := []string{}
	if req.BlastRadius != nil && req.BlastRadius.Files > 15 {
		risks = append(risks, "blast radius large: consider smaller patch or human review")
	}
	approved := len(risks) == 0
	return &ReflectResult{
		Approved:         approved,
		Risks:            risks,
		Suggestions:      []string{"Add more validation steps before solidify"},
		RecommendedScore: 0.85,
	}, nil
}

func (m *MockEvolver) Solidify(_ context.Context, req SolidifyRequest) (*SolidifyResult, error) {
	m.mu.Lock()
	m.solid++
	m.mu.Unlock()

	if req.DryRun {
		return &SolidifyResult{OK: true, DryRun: true, Message: "dry-run ok, would have recorded event + capsule"}, nil
	}
	// In real would persist; here just record a capsule in mock.
	cap := CreateCapsule(Capsule{
		ID:          fmt.Sprintf("cap_%s_%d", req.Intent, m.solid),
		Gene:        req.Gene,
		Summary:     req.Summary,
		Outcome:     &Outcome{Status: "success", Score: 0.9},
		BlastRadius: req.BlastRadius,
	})
	m.mu.Lock()
	m.caps = append(m.caps, cap)
	m.mu.Unlock()

	return &SolidifyResult{
		OK:        true,
		EventID:   fmt.Sprintf("evt_%d", m.solid),
		CapsuleID: cap.ID,
		GeneID:    req.Gene.ID,
		Message:   "solidified (mock)",
	}, nil
}

func (m *MockEvolver) Remember(_ context.Context, req RememberRequest) error {
	m.mu.Lock()
	m.mems = append(m.mems, MemoryHit{
		ID:       req.ID,
		Text:     req.Text,
		Type:     req.Type,
		Score:    req.Importance,
		Metadata: req.Metadata,
	})
	m.mu.Unlock()
	return nil
}

func (m *MockEvolver) Recall(_ context.Context, req RecallRequest) ([]MemoryHit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// naive contains filter for demo
	out := []MemoryHit{}
	for _, h := range m.mems {
		if req.Category != "" && h.Type != req.Category {
			continue
		}
		if stringsContains(h.Text, req.Query) || stringsContains(req.Query, h.Text) {
			out = append(out, h)
		}
	}
	if req.Limit > 0 && len(out) > req.Limit {
		out = out[:req.Limit]
	}
	return out, nil
}

func (m *MockEvolver) MeetingStart(_ context.Context, req MeetingStartRequest) (*Meeting, error) {
	return &Meeting{ID: "meet_demo_1", Type: req.Type, Task: req.Task}, nil
}

func (m *MockEvolver) MeetingStatus(_ context.Context, meetingID string) (*MeetingStatus, error) {
	return &MeetingStatus{ID: meetingID, Stage: "reflect", Progress: 0.6, CanProceed: true}, nil
}

func (m *MockEvolver) FetchTasks(_ context.Context, _ []any) ([]Task, error) { return nil, nil }
func (m *MockEvolver) ClaimTask(_ context.Context, _ string) error           { return nil }
func (m *MockEvolver) CompleteTask(_ context.Context, _, _ string) error     { return nil }

func (m *MockEvolver) Stats(_ context.Context) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]any{
		"gene_count":    len(m.genes),
		"capsule_count": len(m.caps),
		"run_count":     m.runs,
		"solid_count":   m.solid,
	}, nil
}

func (m *MockEvolver) SafetyStatus(_ context.Context) (map[string]any, error) {
	return map[string]any{"self_modification": "allowed_in_dryrun", "source_protected": true}, nil
}

func stringsContains(a, b string) bool {
	return len(a) > 0 && len(b) > 0 && (containsFold(a, b) || containsFold(b, a))
}
func containsFold(a, b string) bool {
	return len(a) >= len(b) && (a == b || len(b) == 0 || (len(a) > 0 && len(b) > 0 && hasSubstr(a, b)))
}
func hasSubstr(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// RecordingEvolver wraps another Evolver and records calls (similar to observability.RecordingTracer).
type RecordingEvolver struct {
	Inner Evolver
	Calls []string
	mu    sync.Mutex
}

func NewRecordingEvolver(inner Evolver) *RecordingEvolver {
	return &RecordingEvolver{Inner: inner}
}

func (r *RecordingEvolver) record(name string) {
	r.mu.Lock()
	r.Calls = append(r.Calls, name)
	r.mu.Unlock()
}

func (r *RecordingEvolver) ListGenes(ctx context.Context, cat string) ([]Gene, error) {
	r.record("ListGenes:" + cat)
	return r.Inner.ListGenes(ctx, cat)
}

// ... (delegate all other methods similarly for brevity in core impl; add as needed)
func (r *RecordingEvolver) Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	r.record("Run")
	return r.Inner.Run(ctx, cfg)
}
func (r *RecordingEvolver) Solidify(ctx context.Context, req SolidifyRequest) (*SolidifyResult, error) {
	r.record("Solidify:" + req.Intent)
	return r.Inner.Solidify(ctx, req)
}
func (r *RecordingEvolver) Reflect(ctx context.Context, req ReflectRequest) (*ReflectResult, error) {
	r.record("Reflect")
	return r.Inner.Reflect(ctx, req)
}

// For other methods, simple delegates (expand if more recording needed):
func (r *RecordingEvolver) UpsertGene(ctx context.Context, g Gene) error {
	r.record("UpsertGene")
	return r.Inner.UpsertGene(ctx, g)
}
func (r *RecordingEvolver) Remember(ctx context.Context, req RememberRequest) error {
	r.record("Remember:" + req.Type)
	return r.Inner.Remember(ctx, req)
}
func (r *RecordingEvolver) Recall(ctx context.Context, req RecallRequest) ([]MemoryHit, error) {
	r.record("Recall")
	return r.Inner.Recall(ctx, req)
}
func (r *RecordingEvolver) Stats(ctx context.Context) (map[string]any, error) {
	r.record("Stats")
	return r.Inner.Stats(ctx)
}
func (r *RecordingEvolver) SafetyStatus(ctx context.Context) (map[string]any, error) {
	r.record("SafetyStatus")
	return r.Inner.SafetyStatus(ctx)
}
func (r *RecordingEvolver) ListCapsules(ctx context.Context, l int) ([]Capsule, error) {
	r.record("ListCapsules")
	return r.Inner.ListCapsules(ctx, l)
}
func (r *RecordingEvolver) DeleteGene(ctx context.Context, id string) error {
	r.record("DeleteGene")
	return r.Inner.DeleteGene(ctx, id)
}
func (r *RecordingEvolver) MeetingStart(ctx context.Context, req MeetingStartRequest) (*Meeting, error) {
	r.record("MeetingStart")
	return r.Inner.MeetingStart(ctx, req)
}
func (r *RecordingEvolver) MeetingStatus(ctx context.Context, id string) (*MeetingStatus, error) {
	r.record("MeetingStatus")
	return r.Inner.MeetingStatus(ctx, id)
}
func (r *RecordingEvolver) FetchTasks(ctx context.Context, q []any) ([]Task, error) {
	r.record("FetchTasks")
	return r.Inner.FetchTasks(ctx, q)
}
func (r *RecordingEvolver) ClaimTask(ctx context.Context, id string) error {
	r.record("ClaimTask")
	return r.Inner.ClaimTask(ctx, id)
}
func (r *RecordingEvolver) CompleteTask(ctx context.Context, tid, aid string) error {
	r.record("CompleteTask")
	return r.Inner.CompleteTask(ctx, tid, aid)
}
