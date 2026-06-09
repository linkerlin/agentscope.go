package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// MCPSidecar runs an in-process MCPGateway reachable from the host.
// Alias for E2BMCPSidecar — used by local dev, Docker, and E2B bootstrap flows.
type MCPSidecar = E2BMCPSidecar

// MCPGatewaySidecarConfig configures a host-side MCP gateway sidecar.
type MCPGatewaySidecarConfig = E2BMCPConfig

// StartMCPGatewaySidecar starts a host-side MCP gateway for workspace sandboxes.
func StartMCPGatewaySidecar(ctx context.Context, cfg MCPGatewaySidecarConfig, register func(*MCPGateway)) (*MCPSidecar, error) {
	return StartE2BMCPGateway(ctx, cfg, register)
}

// LoadMCPGatewayConfigFile reads a PyV2-style gateway bootstrap document.
func LoadMCPGatewayConfigFile(path string) (MCPGatewayConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MCPGatewayConfig{}, err
	}
	var cfg MCPGatewayConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return MCPGatewayConfig{}, fmt.Errorf("parse mcp gateway config: %w", err)
	}
	return cfg, nil
}
