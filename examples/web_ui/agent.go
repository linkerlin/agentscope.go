package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/dashscope"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/toolkit"
	"github.com/linkerlin/agentscope.go/workspace"

	"github.com/redis/go-redis/v9"

	"github.com/linkerlin/agentscope.go/examples/shared/slowtool"
	"github.com/linkerlin/agentscope.go/skill"
)

func buildDashScopeAgent(apiKey, modelName, baseURL string, toolOffload *gateway.ToolOffloadManager) (agent.Agent, error) {
	chatModel, err := dashscope.Builder().
		APIKey(apiKey).
		ModelName(modelName).
		BaseURL(baseURL).
		Build()
	if err != nil {
		return nil, err
	}

	wsDir := envOr("WEB_UI_WORKSPACE", filepath.Join(os.TempDir(), "agentscope-web-ui"))
	_ = os.MkdirAll(wsDir, 0o755)
	ws := workspace.NewLocalWorkspace("web-ui", wsDir)

	tk := toolkit.NewToolkit()
	if err := tk.Register(slowtool.New(slowDemoDelayFromEnv())); err != nil {
		return nil, err
	}
	skillReg := skill.NewRegistry()
	skillReg.Register(&skill.AgentSkill{
		Name:         "web-ui-helper",
		Description:  "Tips for using the web UI demo.",
		SkillContent: "# Web UI Helper\n\n- Call slow_demo for long-running searches\n- Offloaded results may arrive in a follow-up turn\n",
		Source:       "demo",
	})
	skillReg.SetActive("web-ui-helper_demo", true)
	_, skillHook, err := skill.RegisterWithToolkit(tk, skillReg, skill.AttachOptions{})
	if err != nil {
		return nil, err
	}
	if toolOffload != nil {
		tk.Use(gateway.NewToolOffloadMiddleware(toolOffload, ""))
	}
	offloader := workspace.NewWorkspaceOffloader(ws, ".offload")
	tk.Use(toolkit.NewOffloadMiddlewareWithOffloader(offloader, "", ".offload"))

	perm := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Target: "tool_name", Pattern: "slow_demo", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "Skill", Decision: permission.DecisionAllow},
	})

	return react.Builder().
		Name("WebUIAgent").
		SysPrompt("You are a helpful assistant. You may call slow_demo for long-running searches; results can arrive in a follow-up turn after background offload. Use the Skill tool to browse available skills. Be concise.").
		Model(chatModel).
		Workspace(ws).
		PermissionEngine(perm).
		Hooks(skillHook).
		Toolkit(tk).
		Build()
}

func buildGateway(ag agent.Agent, toolOffload *gateway.ToolOffloadManager) *gateway.Server {
	srv := gateway.NewServer(ag).
		WithSessionManager(gateway.NewSessionManager()).
		WithToolOffloadManager(toolOffload)

	// Production-style assembly (18.10, opt-in): WEBUI_STORAGE=redis|sqlite
	// plus WEBUI_API_SECRET enables multi-tenant storage, session ownership
	// checks (22.2) and JWT auth. Without it the demo stays anonymous —
	// explicit opt-in, and the console's server-minted session flow (first
	// packet without session_id, adopt Agent-Session-Id) works in both modes.
	mode := strings.ToLower(envOr("WEBUI_STORAGE", ""))
	if mode == "" {
		return srv
	}
	switch mode {
	case "sqlite":
		st, err := service.NewSQLStorage(context.Background(), envOr("WEBUI_SQLITE_PATH", ".webui.db"))
		if err != nil {
			panic(err)
		}
		srv = srv.WithStorage(st)
	case "redis":
		client := redis.NewClient(&redis.Options{Addr: envOr("WEBUI_REDIS_ADDR", "127.0.0.1:6379")})
		srv = srv.WithStorage(service.NewRedisStorage(client))
	default:
		panic(fmt.Sprintf("unknown WEBUI_STORAGE %q (supported: sqlite, redis)", mode))
	}
	// Fail closed (22.1 discipline): a storage-backed deployment is a
	// multi-tenant, potentially internet-facing assembly; a known literal
	// signing secret would let anyone forge tokens. Require an explicit
	// secret instead of silently falling back to one.
	secret := envOr("WEBUI_API_SECRET", "")
	if secret == "" {
		panic("WEBUI_STORAGE requires WEBUI_API_SECRET (JWT signing key); refusing to start with a guessable secret")
	}
	jwt := service.NewJWTAuthenticator([]byte(secret), "agentscope-webui")
	srv = srv.WithJWTAuth(jwt).WithAuthenticator(jwt)
	srv.RegisterAuthRoutes(jwt)
	// Workspace management APIs (21.1): the console's workspace panel reads
	// /workspace/status (dir + git branch/changes) per session. Needs the
	// manager plus storage — demo mode stays without both, and the panel
	// shows a "not configured" empty state there.
	srv = srv.WithWorkspaceManager(gateway.NewWorkspaceManager("", ""))
	fmt.Println("Storage-backed mode:", mode, "(session ownership + JWT auth enforced)")
	return srv
}

func offloadTimeoutFromEnv() time.Duration {
	return durationFromEnvMS("TOOL_OFFLOAD_TIMEOUT_MS", 800)
}

func slowDemoDelayFromEnv() time.Duration {
	return durationFromEnvMS("SLOW_DEMO_DELAY_MS", 2000)
}

func durationFromEnvMS(key string, defaultMS int) time.Duration {
	s := envOr(key, "")
	if s == "" {
		return time.Duration(defaultMS) * time.Millisecond
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return time.Duration(defaultMS) * time.Millisecond
	}
	return time.Duration(n) * time.Millisecond
}

func buildApp() (agent.Agent, *gateway.Server) {
	toolOffload := gateway.NewToolOffloadManager().WithTimeout(offloadTimeoutFromEnv())

	var ag agent.Agent
	if apiKey := os.Getenv("DASHSCOPE_API_KEY"); apiKey != "" {
		modelName := envOr("DASHSCOPE_MODEL", "qwen3.7-plus")
		baseURL := envOr("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		realAgent, err := buildDashScopeAgent(apiKey, modelName, baseURL, toolOffload)
		if err != nil {
			panic(err)
		}
		ag = realAgent
		fmt.Println("Using DashScope:", modelName)
		fmt.Println("  Base URL:", baseURL)
		fmt.Println("  Tool offload timeout:", offloadTimeoutFromEnv())
	} else {
		ag = newDemoAgent()
		fmt.Println("DASHSCOPE_API_KEY not set — using built-in demo agent")
		fmt.Println("  Tip: send a message containing \"tool\" to preview tool-call UI")
		fmt.Println("  Tip: send \"offload\" after a background notification appears in your message")
	}

	return ag, buildGateway(ag, toolOffload)
}
