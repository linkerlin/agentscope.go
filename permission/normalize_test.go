package permission

import (
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestNormalizeCommand(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"rm -rf /", "rm -rf /"},
		{"sudo rm -rf /", "rm -rf /"},
		{"doas rm -rf /", "rm -rf /"},
		{"sudo -u root rm -rf /", "rm -rf /"},
		{"sudo -n rm -rf /", "rm -rf /"},
		{"sudo -- rm -rf /", "rm -rf /"},
		{"sudo -u root -- rm -rf /", "rm -rf /"},
		{"sudo -n -u alice -- rm -rf /", "rm -rf /"},
		{`sh -c "rm -rf /"`, "rm -rf /"},
		{"bash -c 'rm -rf /'", "rm -rf /"},
		{`sudo sh -c "rm -rf /"`, "rm -rf /"},
		{`sh -c 'sudo rm -rf /'`, "rm -rf /"},
		{`eval "rm -rf /"`, "rm -rf /"},
		{`env -S "rm -rf /"`, "rm -rf /"},
		{`env FOO=bar rm -rf /`, "rm -rf /"},
		{`xargs -n1 rm -rf /`, "rm -rf /"},
		{`nohup rm -rf /`, "rm -rf /"},
		{`nice -n 10 rm -rf /`, "rm -rf /"},
		{`time rm -rf /`, "rm -rf /"},
		{`command rm -rf /`, "rm -rf /"},
		{`rm\ -rf\ /`, "rm -rf /"},
		{"ls -la", "ls -la"},
		{"echo 'rm -rf /'", "echo rm -rf /"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeCommand(tc.in); got != tc.want {
			t.Errorf("NormalizeCommand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeCommand_DepthCapped(t *testing.T) {
	cmd := "sudo "
	for i := 0; i < 20; i++ {
		cmd += "sudo "
	}
	cmd += "rm -rf /"
	got := NormalizeCommand(cmd)
	// The depth cap terminates expansion; the inner command must survive.
	if !strings.Contains(got, "rm -rf /") {
		t.Fatalf("inner command should survive depth-capped unwrapping, got %q", got)
	}
}

func TestNormalizeCommand_NoUnwrapForNonWrapper(t *testing.T) {
	// A command that merely mentions a wrapper word must not be rewritten.
	if got := NormalizeCommand(`echo "sudo rm"`); got != "echo sudo rm" {
		t.Fatalf("expected quote-strip only, got %q", got)
	}
}

func TestSafetyCheck_SeesThroughWrappers(t *testing.T) {
	// checkSafety is bypass-immune: wrapped dangerous commands must yield ASK.
	e := NewEngine(ModeBypass, nil)
	commands := []string{
		"rm -rf /",
		"sudo rm -rf /",
		"sudo -u root -- rm -rf /",
		"sudo -n rm -rf /",
		`sudo sh -c "rm -rf /"`,
		`sh -c "rm -rf /"`,
		`eval "rm -rf /"`,
		`env -S "rm -rf /"`,
		`sudo sh -c "rm -rf /"`,
	}
	for _, cmd := range commands {
		evals, err := e.Evaluate([]*message.ToolUseBlock{{
			Name:  "execute_shell_command",
			Input: map[string]any{"command": cmd},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if evals[0].Decision != DecisionAsk {
			t.Errorf("wrapped dangerous command %q: expected ask, got %s", cmd, evals[0].Decision)
		}
	}
}
