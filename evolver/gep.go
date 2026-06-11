package evolver

import (
	"context"
	"fmt"
)

// GEPFlow provides high-level helpers that orchestrate the core evolver advantages:
// run (signal extract + gene select + prompt) -> reflect (risk) -> solidify (persist capsule/gene update).
// These are the "protocol-constrained evolution" primitives that turn ad-hoc tweaks into auditable assets.

type GEPFlow struct {
	Client Evolver
}

// NewGEPFlow returns a flow bound to a client (use NewMockEvolver() for tests or wire a real MCP-backed client).
func NewGEPFlow(c Evolver) *GEPFlow {
	if c == nil {
		c = NewMockEvolver()
	}
	return &GEPFlow{Client: c}
}

// RunAndSolidify is a full (demo) cycle: Run -> Reflect (optional) -> Solidify (with dryRun support).
// In production agents, call Run to get the GEP prompt, have the agent follow it (or use in system prompt),
// then call Reflect + Solidify with the actual changes performed.
func (f *GEPFlow) RunAndSolidify(ctx context.Context, runCfg RunConfig, dryRun bool) (*RunResult, *SolidifyResult, error) {
	runRes, err := f.Client.Run(ctx, runCfg)
	if err != nil {
		return nil, nil, err
	}
	gene := runRes.SelectedGene
	if gene == nil {
		return runRes, nil, fmt.Errorf("no gene selected by evolver run")
	}

	// Optional reflect step (strongly recommended for non-trivial changes).
	refReq := ReflectRequest{
		Context:         runCfg.Context,
		Gene:            gene,
		Signals:         runRes.Signals,
		BlastRadius:     &BlastRadius{Files: 3, Lines: 20}, // agent should compute real
		ProposedChanges: "Apply strategy from selected gene",
	}
	refRes, _ := f.Client.Reflect(ctx, refReq) // ignore err for flow demo
	if refRes != nil && !refRes.Approved {
		// In real flow: surface to human or abort
	} //nolint:staticcheck // SA9003: intentional empty for demo flow

	solReq := SolidifyRequest{
		Intent:         mapStrategyToIntent(runCfg.Strategy),
		Summary:        fmt.Sprintf("Applied GEP gene %s for context: %s", gene.ID, truncate(runCfg.Context, 80)),
		Signals:        runRes.Signals,
		Gene:           gene,
		BlastRadius:    &BlastRadius{Files: 2, Lines: 12},
		ModifiedFiles:  []string{"example/agent.go"},
		DryRun:         dryRun,
		DecisionSource: "gene_selected",
		PrimaryCause:   "strategy_match",
		SelectorMode:   runRes.SelectorMode,
		RunID:          runRes.RunID,
	}
	solRes, err := f.Client.Solidify(ctx, solReq)
	if err != nil {
		return runRes, nil, err
	}
	// Remember the outcome as capsule memory for future recall.
	_ = f.Client.Remember(ctx, RememberRequest{
		Text:       solReq.Summary,
		Type:       "capsule",
		Category:   gene.Category,
		Importance: 0.8,
	})
	return runRes, solRes, nil
}

func mapStrategyToIntent(s string) string {
	switch s {
	case "innovate":
		return IntentInnovate
	case "harden", "repair-only":
		return IntentRepair
	default:
		return IntentOptimize
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// DistillSkillToGene is a starter "skill2gep" distiller (aligns with evolver/src/gep/skillDistiller.js + skill2gep).
// It converts an ad-hoc AgentSkill into a reusable Gene. Real version would use LLM + validation steps extraction.
func DistillSkillToGene(s AgentSkill, category string) Gene {
	if category == "" {
		category = CategoryOptimize
	}
	signals := extractSignals(s.Description + " " + s.Name)
	strategy := extractStrategy(s.SkillContent)
	g := CreateGene(Gene{
		ID:           "gene_distilled_" + sanitizeID(s.Name),
		Category:     category,
		SignalsMatch: signals,
		Strategy:     strategy,
		Summary:      "Distilled from skill: " + s.Name,
		Source: &GeneSource{
			Kind:          "skill2gep",
			SkillName:     s.Name,
			SkillPlatform: "agentscope.go",
		},
	})
	return g
}

// AgentSkill is local alias to avoid import cycle (skill pkg can import evolver later if wanted).
// In real integration move to shared or have skill import evolver.
type AgentSkill struct {
	Name         string
	Description  string
	SkillContent string
	Resources    map[string]string
	Source       string
}

func extractSignals(text string) []string {
	// Very lightweight signal extraction (real uses evolver gep/signals + learningSignals).
	base := []string{"error", "optimize", "feature", "prompt", "tool"}
	out := []string{}
	low := toLower(text)
	for _, b := range base {
		if containsFold(low, b) {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		out = []string{"general_improvement"}
	}
	return out
}

func extractStrategy(content string) []string {
	// Heuristic: split content into strategy bullets. Real: LLM distiller + skill2gep parser.
	lines := splitLines(content)
	start := []string{}
	for _, l := range lines {
		l = trim(l)
		if len(l) > 10 && (startsWith(l, "-") || startsWith(l, "*") || startsWith(l, "1.") || len(start) < 4) {
			start = append(start, l)
		}
	}
	if len(start) == 0 {
		start = []string{"Follow the skill procedure exactly", "Validate outcome", "Solidify learnings as capsule"}
	}
	return start
}

// tiny utils (avoid heavy deps)
func toLower(s string) string { return s } // placeholder, real would unicode fold but sufficient for demo
func splitLines(s string) []string {
	var res []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			res = append(res, s[start:i])
			start = i + 1
		}
	}
	res = append(res, s[start:])
	return res
}
func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '-' || s[0] == '*') {
		s = s[1:]
	}
	return s
}
func startsWith(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func sanitizeID(name string) string {
	// simple
	out := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out += string(r)
		}
	}
	if out == "" {
		out = "skill"
	}
	return out
}
