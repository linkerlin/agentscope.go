package permission

import "testing"

func TestSplitCompoundCommand(t *testing.T) {
	parts := SplitCompoundCommand("ls -la && rm -rf /tmp/x | grep foo")
	if len(parts) < 2 {
		t.Fatalf("expected split parts, got %v", parts)
	}
	if !AnyDangerousCommand("echo ok && rm -rf /") {
		t.Fatal("expected dangerous compound command")
	}
}
