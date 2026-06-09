package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/workspace"
)

// SessionMCPGatewayPool manages one MCP gateway sidecar per session.
type SessionMCPGatewayPool struct {
	mu        sync.Mutex
	token     string
	gateways  map[string]*workspace.MCPSidecar
}

// NewSessionMCPGatewayPool creates an empty per-session gateway pool.
func NewSessionMCPGatewayPool(token string) *SessionMCPGatewayPool {
	return &SessionMCPGatewayPool{
		token:    token,
		gateways: make(map[string]*workspace.MCPSidecar),
	}
}

// Ensure starts (or returns) the MCP gateway for sessionID.
func (p *SessionMCPGatewayPool) Ensure(ctx context.Context, sessionID string) (url, token string, err error) {
	if p == nil || sessionID == "" {
		return "", "", fmt.Errorf("session mcp gateway: session id required")
	}
	p.mu.Lock()
	if sc, ok := p.gateways[sessionID]; ok && sc != nil {
		p.mu.Unlock()
		return sc.HostURL, p.token, nil
	}
	p.mu.Unlock()

	sc, err := workspace.StartMCPGatewaySidecar(ctx, workspace.MCPGatewaySidecarConfig{Token: p.token}, nil)
	if err != nil {
		return "", "", err
	}

	p.mu.Lock()
	if existing, ok := p.gateways[sessionID]; ok && existing != nil {
		p.mu.Unlock()
		_ = sc.Close(ctx)
		return existing.HostURL, p.token, nil
	}
	p.gateways[sessionID] = sc
	p.mu.Unlock()
	return sc.HostURL, p.token, nil
}

// Close shuts down one session gateway.
func (p *SessionMCPGatewayPool) Close(ctx context.Context, sessionID string) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	sc := p.gateways[sessionID]
	delete(p.gateways, sessionID)
	p.mu.Unlock()
	if sc != nil {
		return sc.Close(ctx)
	}
	return nil
}

// CloseAll shuts down every session gateway.
func (p *SessionMCPGatewayPool) CloseAll(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	all := p.gateways
	p.gateways = make(map[string]*workspace.MCPSidecar)
	p.mu.Unlock()
	var first error
	for _, sc := range all {
		if sc == nil {
			continue
		}
		if err := sc.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}
