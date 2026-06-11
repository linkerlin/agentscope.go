package evolver

import (
	"errors"
	"fmt"
	"strings"
)

// Categories for Genes (aligned with evolver GEP).
const (
	CategoryRepair   = "repair"
	CategoryOptimize = "optimize"
	CategoryInnovate = "innovate"
	CategoryExplore  = "explore"
)

var ValidCategories = []string{CategoryRepair, CategoryOptimize, CategoryInnovate, CategoryExplore}

// Gene represents a reusable evolution strategy template (GEP Gene).
// Compact carrier for experience vs ad-hoc skills. See evolver gene schema.
type Gene struct {
	Type            string           `json:"type"`
	ID              string           `json:"id"`
	Category        string           `json:"category"`
	SignalsMatch    []string         `json:"signals_match"`
	Strategy        []string         `json:"strategy"`
	Validation      []string         `json:"validation,omitempty"`
	Constraints     *GeneConstraints `json:"constraints,omitempty"`
	Preconditions   []string         `json:"preconditions,omitempty"`
	Summary         string           `json:"summary,omitempty"`
	SchemaVersion   string           `json:"schema_version,omitempty"`
	EpigeneticMarks []string         `json:"epigenetic_marks,omitempty"`
	LearningHistory []any            `json:"learning_history,omitempty"`
	AntiPatterns    []string         `json:"anti_patterns,omitempty"`
	RoutingHint     *RoutingHint     `json:"routing_hint,omitempty"`
	ToolPolicy      *ToolPolicy      `json:"tool_policy,omitempty"`
	Avoid           []string         `json:"avoid,omitempty"`
	AssetID         string           `json:"asset_id,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	// Source info for distilled genes (skill2gep etc.)
	Source *GeneSource `json:"_source,omitempty"`
}

// GeneConstraints limits blast radius of evolution guided by this gene.
type GeneConstraints struct {
	MaxFiles       int      `json:"max_files,omitempty"`
	ForbiddenPaths []string `json:"forbidden_paths,omitempty"`
}

// RoutingHint gives router hints (aligned with evolver/EvoX).
type RoutingHint struct {
	Tier           string `json:"tier,omitempty"`            // cheap/mid/expensive
	ReasoningLevel string `json:"reasoning_level,omitempty"` // off/low/medium/high
}

// ToolPolicy restricts/allows tools for safety.
type ToolPolicy struct {
	AllowOnly []string `json:"allow_only,omitempty"`
	Deny      []string `json:"deny,omitempty"`
	Severity  string   `json:"severity,omitempty"` // warn/block
}

// GeneSource records distillation origin (skill2gep etc).
type GeneSource struct {
	Kind           string `json:"kind,omitempty"`
	SkillName      string `json:"skill_name,omitempty"`
	SkillPlatform  string `json:"skill_platform,omitempty"`
	SkillHash      string `json:"skill_hash,omitempty"`
	RationalePaper string `json:"rationale_paper,omitempty"`
}

// Capsule is a snapshot of a successful evolution outcome.
type Capsule struct {
	Type             string         `json:"type"`
	ID               string         `json:"id"`
	SchemaVersion    string         `json:"schema_version,omitempty"`
	Trigger          []string       `json:"trigger,omitempty"`
	Gene             *Gene          `json:"gene,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	Confidence       float64        `json:"confidence,omitempty"`
	BlastRadius      *BlastRadius   `json:"blast_radius,omitempty"`
	Outcome          *Outcome       `json:"outcome,omitempty"`
	SuccessStreak    int            `json:"success_streak,omitempty"`
	SuccessReason    string         `json:"success_reason,omitempty"`
	SourceType       string         `json:"source_type,omitempty"`
	ReusedAssetID    string         `json:"reused_asset_id,omitempty"`
	DerivationTokens *TokenUsage    `json:"derivation_tokens,omitempty"`
	A2A              *A2AInfo       `json:"a2a,omitempty"`
	Content          any            `json:"content,omitempty"`
	Diff             any            `json:"diff,omitempty"`
	Strategy         []string       `json:"strategy,omitempty"`
	ExecutionTrace   []any          `json:"execution_trace,omitempty"`
	AssetID          string         `json:"asset_id,omitempty"`
	Visibility       string         `json:"visibility,omitempty"`
	Scope            []string       `json:"scope,omitempty"`
	CostTier         string         `json:"cost_tier,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// BlastRadius estimates change size for safety/rollback.
type BlastRadius struct {
	Files int `json:"files"`
	Lines int `json:"lines"`
}

// Outcome of the evolution that produced the capsule.
type Outcome struct {
	Status string  `json:"status"` // success/failed
	Score  float64 `json:"score"`
}

// TokenUsage measured cost at derivation time.
type TokenUsage struct {
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	Basis        string `json:"basis"`
}

// A2AInfo for hub broadcast eligibility.
type A2AInfo struct {
	EligibleToBroadcast bool `json:"eligible_to_broadcast"`
}

// Task from EvoMap Hub (ATP).
type Task struct {
	Type                     string   `json:"type"`
	TaskID                   string   `json:"task_id"`
	Title                    string   `json:"title,omitempty"`
	Signals                  string   `json:"signals,omitempty"`
	Status                   string   `json:"status"`
	ClaimedBy                string   `json:"claimed_by,omitempty"`
	BountyID                 string   `json:"bounty_id,omitempty"`
	BountyAmount             float64  `json:"bounty_amount,omitempty"`
	ComplexityScore          float64  `json:"complexity_score,omitempty"`
	HistoricalCompletionRate float64  `json:"historical_completion_rate,omitempty"`
	Body                     string   `json:"body,omitempty"`
	Description              string   `json:"description,omitempty"`
	ValidationCommands       []string `json:"validation_commands,omitempty"`
	ResultAssetID            string   `json:"result_asset_id,omitempty"`
}

// GEP intents (for solidify etc).
const (
	IntentRepair   = "repair"
	IntentOptimize = "optimize"
	IntentInnovate = "innovate"
)

// Default schema version used.
const DefaultSchemaVersion = "1.6.0"

// CreateGene merges partial into a valid Gene with defaults and normalized slices.
func CreateGene(partial Gene) Gene {
	g := partial // copy
	if g.Type == "" {
		g.Type = "Gene"
	}
	if g.Category == "" || !contains(ValidCategories, g.Category) {
		g.Category = CategoryInnovate
	}
	g.SignalsMatch = cloneStrings(g.SignalsMatch)
	g.Strategy = cloneStrings(g.Strategy)
	g.Validation = cloneStrings(g.Validation)
	g.Preconditions = cloneStrings(g.Preconditions)
	g.EpigeneticMarks = cloneStrings(g.EpigeneticMarks)
	g.AntiPatterns = cloneStrings(g.AntiPatterns)
	g.Avoid = cloneStrings(g.Avoid)

	if g.Constraints == nil {
		g.Constraints = &GeneConstraints{MaxFiles: 20, ForbiddenPaths: []string{".git", "node_modules"}}
	} else {
		if g.Constraints.MaxFiles <= 0 {
			g.Constraints.MaxFiles = 20
		}
		if len(g.Constraints.ForbiddenPaths) == 0 {
			g.Constraints.ForbiddenPaths = []string{".git", "node_modules"}
		}
	}
	if g.SchemaVersion == "" {
		g.SchemaVersion = DefaultSchemaVersion
	}
	if g.Summary == "" && len(g.Strategy) > 0 {
		g.Summary = strings.Join(g.Strategy[:min(1, len(g.Strategy))], "; ")
	}
	if g.Metadata == nil {
		g.Metadata = map[string]any{}
	}
	return g
}

// ValidateGene checks required fields and invariants (aligned with evolver schema).
func ValidateGene(g Gene) error {
	if g.Type != "Gene" {
		return errors.New("Gene.type must be \"Gene\"")
	}
	if g.ID == "" {
		return errors.New("Gene.id is required")
	}
	if !contains(ValidCategories, g.Category) {
		return fmt.Errorf("Gene.category must be one of %v", ValidCategories)
	}
	if len(g.SignalsMatch) == 0 {
		return errors.New("Gene.signals_match must be non-empty array for matching")
	}
	if len(g.Strategy) == 0 {
		return errors.New("Gene.strategy must be non-empty")
	}
	return nil
}

// CreateCapsule normalizes a capsule snapshot.
func CreateCapsule(partial Capsule) Capsule {
	c := partial
	if c.Type == "" {
		c.Type = "Capsule"
	}
	if c.SchemaVersion == "" {
		c.SchemaVersion = DefaultSchemaVersion
	}
	c.Trigger = cloneStrings(c.Trigger)
	c.Strategy = cloneStrings(c.Strategy)
	if c.BlastRadius == nil {
		c.BlastRadius = &BlastRadius{}
	}
	if c.Outcome == nil {
		c.Outcome = &Outcome{Status: "failed", Score: 0}
	}
	if c.A2A == nil {
		c.A2A = &A2AInfo{}
	}
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}
	return c
}

// ValidateCapsule basic checks.
func ValidateCapsule(c Capsule) error {
	if c.Type != "Capsule" {
		return errors.New("Capsule.type must be \"Capsule\"")
	}
	if c.ID == "" {
		return errors.New("Capsule.id is required")
	}
	if c.Outcome == nil || !contains([]string{"success", "failed"}, c.Outcome.Status) {
		return errors.New("Capsule.outcome.status must be success or failed")
	}
	return nil
}

// CreateTask normalizes a hub task.
func CreateTask(partial Task) Task {
	t := partial
	if t.Type == "" {
		t.Type = "Task"
	}
	t.ValidationCommands = cloneStrings(t.ValidationCommands)
	if t.Status == "" {
		t.Status = "open"
	}
	return t
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
