package permission

import "testing"

// TestEngineAccessors covers the Mode() and WorkingDirs() accessors added for
// Tier 1A wiring introspection.
func TestEngineAccessors(t *testing.T) {
	e := NewEngineWithContext(NewContext(ModeAcceptEdits).WithWorkingDirs("/a", "/b"), nil)
	if e.Mode() != ModeAcceptEdits {
		t.Fatalf("expected accept_edits mode, got %q", e.Mode())
	}
	wd := e.WorkingDirs()
	if len(wd) != 2 || wd[0] != "/a" || wd[1] != "/b" {
		t.Fatalf("unexpected working dirs: %v", wd)
	}
	// Returned slice is a copy — mutating it must not affect the engine.
	wd[0] = "/changed"
	if e.WorkingDirs()[0] != "/a" {
		t.Fatal("WorkingDirs() should return a defensive copy")
	}

	// NewEngine (no explicit context) still reports its mode and empty dirs.
	e2 := NewEngine(ModeExplore, nil)
	if e2.Mode() != ModeExplore {
		t.Fatalf("expected explore mode, got %q", e2.Mode())
	}
	if len(e2.WorkingDirs()) != 0 {
		t.Fatalf("expected empty working dirs, got %v", e2.WorkingDirs())
	}

	// Nil-safety.
	var ne *Engine
	if ne.Mode() != "" || ne.WorkingDirs() != nil {
		t.Fatal("nil engine accessors should return zero values")
	}
}
