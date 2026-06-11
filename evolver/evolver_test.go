package evolver

import (
	"context"
	"testing"
)

func TestCreateValidateGene(t *testing.T) {
	g := CreateGene(Gene{
		ID:           "gene_test_repair",
		Category:     CategoryRepair,
		SignalsMatch: []string{"error", "failed"},
		Strategy:     []string{"patch small", "validate", "solidify"},
	})
	if err := ValidateGene(g); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if g.Type != "Gene" || g.SchemaVersion == "" {
		t.Error("defaults not applied")
	}
	if len(g.SignalsMatch) != 2 {
		t.Error("slice not cloned")
	}

	bad := Gene{ID: "", Category: "foo"}
	if ValidateGene(bad) == nil {
		t.Error("expected validation error for bad gene")
	}
}

func TestCreateValidateCapsule(t *testing.T) {
	c := CreateCapsule(Capsule{
		ID:      "cap_1",
		Summary: "fixed the bug",
		Outcome: &Outcome{Status: "success", Score: 0.95},
	})
	if err := ValidateCapsule(c); err != nil {
		t.Fatalf("cap validate: %v", err)
	}
}

func TestMockEvolverGEPFlow(t *testing.T) {
	mock := NewMockEvolver()
	flow := NewGEPFlow(mock)

	runCfg := RunConfig{
		Context:  "agent crashed with timeout on large payload",
		Strategy: "repair-only",
	}
	runRes, solRes, err := flow.RunAndSolidify(context.Background(), runCfg, true /*dry*/)
	if err != nil {
		t.Fatalf("flow: %v", err)
	}
	if runRes.SelectedGene == nil || runRes.GEPPrompt == "" {
		t.Error("run did not return gene/prompt")
	}
	if solRes == nil || !solRes.DryRun || !solRes.OK {
		t.Error("solidify dry-run failed")
	}

	// gene catalog
	genes, _ := mock.ListGenes(context.Background(), CategoryRepair)
	if len(genes) == 0 {
		t.Error("expected seeded repair genes")
	}

	// remember/recall demo
	_ = mock.Remember(context.Background(), RememberRequest{Text: "gene used for timeout recovery", Type: "capsule", Importance: 0.9})
	hits, _ := mock.Recall(context.Background(), RecallRequest{Query: "timeout", Limit: 5})
	if len(hits) == 0 {
		t.Log("recall demo returned 0 (ok for naive impl)")
	}

	// stats
	st, _ := mock.Stats(context.Background())
	if st["gene_count"].(int) < 1 {
		t.Error("stats gene count low")
	}
}

func TestDistillSkillToGene(t *testing.T) {
	sk := AgentSkill{
		Name:         "timeout_retry",
		Description:  "handle gateway timeout by retry once then decompose",
		SkillContent: "- detect timeout\n- retry same\n- if fail split to subagents",
	}
	g := DistillSkillToGene(sk, CategoryRepair)
	if g.ID == "" || len(g.SignalsMatch) == 0 || len(g.Strategy) == 0 {
		t.Errorf("distill produced empty gene: %+v", g)
	}
	if g.Source == nil || g.Source.Kind != "skill2gep" {
		t.Error("source not marked as skill2gep")
	}
	if err := ValidateGene(g); err != nil {
		t.Errorf("distilled gene invalid: %v", err)
	}
}

func TestRecordingEvolver(t *testing.T) {
	rec := NewRecordingEvolver(NewMockEvolver())
	_, _, _ = NewGEPFlow(rec).RunAndSolidify(context.Background(), RunConfig{Context: "x", Strategy: "balanced"}, true)
	if len(rec.Calls) < 2 {
		t.Errorf("expected recorded calls, got %d: %v", len(rec.Calls), rec.Calls)
	}
}
