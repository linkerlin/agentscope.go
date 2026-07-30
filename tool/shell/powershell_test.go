package shell

import (
	"context"
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
