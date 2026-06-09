package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMCPGatewayConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gw.json")
	if err := os.WriteFile(path, []byte(`{"token":"t1","servers":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadMCPGatewayConfigFile(path)
	if err != nil || cfg.Token != "t1" {
		t.Fatalf("cfg: err=%v token=%q", err, cfg.Token)
	}
}

func TestStartMCPGatewaySidecar_Alias(t *testing.T) {
	ctx := context.Background()
	sidecar, err := StartMCPGatewaySidecar(ctx, MCPGatewaySidecarConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sidecar.Close(ctx)
	if sidecar.HostURL == "" {
		t.Fatal("expected host URL")
	}
}
