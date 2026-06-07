package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/dashscope"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
	"github.com/linkerlin/agentscope.go/workspace"
)

// buildAgent creates the ReAct agent with workspace tools and permission engine.
func buildAgent(wsDir string) (agent.Agent, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DASHSCOPE_API_KEY is required")
	}

	chatModel, err := dashscope.Builder().
		APIKey(apiKey).
		ModelName(envOr("DASHSCOPE_MODEL", "qwen3.7-plus")).
		BaseURL(envOr("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")).
		Build()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return nil, err
	}
	_ = os.WriteFile(filepath.Join(wsDir, "welcome.txt"), []byte("Hello from multi-tenant workspace example.\n"), 0o644)

	ws := workspace.NewLocalWorkspace("default", wsDir)
	permEngine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Target: "tool_name", Pattern: "read_file", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "list_directory", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
	})

	tools := []tool.Tool{
		file.NewReadFileTool(wsDir),
		file.NewListDirectoryTool(wsDir),
		file.NewWriteFileTool(wsDir),
		shell.NewShellCommandTool(wsDir, []string{"ls", "cat", "pwd", "echo"}, nil),
	}

	return react.Builder().
		Name("MultiTenantAgent").
		SysPrompt("You are a helpful assistant with access to a local workspace. You can read files, list directories, write files (requires user confirmation), and run safe shell commands. Be concise.").
		Model(chatModel).
		Workspace(ws).
		PermissionEngine(permEngine).
		Tools(tools...).
		Build()
}

// buildGateway wires storage, auth, session manager, and HTTP routes.
func buildGateway(ag agent.Agent) *gateway.Server {
	storage := service.NewMemoryStorage()
	cipher, _ := service.NewCipherFromEnv()

	jwtSecret := envOr("JWT_SECRET", "dev-jwt-secret-change-me")
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	combinedAuth := service.NewAnyAuthenticator(apiAuth, jwtAuth)

	sessionMgr := gateway.NewSessionManager().WithStorage(storage)

	srv := gateway.NewServer(ag).
		WithStorage(storage).
		WithCipher(cipher).
		WithSessionManager(sessionMgr).
		WithAuthenticator(combinedAuth)

	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterServiceRoutes()
	srv.RegisterV2Routes()
	return srv
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func runServer(addr string) (http.Handler, error) {
	wsRoot := envOr("WORKSPACE_ROOT", "./workspace_data")
	ag, err := buildAgent(wsRoot)
	if err != nil {
		return nil, err
	}

	srv := buildGateway(ag)
	handler := gateway.CORSMiddleware(srv)

	log.Printf("Multi-tenant workspace example on http://localhost%s", addr)
	log.Println("Endpoints:")
	log.Println("  POST /api/v1/auth/register   -> register tenant (returns api_key)")
	log.Println("  POST /api/v1/auth/login      -> login (returns JWT)")
	log.Println("  GET  /api/v1/me              -> current user (X-API-Key or Bearer)")
	log.Println("  POST /api/v1/sessions        -> create session")
	log.Println("  POST /api/v1/credentials     -> store encrypted credential")
	log.Println("  POST /v2/chat/stream         -> SSE (session_id for persistence)")
	log.Println("  POST /v2/resume              -> resume HITL suspended session")
	log.Printf("Workspace root: %s", wsRoot)
	log.Printf("Model: %s", envOr("DASHSCOPE_MODEL", "qwen3.7-plus"))

	return handler, nil
}
