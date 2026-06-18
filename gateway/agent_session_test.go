package gateway

import (
	"testing"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/workspace"
)

// TestBuildSessionAgent_WiresWorkspaceIntoPermission verifies Tier 1A: the
// session workspace root is injected into the permission engine's WorkingDirs
// so ACCEPT_EDITS mode can auto-allow edits inside the session workspace
// (aligns with Python agentscope #1823).
func TestBuildSessionAgent_WiresWorkspaceIntoPermission(t *testing.T) {
	f := NewAgentFactory(nil)
	f.RegisterProvider("mock", func(key, name, url string) (model.ChatModel, error) {
		return &mockChatModel{name: name}, nil
	})

	dir := t.TempDir()
	sw := NewSessionWorkspace(workspace.NewLocalWorkspace("sess", dir), dir)
	cfg := &service.AgentConfig{ID: "a", Name: "a", ModelID: "mock/m"}
	cred := &service.Credential{Provider: "mock", Encrypted: "k"}

	leader, err := f.BuildSessionAgent(cfg, cred, sw, SessionAgentDeps{PermissionMode: permission.ModeAcceptEdits})
	if err != nil {
		t.Fatalf("BuildSessionAgent: %v", err)
	}
	re, ok := leader.(*react.ReActAgent)
	if !ok {
		t.Fatalf("expected *react.ReActAgent, got %T", leader)
	}
	pe := re.PermissionEngine()
	if pe == nil {
		t.Fatal("expected non-nil permission engine")
	}
	if pe.Mode() != permission.ModeAcceptEdits {
		t.Fatalf("expected accept_edits mode, got %q", pe.Mode())
	}
	found := false
	for _, wd := range pe.WorkingDirs() {
		if wd == dir {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected WorkingDirs to contain session dir %q, got %v", dir, pe.WorkingDirs())
	}
}

// TestBuildSessionAgent_DefaultPermissionMode verifies the default mode is
// Explore when deps.PermissionMode is empty, and the engine is still wired
// with the workspace root.
func TestBuildSessionAgent_DefaultPermissionMode(t *testing.T) {
	f := NewAgentFactory(nil)
	f.RegisterProvider("mock", func(key, name, url string) (model.ChatModel, error) {
		return &mockChatModel{name: name}, nil
	})
	dir := t.TempDir()
	sw := NewSessionWorkspace(workspace.NewLocalWorkspace("sess", dir), dir)
	cfg := &service.AgentConfig{ID: "a", Name: "a", ModelID: "mock/m"}
	cred := &service.Credential{Provider: "mock", Encrypted: "k"}

	leader, err := f.BuildSessionAgent(cfg, cred, sw, SessionAgentDeps{}) // empty mode
	if err != nil {
		t.Fatal(err)
	}
	pe := leader.(*react.ReActAgent).PermissionEngine()
	if pe.Mode() != permission.ModeExplore {
		t.Fatalf("expected default explore mode, got %q", pe.Mode())
	}
	if len(pe.WorkingDirs()) != 1 || pe.WorkingDirs()[0] != dir {
		t.Fatalf("expected workspace dir wired, got %v", pe.WorkingDirs())
	}
}
