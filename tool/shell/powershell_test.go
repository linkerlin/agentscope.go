package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEncodePowerShellCommand(t *testing.T) {
	encoded := encodePowerShellCommand("Write-Output 'hello'")
	if encoded == "" {
		t.Fatal("encoded command should not be empty")
	}
	// Base64 should not contain raw PowerShell text
	if strings.Contains(encoded, "Write-Output") {
		t.Fatal("encoded command should not contain plaintext")
	}
}

func TestPowerShellTool_Name(t *testing.T) {
	tool := NewPowerShellTool()
	if tool.Name() != "PowerShell" {
		t.Fatalf("expected 'PowerShell', got %s", tool.Name())
	}
}

func TestPowerShellTool_Spec(t *testing.T) {
	tool := NewPowerShellTool()
	spec := tool.Spec()
	if spec.Name != "PowerShell" {
		t.Fatalf("expected spec name 'PowerShell', got %s", spec.Name)
	}
	params, ok := spec.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("spec should have properties")
	}
	if _, ok := params["command"]; !ok {
		t.Fatal("spec should have command property")
	}
}

func TestPowerShellTool_EmptyCommand(t *testing.T) {
	tool := NewPowerShellTool()
	_, err := tool.Execute(context.Background(), map[string]any{"command": ""})
	if err == nil {
		t.Fatal("empty command should return error")
	}
}

func TestPowerShellTool_TruncOutput(t *testing.T) {
	long := strings.Repeat("a", psMaxOutputLen+100)
	out := truncOutput(long)
	if len(out) > psMaxOutputLen+100 {
		t.Fatalf("output should be truncated, got len=%d", len(out))
	}
	if !strings.Contains(out, "[output truncated]") {
		t.Fatal("truncated output should have marker")
	}
}

func TestPowerShellTool_WithBaseDir(t *testing.T) {
	tool := NewPowerShellTool().WithBaseDir("/tmp")
	if tool.BaseDir != "/tmp" {
		t.Fatalf("expected BaseDir '/tmp', got %s", tool.BaseDir)
	}
}

func TestPowerShellTool_WithTimeout(t *testing.T) {
	tool := NewPowerShellTool().WithTimeout(psMaxTimeout + 1000)
	if tool.DefaultTimeout > psMaxTimeout {
		t.Fatalf("timeout should be capped at psMaxTimeout, got %v", tool.DefaultTimeout)
	}
}

func TestPowerShellTool_IsReadOnly(t *testing.T) {
	tool := NewPowerShellTool()
	if tool.IsReadOnly() {
		t.Fatal("PowerShell should not be read-only")
	}
}

// fakeBackend records invocations and returns scripted output.
type fakeBackend struct {
	lastName string
	lastArgs []string
	lastDir  string
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
}

func (f *fakeBackend) Run(ctx context.Context, name string, args []string, dir string) ([]byte, []byte, int, error) {
	f.lastName = name
	f.lastArgs = append([]string(nil), args...)
	f.lastDir = dir
	return f.stdout, f.stderr, f.exitCode, f.err
}

func TestPowerShellTool_BackendExecution(t *testing.T) {
	fb := &fakeBackend{stdout: []byte("hi"), exitCode: 0}
	tool := NewPowerShellTool().WithBaseDir("/tmp").WithBackend(fb)
	resp, err := tool.Execute(context.Background(), map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "<returncode>0</returncode>") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	if fb.lastDir != "/tmp" {
		t.Fatalf("expected dir /tmp, got %q", fb.lastDir)
	}
	found := false
	for i, a := range fb.lastArgs {
		if a == "-EncodedCommand" && i+1 < len(fb.lastArgs) && fb.lastArgs[i+1] != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected -EncodedCommand argument, got %v", fb.lastArgs)
	}
}

func TestPowerShellTool_BackendError(t *testing.T) {
	fb := &fakeBackend{stderr: []byte("boom"), exitCode: 1, err: errors.New("exit status 1")}
	tool := NewPowerShellTool().WithBackend(fb)
	resp, err := tool.Execute(context.Background(), map[string]any{"command": "bad"})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "<returncode>1</returncode>") || !strings.Contains(text, "boom") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func TestPowerShellTool_DefaultBackend(t *testing.T) {
	tool := NewPowerShellTool()
	if _, ok := tool.backend().(LocalBackend); !ok {
		t.Fatalf("expected LocalBackend default, got %T", tool.backend())
	}
}
