package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
)

// 全功能生产级 Agent 服务示例。
//
// 启动：
//
//	OPENAI_API_KEY=sk-xxx JWT_SECRET=secret go run ./examples/production/
//
// 端点：
//
//	GET  /health                    — 健康检查
//	POST /api/v1/auth/register      — 注册
//	POST /api/v1/auth/login         — 登录
//	POST /v2/chat/stream            — SSE 对话（需 JWT）
//	GET  /v2/chat/ws                — WebSocket 对话（需 JWT）
//	POST /v2/resume                 — 恢复挂起的 Agent
func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// 1. 模型
	model, _ := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()

	// 2. 存储（生产用 RedisStorage，示例用内存）
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")

	// 3. 构建初始 static agent 时也默认使用 auto tools（当启用 AutoStandardTools 时）。
	// 之前主要服务动态 session agent；现在 static base agent 也一致使用自动装配的工具集。
	// StandardTools 内部会在 IncludeTask 且未提供 store 时自动创建简单的 in-memory TaskStore。
	staticToolOpts := gateway.StandardToolsOptions{
		WorkspaceDir:    "./workspaces/default",
		ReadOnly:        false,
		IncludeWeb:      true,
		IncludeJSON:     true,
		IncludeTask:     true,
		IncludeSchedule: false, // 静态示例中 schedule mgr 通常在运行时由 BTM 提供
	}
	staticTools := gateway.StandardTools(staticToolOpts)

	// 4. ReAct Agent (static base，使用 auto tools)
	baseAgent, err := react.Builder().
		Name("Assistant").
		SysPrompt("You are a helpful production assistant. Be concise and precise.").
		Model(model).
		Memory(memory.NewInMemoryMemory()).
		Tools(staticTools...).
		PermissionEngine(permission.NewEngine(permission.ModeExplore, []permission.Rule{
			{Target: "tool_name", Pattern: "web_fetch", Decision: permission.DecisionAsk},
		})).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 5. 使用高层 NewApp（更接近 Python create_app + lifespan）
	// 更多自动装配：BTM (schedule restore)、WorkspaceManager、StandardTools（file+task+web+json）、ToolOffload、默认权限等。
	// 初始 static agent 也使用了相同的 auto tools 逻辑。
	appCfg := gateway.AppConfig{
		Agent:                 baseAgent,
		Storage:               storage,
		Authenticator:         jwtAuth,
		JWTAuth:               jwtAuth,
		WorkspaceBaseDir:      "./workspaces",
		AutoStandardTools:     true,
		AutoToolOffload:       true,
		DefaultPermissionMode: permission.ModeExplore,
	}
	srv := gateway.NewApp(appCfg)
	srv.RegisterAppRoutes(jwtAuth)

	srv.Start()
	defer srv.Close()

	addr := ":" + envOrDefault("PORT", "8080")
	fmt.Printf("=== AgentScope Production Server (full bootstrap) ===\n")
	fmt.Printf("Health:  http://localhost%s/health\n", addr)
	fmt.Printf("使用 NewApp + RegisterAppRoutes + Start() 获得自动 schedule restore\n")
	fmt.Printf("SSE:     http://localhost%s/v2/chat/stream\n", addr)
	fmt.Printf("WS:      ws://localhost%s/v2/chat/ws\n", addr)
	fmt.Printf("Schedules 持久化后重启会自动恢复（通过 BackgroundTaskManager）\n")
	fmt.Printf("====================================================\n")
	log.Fatal(http.ListenAndServe(addr, srv))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
