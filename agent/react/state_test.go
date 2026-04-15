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
