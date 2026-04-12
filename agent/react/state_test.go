package react

import (
	"path/filepath"
	"testing"

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
		agentID:       "aid",
		name:          "n",
		description:   "d",
		sysPrompt:     "sys",
		maxIterations: 7,
		memory:        memory.NewInMemoryMemory(),
		meta:          map[string]any{"k": 1},
	}
	if err := a.SaveTo(store, "run1"); err != nil {
		t.Fatal(err)
	}
	b := &ReActAgent{memory: memory.NewInMemoryMemory()}
	ok, err := b.LoadIfExists(store, "run1")
	if err != nil || !ok {
		t.Fatalf("load err=%v ok=%v", err, ok)
	}
	if b.name != "n" || b.sysPrompt != "sys" || b.maxIterations != 7 || b.agentID != "aid" {
		t.Fatalf("fields %+v %+v %d %+v", b.name, b.sysPrompt, b.maxIterations, b.agentID)
	}
	// JSON 数字默认解码为 float64
	if v, ok := b.meta["k"].(float64); !ok || v != 1 {
		t.Fatalf("meta %#v", b.meta)
	}
}
