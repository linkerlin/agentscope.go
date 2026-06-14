package evolver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockMCPCaller captures the last call and returns a preset response.
type mockMCPCaller struct {
	toolName string
	args     map[string]any
	resp     map[string]any
	err      error
}

func (m *mockMCPCaller) call(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	m.toolName = toolName
	m.args = args
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestMCPEvolver_ListGenes(t *testing.T) {
	mc := &mockMCPCaller{resp: map[string]any{
		"genes": []any{
			map[string]any{"id": "g1", "name": "repair"},
		},
	}}
	e := NewMCPEvolver(mc.call)

	genes, err := e.ListGenes(context.Background(), "repair")
	if err != nil {
		t.Fatal(err)
	}
	if len(genes) != 1 || genes[0].ID != "g1" {
		t.Fatalf("unexpected genes: %+v", genes)
	}
	if mc.toolName != toolListGenes {
		t.Errorf("expected tool %s, got %s", toolListGenes, mc.toolName)
	}
	if mc.args["category"] != "repair" {
		t.Errorf("expected category=repair, got %v", mc.args["category"])
	}
}

func TestMCPEvolver_UpsertGene(t *testing.T) {
	mc := &mockMCPCaller{resp: map[string]any{}}
	e := NewMCPEvolver(mc.call)

	gene := &Gene{ID: "g1", Category: "test"}
	err := e.UpsertGene(context.Background(), *gene)
	if err != nil {
		t.Fatal(err)
	}
	if mc.toolName != toolUpsertGene {
		t.Errorf("wrong tool: %s", mc.toolName)
	}
}

func TestMCPEvolver_DeleteGene(t *testing.T) {
	mc := &mockMCPCaller{resp: map[string]any{}}
	e := NewMCPEvolver(mc.call)

	err := e.DeleteGene(context.Background(), "g1")
	if err != nil {
		t.Fatal(err)
	}
	if mc.args["gene_id"] != "g1" {
		t.Errorf("expected gene_id=g1, got %v", mc.args["gene_id"])
	}
}

func TestMCPEvolver_Run(t *testing.T) {
	mc := &mockMCPCaller{resp: map[string]any{
		"RunID": "r1",
	}}
	e := NewMCPEvolver(mc.call)

	result, err := e.Run(context.Background(), RunConfig{Context: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != "r1" {
		t.Fatalf("expected run_id=r1, got %q", result.RunID)
	}
}

func TestMCPEvolver_Solidify_AllFields(t *testing.T) {
	var captured map[string]any
	mc := &mockMCPCaller{resp: map[string]any{"ok": true}}
	e := NewMCPEvolver(func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
		captured = args
		return mc.resp, nil
	})

	req := SolidifyRequest{
		Intent:                  "fix",
		ManualInterventionCount: 2,
		ReusedAssetID:           "asset-1",
		SourceType:              "reflection",
	}
	_, err := e.Solidify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify previously-missing fields are now passed
	if captured["manual_intervention_count"] != 2 {
		t.Errorf("manual_intervention_count not passed: %v", captured["manual_intervention_count"])
	}
	if captured["reused_asset_id"] != "asset-1" {
		t.Errorf("reused_asset_id not passed: %v", captured["reused_asset_id"])
	}
	if captured["source_type"] != "reflection" {
		t.Errorf("source_type not passed: %v", captured["source_type"])
	}
}

func TestMCPEvolver_Remember_AllFields(t *testing.T) {
	var captured map[string]any
	e := NewMCPEvolver(func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
		captured = args
		return map[string]any{}, nil
	})

	req := RememberRequest{
		Text:     "memory",
		Type:     "event",
		ID:       "m1",
		Scope:    "global",
		Metadata: map[string]any{"k": "v"},
	}
	err := e.Remember(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if captured["id"] != "m1" {
		t.Errorf("id not passed: %v", captured["id"])
	}
	if captured["scope"] != "global" {
		t.Errorf("scope not passed: %v", captured["scope"])
	}
	if captured["metadata"] == nil {
		t.Error("metadata not passed")
	}
}

func TestMCPEvolver_Recall_AllFields(t *testing.T) {
	var captured map[string]any
	e := NewMCPEvolver(func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
		captured = args
		return map[string]any{"hits": []any{}}, nil
	})

	req := RecallRequest{
		Query:    "test",
		Scope:    "local",
		MinScore: 0.8,
	}
	_, err := e.Recall(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if captured["scope"] != "local" {
		t.Errorf("scope not passed: %v", captured["scope"])
	}
	if captured["min_score"] != 0.8 {
		t.Errorf("min_score not passed: %v", captured["min_score"])
	}
}

func TestMCPEvolver_Stats(t *testing.T) {
	mc := &mockMCPCaller{resp: map[string]any{"genes": 10, "capsules": 5}}
	e := NewMCPEvolver(mc.call)

	stats, err := e.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats["genes"] != 10 {
		t.Errorf("expected genes=10, got %v", stats["genes"])
	}
}

func TestMCPEvolver_ErrorHandling(t *testing.T) {
	mc := &mockMCPCaller{err: errors.New("connection refused")}
	e := NewMCPEvolver(mc.call)

	_, err := e.ListGenes(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMCPEvolverFromClient(t *testing.T) {
	mockClient := &mockMCPClient{
		resp: map[string]any{"genes": []any{}},
	}
	e := NewMCPEvolverFromClient(mockClient)

	genes, err := e.ListGenes(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(genes) != 0 {
		t.Fatalf("expected 0 genes, got %d", len(genes))
	}
	if mockClient.lastTool != toolListGenes {
		t.Errorf("expected tool %s, got %s", toolListGenes, mockClient.lastTool)
	}
}

func TestMCPEvolverFromClient_StringResult(t *testing.T) {
	mockClient := &mockMCPClient{
		resp: `{"genes": [{"id": "g1", "name": "test"}]}`,
	}
	e := NewMCPEvolverFromClient(mockClient)

	genes, err := e.ListGenes(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(genes) != 1 || genes[0].ID != "g1" {
		t.Fatalf("unexpected genes: %+v", genes)
	}
}

func TestParseMCPResult(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  map[string]any
	}{
		{"nil", nil, map[string]any{}},
		{"map", map[string]any{"a": 1}, map[string]any{"a": 1}},
		{"json_string", `{"b": 2}`, map[string]any{"b": float64(2)}},
		{"plain_string", "hello", map[string]any{"result": "hello"}},
		{"bytes", []byte(`{"c": 3}`), map[string]any{"c": float64(3)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMCPResult(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("got %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// mockMCPClient implements MCPClient for testing.
type mockMCPClient struct {
	resp     any
	lastTool string
	lastArgs map[string]any
}

func (m *mockMCPClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	m.lastTool = name
	m.lastArgs = args
	return m.resp, nil
}
