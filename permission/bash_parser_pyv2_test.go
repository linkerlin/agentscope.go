package permission

import "testing"

// Cases aligned with PyV2 tree-sitter bash parser expectations (heuristic implementation).
func TestSplitCompoundCommand_PyV2Aligned(t *testing.T) {
	tests := []struct {
		cmd   string
		parts int
	}{
		{"cmd1 && cmd2 || cmd3", 3},
		{"a | b | c", 3},
		{"echo 'a; b' && ls", 2},
		{"(echo hi)", 1},
		{"export FOO=bar && ls", 2},
	}
	for _, tc := range tests {
		got := SplitCompoundCommand(tc.cmd)
		if len(got) != tc.parts {
			t.Fatalf("SplitCompoundCommand(%q) = %v, want %d parts", tc.cmd, got, tc.parts)
		}
	}
}

func TestAnyDangerousCommand_NestedPipeline(t *testing.T) {
	if !AnyDangerousCommand("ls | xargs rm -rf /") {
		t.Fatal("expected dangerous command in pipeline segment")
	}
}

func TestIsReadOnlyCommand_GitStatus(t *testing.T) {
	if !IsReadOnlyCommandSingle("git status --short") {
		t.Fatal("git status should be read-only")
	}
	if IsReadOnlyCommand("git push origin main") {
		t.Fatal("git push should not be read-only")
	}
}

func TestHasOutputRedirection_HeredocSafe(t *testing.T) {
	if HasOutputRedirection("cat <<EOF\nhello\nEOF") {
		t.Fatal("heredoc should not count as stdout redirect")
	}
	if !HasOutputRedirection("echo hi > out.txt") {
		t.Fatal("expected redirect detection")
	}
}
