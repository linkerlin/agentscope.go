package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestServerSpec_IsEnabledDefault(t *testing.T) {
	s := ServerSpec{Name: "x"}
	if !s.IsEnabled() {
		t.Fatal("default should be enabled")
	}
	f := false
	s2 := ServerSpec{Name: "x", Enabled: &f}
	if s2.IsEnabled() {
		t.Fatal("explicitly disabled should be false")
	}
}

func TestServerSpec_ExpandEnv(t *testing.T) {
	t.Setenv("MY_TOKEN", "secret123")
	t.Setenv("MY_PATH", "/data")
	s := ServerSpec{
		Name: "github", Transport: "stdio",
		Command: "$NOPE_CMD", // unset -> ""
		Args:    []string{"-y", "server", "$MY_PATH"},
		Env:     map[string]string{"TOKEN": "$MY_TOKEN"},
		URL:     "https://$MY_TOKEN@example.com",
	}
	s.ExpandEnv()
	if s.Args[2] != "/data" {
		t.Fatalf("arg not expanded: %q", s.Args[2])
	}
	if s.Env["TOKEN"] != "secret123" {
		t.Fatalf("env not expanded: %q", s.Env["TOKEN"])
	}
	if s.URL != "https://secret123@example.com" {
		t.Fatalf("url not expanded: %q", s.URL)
	}
}

func TestServerSpec_BuildClientStdioMissingCommand(t *testing.T) {
	s := ServerSpec{Name: "x", Transport: "stdio"}
	if _, err := s.BuildClient(); err == nil {
		t.Fatal("expected error for stdio without command")
	}
}

func TestServerSpec_BuildClientUnknownTransport(t *testing.T) {
	s := ServerSpec{Name: "x", Transport: "ftp"}
	if _, err := s.BuildClient(); err == nil {
		t.Fatal("expected error for unknown transport")
	}
}

func TestConnectServers_DisabledSkipped(t *testing.T) {
	f := false
	specs := []ServerSpec{{Name: "off", Transport: "stdio", Command: "echo", Enabled: &f}}
	mgr, results := ConnectServers(context.Background(), specs)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil || results[0].Err.Error() != "disabled" {
		t.Fatalf("expected disabled error, got %v", results[0].Err)
	}
}

func TestConnectServers_MissingBinarySkipped(t *testing.T) {
	// A stdio server whose binary does not exist should be skipped, not fatal.
	specs := []ServerSpec{{Name: "ghost", Transport: "stdio", Command: "this-binary-does-not-exist-12345"}}
	mgr, results := ConnectServers(context.Background(), specs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error for missing binary")
	}
	// manager should still be usable (no registered client)
	tools, err := mgr.Tools(context.Background())
	if err != nil {
		t.Fatalf("manager.Tools should not error: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestLoadSpecsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.yaml")
	content := `servers:
  - name: filesystem
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  - name: disabled-one
    transport: stdio
    command: echo
    enabled: false
`
	os.WriteFile(path, []byte(content), 0o644)
	specs, err := LoadSpecsFromYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].Name != "filesystem" || specs[0].Command != "npx" {
		t.Fatalf("first spec wrong: %+v", specs[0])
	}
	if specs[0].Args[1] != "@modelcontextprotocol/server-filesystem" {
		t.Fatalf("args wrong: %v", specs[0].Args)
	}
	if specs[1].IsEnabled() {
		t.Fatal("second spec should be disabled")
	}
}

func TestLoadSpecsFromYAML_MissingFile(t *testing.T) {
	if _, err := LoadSpecsFromYAML("/no/such/file.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCommonServers_HasExpected(t *testing.T) {
	for _, key := range []string{"filesystem", "fetch", "playwright", "github"} {
		if _, ok := CommonServers[key]; !ok {
			t.Fatalf("CommonServers missing %q", key)
		}
	}
	if CommonServers["filesystem"].Command != "npx" {
		t.Fatal("filesystem command should be npx")
	}
}

func TestCloseManager_NilSafe(t *testing.T) {
	CloseManager(nil) // must not panic
}
