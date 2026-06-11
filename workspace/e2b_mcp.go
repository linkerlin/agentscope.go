package workspace

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// E2BMCPConfig configures an MCP gateway sidecar for an E2B workspace.
type E2BMCPConfig struct {
	Token string
	Port  int // host port to bind; 0 picks a free port
}

// E2BMCPSidecar runs an in-process MCPGateway reachable from the host.
// In production the gateway may run inside the sandbox; this sidecar
// mirrors PyV2 DockerWorkspace/E2BWorkspace bootstrap for local dev.
type E2BMCPSidecar struct {
	Gateway    *MCPGateway
	HostURL    string
	httpServer *http.Server
}

// StartE2BMCPGateway starts a host-side MCP gateway for an E2B workspace.
func StartE2BMCPGateway(ctx context.Context, cfg E2BMCPConfig, register func(*MCPGateway)) (*E2BMCPSidecar, error) {
	port := cfg.Port
	if port == 0 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port = ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
	}

	gw := NewMCPGateway(cfg.Token)
	if register != nil {
		register(gw)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{Addr: addr, Handler: gw.Handler(), ReadHeaderTimeout: 10 * time.Second} // G112 fix: prevent Slowloris

	go func() {
		_ = srv.ListenAndServe()
	}()

	if err := waitGatewayHealth(ctx, "http://"+addr); err != nil {
		_ = srv.Close()
		return nil, err
	}

	return &E2BMCPSidecar{
		Gateway:    gw,
		HostURL:    "http://" + addr,
		httpServer: srv,
	}, nil
}

// Close shuts down the sidecar HTTP server.
func (s *E2BMCPSidecar) Close(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// NewGatewayClient returns a host-side client for this sidecar.
func (s *E2BMCPSidecar) NewGatewayClient(token string) *GatewayClient {
	return NewGatewayClient(s.HostURL, token)
}

func waitGatewayHealth(ctx context.Context, baseURL string) error {
	client := NewGatewayClient(baseURL, "")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Health(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return fmt.Errorf("gateway health check timed out: %s", baseURL)
}

// AttachMCPGateway records gateway metadata on an E2B workspace.
func (w *E2BWorkspace) AttachMCPGateway(hostURL, token string) *GatewayClient {
	return NewGatewayClient(hostURL, token)
}
