package react

import (
	"path/filepath"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/state"
)

func TestReActAgentStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewJSONStore(filepath.Join(dir, "st"))
	if err != nil {
		t.Fatal(err)
	}
	a := &ReActAgent{
		Base: agent.NewBase("aid", "n", "d", "sys", map[string]any{"k": 1}, nil, nil),
		maxIterations: 7,
		memory:        memory.NewInMemoryMemory(),
	}
	if err := a.SaveTo(store, "run1"); err != nil {
		t.Fatal(err)
	}
	b := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	ok, err := b.LoadIfExists(store, "run1")
	if err != nil || !ok {
		t.Fatalf("load err=%v ok=%v", err, ok)
	}
	if b.Base.Name != "n" || b.Base.SysPrompt != "sys" || b.maxIterations != 7 || b.Base.ID != "aid" {
		t.Fatalf("fields %+v %+v %d %+v", b.Base.Name, b.Base.SysPrompt, b.maxIterations, b.Base.ID)
	}
	// JSON 数字默认解码为 float64
	if v, ok := b.Base.Meta["k"].(float64); !ok || v != 1 {
		t.Fatalf("meta %#v", b.Base.Meta)
	}
}


func TestAgentState_StateType(t *testing.T) {
	st := AgentState{}
	if st.StateType() != "agent_state" {
		t.Fatal("unexpected state type")
	}
}

func TestReActAgent_SaveTo_NilStore(t *testing.T) {
	a := &ReActAgent{Base: agent.NewBase("", "n", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	if err := a.SaveTo(nil, "k"); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestReActAgent_LoadFrom_NilStore(t *testing.T) {
	a := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	if err := a.LoadFrom(nil, "k"); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestReActAgent_LoadIfExists_NilStore(t *testing.T) {
	a := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	ok, err := a.LoadIfExists(nil, "k")
	if err == nil || ok {
		t.Fatal("expected error and false for nil store")
	}
}

func TestReActAgent_LoadIfExists_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := state.NewJSONStore(filepath.Join(dir, "st"))
	a := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	ok, err := a.LoadIfExists(store, "missing")
	if err != nil || ok {
		t.Fatalf("expected false and no error, got ok=%v err=%v", ok, err)
	}
}

func TestReActAgent_SaveTo_FallbackID(t *testing.T) {
	dir := t.TempDir()
	store, _ := state.NewJSONStore(filepath.Join(dir, "st"))
	a := &ReActAgent{Base: agent.NewBase("", "nameID", "", "", nil, nil, nil), maxIterations: 3, memory: memory.NewInMemoryMemory()}
	if err := a.SaveTo(store, "k"); err != nil {
		t.Fatal(err)
	}
	b := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	if err := b.LoadFrom(store, "k"); err != nil {
		t.Fatal(err)
	}
	if b.Base.ID != "nameID" {
		t.Fatalf("expected ID fallback to name, got %s", b.Base.ID)
	}
}

func TestReActAgent_metadata_Empty(t *testing.T) {
	a := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), memory: memory.NewInMemoryMemory()}
	if a.metadata() != nil {
		t.Fatalf("expected nil metadata, got %v", a.metadata())
	}
}

func TestReActAgent_applyAgentState_ZeroValues(t *testing.T) {
	a := &ReActAgent{Base: agent.NewBase("", "", "", "", nil, nil, nil), maxIterations: 5, memory: memory.NewInMemoryMemory()}
	a.applyAgentState(AgentState{AgentID: "id", Name: "n", SystemPrompt: "sys"})
	if a.maxIterations != 5 {
		t.Fatalf("expected maxIterations unchanged when zero, got %d", a.maxIterations)
	}
	if a.Base.Meta != nil {
		t.Fatalf("expected meta nil, got %v", a.Base.Meta)
	}
}
