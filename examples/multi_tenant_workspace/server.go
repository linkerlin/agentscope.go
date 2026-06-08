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
	"github.com/linkerlin/agentscope.go/model/dashscope"
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
	skillReg.Register(&skill.AgentSkill{
		Name:        "workspace-helper",
		Description: "Tips for using the multi-tenant workspace example.",
		SkillContent: "# Workspace Helper\n\n- Use read_file to inspect welcome.txt\n" +
			"- slow_demo runs in the background when it exceeds the offload timeout\n" +
			"- TaskStop can cancel an offloaded task by task_id\n",
		Source: "demo",
	})
	skillReg.SetActive("workspace-helper_demo", true)

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

func runServer(addr string) (http.Handler, error) {
	wsRoot := envOr("WORKSPACE_ROOT", "./workspace_data")

	// Bootstrap gateway deps first so agent tools can use schedule/offload managers.
	storage := service.NewMemoryStorage()
	sessionMgr := gateway.NewSessionManager().WithStorage(storage)
	registry := gateway.NewAgentRegistry().WithStorage(storage)
	btm := gateway.NewBackgroundTaskManager(registry, sessionMgr)
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
		return nil, err
	}
	registry.Register(defaultAgentID, ag)

	cipher, _ := service.NewCipherFromEnv()
	jwtSecret := envOr("JWT_SECRET", "dev-jwt-secret-change-me")
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	combinedAuth := service.NewAnyAuthenticator(apiAuth, jwtAuth)

	srv := gateway.NewServer(ag).
		WithStorage(storage).
		WithCipher(cipher).
		WithSessionManager(sessionMgr).
		WithRegistry(registry).
		WithBackgroundTaskManager(btm).
		WithToolOffloadManager(btm.ToolOffload()).
		WithAuthenticator(combinedAuth)

	_, file, _, _ := runtime.Caller(0)
	cardsDir := filepath.Join(filepath.Dir(file), "..", "..", "model", "cards")

	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterServiceRoutes()
	srv.RegisterScheduleRoutes()
	srv.WithModelCardsDir(cardsDir).RegisterModelRoutes()
	srv.RegisterV2Routes()

	handler := gateway.CORSMiddleware(srv)

	log.Printf("Multi-tenant workspace example on http://localhost%s", addr)
	log.Println("Endpoints:")
	log.Println("  POST /api/v1/auth/register   -> register tenant (returns api_key)")
	log.Println("  POST /api/v1/auth/login      -> login (returns JWT)")
	log.Println("  GET  /api/v1/me              -> current user (X-API-Key or Bearer)")
	log.Println("  POST /api/v1/sessions        -> create session")
	log.Println("  POST /api/v1/credentials     -> store encrypted credential")
	log.Println("  POST /schedule               -> HTTP cron schedule API")
	log.Println("  POST /v2/chat               -> Streamable HTTP (POST stream / GET subscribe / DELETE terminate)")
	log.Println("  POST /v2/chat/stream        -> legacy SSE (deprecated)")
	log.Println("  GET  /api/v1/models          -> list model cards")
	log.Println("  POST /v2/resume              -> resume HITL suspended session")
	log.Printf("Workspace root: %s", wsRoot)
	log.Printf("Model: %s", envOr("DASHSCOPE_MODEL", "qwen3.7-plus"))
	log.Printf("Agent tools: file + shell + Task* + Schedule* + TaskStop + slow_demo + Skill")
	log.Printf("Tool offload timeout: %s (set TOOL_OFFLOAD_TIMEOUT_MS)", offloadTimeoutFromEnv())

	return handler, nil
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
