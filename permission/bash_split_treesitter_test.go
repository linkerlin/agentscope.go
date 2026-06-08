//go:build treesitter

package permission

import "testing"

func TestSplitCompoundCommand_TreeSitterBackend(t *testing.T) {
	if ParserBackend() != "treesitter" {
		t.Fatalf("backend=%q", ParserBackend())
	}
	parts := SplitCompoundCommand("ls -la && rm -rf /tmp/x | grep foo")
	if len(parts) != 3 {
		t.Fatalf("parts=%v", parts)
	}
	if parts[0] != "ls -la" || parts[1] != "rm -rf /tmp/x" || parts[2] != "grep foo" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestSplitCompoundCommand_TreeSitterFallback(t *testing.T) {
	// Grouped subshell stays one segment (tree-sitter command node).
	parts := SplitCompoundCommand("(echo hi && ls)")
	if len(parts) != 1 {
		t.Fatalf("expected one grouped segment, got %v", parts)
	}
}
