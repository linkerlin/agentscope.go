package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/examples/shared/slowtool"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/dashscope"
	modelembed "github.com/linkerlin/agentscope.go/model/embedding"
	memembed "github.com/linkerlin/agentscope.go/memory/embedding"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/skill"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	scheduletool "github.com/linkerlin/agentscope.go/tool/schedule"
	"github.com/linkerlin/agentscope.go/tool/shell"
	tasktool "github.com/linkerlin/agentscope.go/tool/task"
	"github.com/linkerlin/agentscope.go/toolkit"
	"github.com/linkerlin/agentscope.go/workspace"
)

const defaultAgentID = "MultiTenantAgent"

type agentDeps struct {
	taskStore    *state.TaskStore
	scheduleMgr  scheduletool.Manager
	taskStop     tool.Tool
	toolOffload  *gateway.ToolOffloadManager
	slowDemoDelay time.Duration
}

func ensureDefaultSkills(reg *skill.Registry) {
	if reg == nil {
		return
	}
	for _, s := range reg.List() {
		if s.Name == "workspace-helper" {
			return
		}
	}
	reg.Register(&skill.AgentSkill{
		Name:        "workspace-helper",
		Description: "Tips for using the multi-tenant workspace example.",
		SkillContent: "# Workspace Helper\n\n- Use read_file to inspect welcome.txt\n" +
			"- slow_demo runs in the background when it exceeds the offload timeout\n" +
			"- TaskStop can cancel an offloaded task by task_id\n",
		Source: "demo",
	})
	reg.SetActive("workspace-helper_demo", true)
}

// buildAgentForSession creates a per-session ReAct agent (Py get_agent parity).
func buildAgentForSession(sw *gateway.SessionWorkspace, deps agentDeps) (agent.Agent, error) {
	if sw == nil || sw.Workspace == nil {
		return nil, fmt.Errorf("session workspace required")
	}
	wsDir := sw.Dir()
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
	welcome := filepath.Join(wsDir, "welcome.txt")
	if _, err := os.Stat(welcome); os.IsNotExist(err) {
		_ = os.WriteFile(welcome, []byte("Hello from multi-tenant workspace example.\n"), 0o644)
	}

	ensureDefaultSkills(sw.Skills)

	permEngine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
		{Target: "tool_name", Pattern: "read_file", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "list_directory", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "TaskGet", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "TaskList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleView", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "Skill", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "slow_demo", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskUpdate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleStop", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskStop", Decision: permission.DecisionAsk},
	})

	tools := []tool.Tool{
		file.NewReadFileTool(wsDir),
		file.NewListDirectoryTool(wsDir),
		file.NewWriteFileTool(wsDir),
		shell.NewShellCommandTool(wsDir, []string{"ls", "cat", "pwd", "echo"}, nil),
		slowtool.New(deps.slowDemoDelay),
	}
	if deps.taskStore != nil {
		tools = append(tools, tasktool.RegisterTools(deps.taskStore)...)
	}
	if deps.scheduleMgr != nil {
		tools = append(tools, scheduletool.RegisterTools(deps.scheduleMgr)...)
	}
	if deps.taskStop != nil {
		tools = append(tools, deps.taskStop)
	}

	tk := toolkit.NewToolkit()
	for _, t := range tools {
		if err := tk.Register(t); err != nil {
			return nil, err
		}
	}
	_, skillHook, err := skill.RegisterWithToolkit(tk, sw.Skills, skill.AttachOptions{})
	if err != nil {
		return nil, err
	}
	if deps.toolOffload != nil {
		tk.Use(gateway.NewToolOffloadMiddleware(deps.toolOffload, ""))
	}
	offloader := workspace.NewWorkspaceOffloader(sw.Workspace, ".offload")
	tk.Use(toolkit.NewOffloadMiddlewareWithOffloader(offloader, "", ".offload"))

	return react.Builder().
		Name(defaultAgentID).
		SysPrompt("You are a helpful assistant with access to a local workspace. You can read files, list directories, write files (requires user confirmation), run safe shell commands, manage tasks (TaskCreate/Get/List/Update), schedule recurring jobs (ScheduleCreate/List/Stop/View), cancel background tool runs (TaskStop), invoke slow_demo for long searches, and browse skills via Skill. Be concise.").
		Model(chatModel).
		Workspace(sw.Workspace).
		PermissionEngine(permEngine).
		Hooks(skillHook).
		Toolkit(tk).
		Build()
}

// buildAgent creates the ReAct agent with workspace tools and permission engine.
func buildAgent(wsDir string, deps agentDeps) (agent.Agent, error) {
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
		{Target: "tool_name", Pattern: "TaskGet", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "TaskList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleView", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "Skill", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "slow_demo", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskUpdate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleStop", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskStop", Decision: permission.DecisionAsk},
	})

	skillReg := skill.NewRegistry()
	ensureDefaultSkills(skillReg)

	tools := []tool.Tool{
		file.NewReadFileTool(wsDir),
		file.NewListDirectoryTool(wsDir),
		file.NewWriteFileTool(wsDir),
		shell.NewShellCommandTool(wsDir, []string{"ls", "cat", "pwd", "echo"}, nil),
		slowtool.New(deps.slowDemoDelay),
	}
	if deps.taskStore != nil {
		tools = append(tools, tasktool.RegisterTools(deps.taskStore)...)
	}
	if deps.scheduleMgr != nil {
		tools = append(tools, scheduletool.RegisterTools(deps.scheduleMgr)...)
	}
	if deps.taskStop != nil {
		tools = append(tools, deps.taskStop)
	}

	tk := toolkit.NewToolkit()
	for _, t := range tools {
		if err := tk.Register(t); err != nil {
			return nil, err
		}
	}
	_, skillHook, err := skill.RegisterWithToolkit(tk, skillReg, skill.AttachOptions{})
	if err != nil {
		return nil, err
	}
	if deps.toolOffload != nil {
		tk.Use(gateway.NewToolOffloadMiddleware(deps.toolOffload, ""))
	}
	offloader := workspace.NewWorkspaceOffloader(ws, ".offload")
	tk.Use(toolkit.NewOffloadMiddlewareWithOffloader(offloader, "", ".offload"))

	return react.Builder().
		Name(defaultAgentID).
		SysPrompt("You are a helpful assistant with access to a local workspace. You can read files, list directories, write files (requires user confirmation), run safe shell commands, manage tasks (TaskCreate/Get/List/Update), schedule recurring jobs (ScheduleCreate/List/Stop/View), cancel background tool runs (TaskStop), invoke slow_demo for long searches, and browse skills via Skill. Be concise.").
		Model(chatModel).
		Workspace(ws).
		PermissionEngine(permEngine).
		Hooks(skillHook).
		Toolkit(tk).
		Build()
}

// buildGateway wires a minimal gateway for e2e tests (mock agent without extra tools).
func buildGateway(ag agent.Agent) (*gateway.Server, *gateway.ToolOffloadManager) {
	storage := service.NewMemoryStorage()
	cipher, _ := service.NewCipherFromEnv()

	jwtSecret := envOr("JWT_SECRET", "dev-jwt-secret-change-me")
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	combinedAuth := service.NewAnyAuthenticator(apiAuth, jwtAuth)

	sessionMgr := gateway.NewSessionManager().WithStorage(storage)
	toolOffload := gateway.NewToolOffloadManager()

	srv := gateway.NewServer(ag).
		WithStorage(storage).
		WithCipher(cipher).
		WithSessionManager(sessionMgr).
		WithToolOffloadManager(toolOffload).
		WithAuthenticator(combinedAuth)

	_, file, _, _ := runtime.Caller(0)
	cardsDir := filepath.Join(filepath.Dir(file), "..", "..", "model", "cards")

	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterServiceRoutes()
	srv.WithModelCardsDir(cardsDir).RegisterModelRoutes()
	srv.RegisterV2Routes()
	return srv, toolOffload
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resolveEmbeddingModel() model.EmbeddingModel {
	if url := envOr("EMBEDDING_BASE_URL", envOr("OLLAMA_EMBED_URL", "")); url != "" {
		dim := 768
		if d := os.Getenv("EMBEDDING_DIMENSIONS"); d != "" {
			if v, err := parsePositiveInt(d); err == nil {
				dim = v
			}
		}
		return modelembed.NewOllamaEmbedder(url, envOr("EMBEDDING_MODEL", "nomic-embed-text"), dim).AsModel()
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		dim := 1536
		if d := os.Getenv("EMBEDDING_DIMENSIONS"); d != "" {
			if v, err := parsePositiveInt(d); err == nil {
				dim = v
			}
		}
		embedder := memembed.NewOpenAIEmbedderWithBaseURL(
			key,
			envOr("EMBEDDING_BASE_URL", ""),
			envOr("EMBEDDING_MODEL", ""),
		)
		return embedder.AsModel(dim)
	}
	return nil
}

func runServer(addr string) (http.Handler, func(context.Context) error, error) {
	wsRoot := envOr("WORKSPACE_ROOT", "./workspace_data")

	// Bootstrap gateway deps first so agent tools can use schedule/offload managers.
	storage := service.NewMemoryStorage()
	sessionMgr := gateway.NewSessionManager().WithStorage(storage)
	registry := gateway.NewAgentRegistry().WithStorage(storage)
	btm := gateway.NewBackgroundTaskManager(registry, sessionMgr).WithStorage(storage)
	btm.Start()

	taskStore := state.NewTaskStore()
	scheduleMgr := scheduleManagerAdapter{btm: btm}
	toolOffload := btm.ToolOffload().WithTimeout(offloadTimeoutFromEnv())
	taskStop := gateway.NewTaskStopTool(toolOffload)

	ag, err := buildAgent(wsRoot, agentDeps{
		taskStore:     taskStore,
		scheduleMgr:   scheduleMgr,
		taskStop:      taskStop,
		toolOffload:   toolOffload,
		slowDemoDelay: slowDemoDelayFromEnv(),
	})
	if err != nil {
		btm.Stop()
		return nil, nil, err
	}
	registry.Register(defaultAgentID, ag)

	cipher, _ := service.NewCipherFromEnv()
	jwtSecret := envOr("JWT_SECRET", "dev-jwt-secret-change-me")
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	combinedAuth := service.NewAnyAuthenticator(apiAuth, jwtAuth)

	_, file, _, _ := runtime.Caller(0)
	cardsDir := filepath.Join(filepath.Dir(file), "..", "..", "model", "cards")
	wsMgr := gateway.NewWorkspaceManager(wsRoot, filepath.Join(wsRoot, "skills"))

	mcpGWURL := envOr("MCP_GATEWAY_URL", "")
	mcpGWToken := envOr("MCP_GATEWAY_TOKEN", "dev-mcp-gateway-token")
	perSessionMCP := envOr("MCP_GATEWAY_PER_SESSION", "true") != "false"
	var mcpSidecar *workspace.MCPSidecar

	if perSessionMCP && mcpGWURL == "" {
		wsMgr = wsMgr.WithPerSessionMCPGateway(mcpGWToken)
		log.Println("MCP gateway: per-session sidecars (set MCP_GATEWAY_PER_SESSION=false for shared gateway)")
	} else if mcpGWURL == "" && envOr("MCP_GATEWAY_AUTO_START", "true") != "false" {
		sidecar, err := startMCPSidecar(context.Background(), mcpGWToken)
		if err != nil {
			btm.Stop()
			return nil, nil, fmt.Errorf("mcp gateway sidecar: %w", err)
		}
		mcpSidecar = sidecar
		mcpGWURL = sidecar.HostURL
	}
	if mcpGWURL != "" && !perSessionMCP {
		wsMgr = wsMgr.WithDefaultMCPGateway(mcpGWURL, mcpGWToken)
		if err := wsMgr.RestoreAllMCPsToGateway(context.Background()); err != nil {
			log.Printf("MCP gateway restore from .mcp files: %v", err)
		}
	}

	agentDepsBundle := agentDeps{
		taskStore:     taskStore,
		scheduleMgr:   scheduleMgr,
		taskStop:      taskStop,
		toolOffload:   toolOffload,
		slowDemoDelay: slowDemoDelayFromEnv(),
	}

	embeddingModel := resolveEmbeddingModel()

	srv := gateway.NewApp(gateway.AppConfig{
		Agent:              ag,
		Storage:            storage,
		Authenticator:      combinedAuth,
		JWTAuth:            jwtAuth,
		Cipher:             cipher,
		Registry:           registry,
		SessionManager:     sessionMgr,
		BackgroundTaskMgr:  btm,
		WorkspaceManager:   wsMgr,
		ToolOffloadManager: btm.ToolOffload(),
		ModelCardsDir:      cardsDir,
		EmbeddingModel:     embeddingModel,
	})
	srv.WithSessionAgentBuilder(func(ctx context.Context, userID, agentID, sessionID string) (agent.Agent, error) {
		sw, err := wsMgr.GetOrCreate(ctx, storage, userID, agentID, sessionID)
		if err != nil {
			return nil, err
		}
		return buildAgentForSession(sw, agentDepsBundle)
	})
	srv.RegisterAppRoutes(jwtAuth)

	handler := gateway.CORSMiddleware(srv)

	log.Printf("Multi-tenant workspace example on http://localhost%s", addr)
	log.Println("Endpoints:")
	log.Println("  POST /api/v1/auth/register   -> register tenant (returns api_key)")
	log.Println("  POST /api/v1/auth/login      -> login (returns JWT)")
	log.Println("  GET  /api/v1/me              -> current user (X-API-Key or Bearer)")
	log.Println("  POST /api/v1/sessions        -> create session")
	log.Println("  POST /api/v1/credentials     -> store encrypted credential")
	log.Println("  GET/POST/PATCH/DELETE /schedule -> persisted cron schedule API")
	log.Println("  GET/DELETE /background-tasks/{session_id} -> offloaded tool tasks")
	log.Println("  GET/POST /workspace/mcp|skill -> session workspace management")
	log.Println("  POST /v2/chat               -> Streamable HTTP (POST stream / GET subscribe / DELETE terminate)")
	log.Println("  POST /v2/chat/stream        -> legacy SSE (deprecated)")
	log.Println("  GET  /api/v1/models          -> list model cards")
	log.Println("  POST /v2/resume              -> resume HITL suspended session")
	log.Printf("Workspace root: %s", wsRoot)
	log.Printf("Model: %s", envOr("DASHSCOPE_MODEL", "qwen3.7-plus"))
	log.Printf("Agent tools: file + shell + Task* + Schedule* + TaskStop + slow_demo + Skill")
	log.Printf("Tool offload timeout: %s (set TOOL_OFFLOAD_TIMEOUT_MS)", offloadTimeoutFromEnv())
	if perSessionMCP && mcpGWURL == "" {
		log.Printf("MCP gateway: per-session (POST /workspace/mcp; .mcp persists per session)")
	} else if mcpGWURL != "" {
		log.Printf("MCP gateway: %s (POST /workspace/mcp registers MCP; .mcp persists per session)", mcpGWURL)
		if mcpSidecar != nil {
			log.Printf("MCP gateway sidecar: auto-started (set MCP_GATEWAY_URL to use external gateway)")
		}
	} else {
		log.Println("MCP gateway: disabled (set MCP_GATEWAY_AUTO_START=true or MCP_GATEWAY_URL)")
	}
	if embeddingModel != nil {
		log.Printf("Embedding API: POST /api/v1/embeddings (model %s)", embeddingModel.ModelName())
	}

	shutdown := func(ctx context.Context) error {
		btm.Stop()
		wsMgr.CloseAll()
		if mcpSidecar != nil {
			return mcpSidecar.Close(ctx)
		}
		return nil
	}
	return handler, shutdown, nil
}

func startMCPSidecar(ctx context.Context, token string) (*workspace.MCPSidecar, error) {
	port := 0
	if p := os.Getenv("MCP_GATEWAY_PORT"); p != "" {
		if v, err := parsePositiveInt(p); err == nil {
			port = v
		}
	}
	return workspace.StartMCPGatewaySidecar(ctx, workspace.MCPGatewaySidecarConfig{
		Token: token,
		Port:  port,
	}, func(gw *workspace.MCPGateway) {
		configPath := os.Getenv("MCP_GATEWAY_CONFIG")
		if configPath == "" {
			_, file, _, _ := runtime.Caller(0)
			defaultCfg := filepath.Join(filepath.Dir(file), "mcp_gateway.example.json")
			if _, err := os.Stat(defaultCfg); err == nil {
				configPath = defaultCfg
			}
		}
		if configPath == "" {
			return
		}
		cfg, err := workspace.LoadMCPGatewayConfigFile(configPath)
		if err != nil {
			log.Printf("MCP gateway config %s: %v", configPath, err)
			return
		}
		if err := gw.LoadConfig(ctx, cfg); err != nil {
			log.Printf("MCP gateway bootstrap from %s: %v", configPath, err)
		}
	})
}

func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid port: %q", s)
	}
	return n, nil
}

func offloadTimeoutFromEnv() time.Duration {
	if ms := envOr("TOOL_OFFLOAD_TIMEOUT_MS", "800"); ms != "" {
		if v, err := parsePositiveMS(ms); err == nil {
			return v
		}
	}
	return 800 * time.Millisecond
}

func slowDemoDelayFromEnv() time.Duration {
	if ms := envOr("SLOW_DEMO_DELAY_MS", "2000"); ms != "" {
		if v, err := parsePositiveMS(ms); err == nil {
			return v
		}
	}
	return 2 * time.Second
}

func parsePositiveMS(s string) (time.Duration, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid ms: %q", s)
	}
	return time.Duration(n) * time.Millisecond, nil
}

type scheduleManagerAdapter struct {
	btm *gateway.BackgroundTaskManager
}

func (a scheduleManagerAdapter) Schedule(ctx context.Context, job *schedule.Job) error {
	return a.btm.Schedule(ctx, job)
}

func (a scheduleManagerAdapter) Cancel(ctx context.Context, jobID string) error {
	return a.btm.Cancel(ctx, jobID)
}

func (a scheduleManagerAdapter) NextRun(jobID string) (string, error) {
	return a.btm.NextRunString(jobID)
}

func (a scheduleManagerAdapter) List() []*schedule.Job {
	return a.btm.List()
}
