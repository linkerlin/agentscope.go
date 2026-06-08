package permission

import "testing"

func TestSplitCompoundCommand(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"ls -la && rm -rf /tmp/x | grep foo", 3},
		{"echo ok; pwd", 2},
		{"false || echo fallback", 2},
		{"echo \"a && b\"", 1},
		{"echo 'a | b'", 1},
		{"echo $(rm -rf /)", 1},
	}
	for _, tc := range tests {
		parts := SplitCompoundCommand(tc.in)
		if len(parts) != tc.want {
			t.Fatalf("SplitCompoundCommand(%q) = %v, want %d parts", tc.in, parts, tc.want)
		}
	}
}

func TestAnyDangerousCommand_Subshell(t *testing.T) {
	if !AnyDangerousCommand("echo ok && rm -rf /") {
		t.Fatal("expected dangerous compound")
	}
	if !AnyDangerousCommand("echo $(rm -rf /)") {
		t.Fatal("expected dangerous subshell")
	}
	if AnyDangerousCommand("ls -la && cat welcome.txt") {
		t.Fatal("expected safe compound")
	}
}

func TestAllReadOnlyCommands_Redirect(t *testing.T) {
	if AllReadOnlyCommands("ls > out.txt") {
		t.Fatal("redirect should not be read-only")
	}
	if !AllReadOnlyCommands("ls -la && cat file.txt") {
		t.Fatal("expected read-only compound")
	}
}

func TestExtractCommandPrefix(t *testing.T) {
	if got := ExtractCommandPrefix("NODE_ENV=prod npm run build"); got != "npm run" {
		t.Fatalf("prefix=%q", got)
	}
	if got := ExtractCommandPrefix("git status --short"); got != "git status" {
		t.Fatalf("prefix=%q", got)
	}
}

func TestExtractFilePaths(t *testing.T) {
	paths := ExtractFilePaths("rm -rf /tmp/foo bar")
	if len(paths) != 2 || paths[0] != "/tmp/foo" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestExtractSubshellBodies(t *testing.T) {
	bodies := ExtractSubshellBodies("echo $(pwd) and `whoami`")
	if len(bodies) != 2 {
		t.Fatalf("bodies=%v", bodies)
	}
}

func TestShellTokenize(t *testing.T) {
	toks := ShellTokenize(`echo "hello world"`)
	if len(toks) != 2 || toks[1] != "hello world" {
		t.Fatalf("tokens=%v", toks)
	}
}
