// Package snippets holds the documentation examples from docs/*.md as
// compilable Go code (23.5 doc contract): the compiler verifies that every
// API the docs reference actually exists with the documented shape. Edit
// the docs and this file together — scripts/check_docs.sh guards the
// mirrored copies and constructor references.
package snippets

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/workspace"
)

// --- docs/A2A.md ---

// A2AServerDoc mirrors the "启动 A2A Server" snippet.
func A2AServerDoc(model any) error {
	var ag agent.Agent = stubAgent{agentName: "coder"}

	card := a2a.AgentCard{
		Name:         "coder",
		Description:  "A coding assistant agent",
		URL:          "http://localhost:9000",
		Version:      "1.0.0",
		Capabilities: []string{"streaming"},
	}

	server := a2a.NewServer(card, a2a.NewAgentAdapter(ag), nil) // nil store → in-memory
	_ = model
	_ = server
	return nil
}

// A2AClientDoc mirrors the "发送任务" snippets.
func A2AClientDoc(ctx context.Context) error {
	client := a2a.NewHTTPClient("http://localhost:9000")
	reply, err := client.Send(ctx, &a2a.Message{
		Role:    "user",
		Content: "Write a Go function that reverses a string.",
	})
	if err != nil {
		return err
	}

	ch, err := client.SendSubscribe(ctx, &a2a.Message{
		Role:    "user",
		Content: "Write a Go function that reverses a string.",
	})
	if err != nil {
		return err
	}
	for msg := range ch {
		fmt.Println("chunk:", msg.Content)
	}
	_ = reply
	return client.Close()
}

// A2ARegistryDoc mirrors the "Registry 动态发现" snippet.
func A2ARegistryDoc(ctx context.Context) {
	registry := a2a.NewRegistry()
	_ = registry.Register(a2a.AgentCard{
		Name: "peer", URL: "http://peer:9000", Version: "1.0.0",
	})
	stop := registry.StartBackgroundHealthCheck(ctx, 30*time.Second)
	defer stop()

	for _, entry := range registry.List() {
		if strings.Contains(entry.Card.Description, "coding") {
			fmt.Println(entry.Card.URL)
		}
	}
}

// A2AWebSocketDoc mirrors the "WebSocket 实时推送" snippet.
func A2AWebSocketDoc() error {
	card := a2a.AgentCard{Name: "coder", URL: "http://localhost:9000", Version: "1.0.0"}
	a2aServer := a2a.NewServer(card, a2a.NewAgentAdapter(stubAgent{}), nil)
	wsServer := a2a.NewWebSocketServer(a2aServer)

	mux := http.NewServeMux()
	mux.Handle("/a2a/", a2aServer)
	mux.HandleFunc("/a2a/ws", wsServer.HandleWebSocket)
	srv := docHTTPServer(":9000", mux)
	return srv.Close()
}

// A2AAuthDoc mirrors the "认证中间件" + "SecureServer" snippets.
func A2AAuthDoc() error {
	auth := a2a.NewAuthMiddleware()
	auth.AddAPIKey(os.Getenv("A2A_API_KEY"), "ops")
	auth.SetJWTSecret(os.Getenv("A2A_JWT_SECRET"))
	auth.AddPublicPath("/.well-known/agent.json")

	card := a2a.AgentCard{Name: "coder", URL: "http://localhost:9000", Version: "1.0.0"}
	secure := a2a.NewSecureServer(card, a2a.NewAgentAdapter(stubAgent{}), nil).
		WithAuth(auth)

	srv := docHTTPServer(":9001", secure)
	return srv.Close()
}

// docHTTPServer returns an http.Server with the production timeouts the
// deployment docs recommend (never ship a zero-timeout server).
func docHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

type stubAgent struct{ agentName string }

// Name implements agent.Agent.
func (s stubAgent) Name() string { return s.agentName }

// Call implements agent.Agent with a canned reply.
func (s stubAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

// CallStream implements agent.Agent with a one-shot stream.
func (s stubAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

// --- docs/WORKSPACE.md ---

// WorkspaceBackendsDoc mirrors the "7 个后端" constructor table.
func WorkspaceBackendsDoc(ctx context.Context) error {
	local := workspace.NewLocalWorkspace("demo", "./sandbox")
	defer func() { _ = local.Close() }()

	k8s, err := workspace.NewK8sWorkspace(ctx, workspace.K8sConfig{})
	if err != nil {
		log.Printf("k8s unavailable: %v", err)
	} else {
		defer func() { _ = k8s.Close() }()
	}

	bw, err := workspace.NewBubblewrapWorkspace(workspace.BubblewrapConfig{})
	if err != nil {
		log.Printf("bubblewrap unavailable: %v", err)
	} else {
		defer func() { _ = bw.Close() }()
	}

	daytona, err := workspace.NewDaytonaWorkspace(ctx, workspace.DaytonaConfig{})
	if err != nil {
		log.Printf("daytona unavailable: %v", err)
	} else {
		defer func() { _ = daytona.Close() }()
	}

	sb, err := workspace.NewOpenSandboxWorkspace(ctx, workspace.OpenSandboxConfig{})
	if err != nil {
		log.Printf("opensandbox unavailable: %v", err)
	} else {
		defer func() { _ = sb.Close() }()
	}

	// Docker/E2B constructors take runtime handles; existence-checked by
	// scripts/check_docs.sh.
	_ = workspace.NewDockerWorkspace("demo", "container-id")
	_ = workspace.NewE2BWorkspace("demo", "sandbox-id", nil)
	_ = workspace.NewK8sWorkspaceForExistingPod("demo", "default", "pod")
	return nil
}
