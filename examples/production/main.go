package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
	jsontool "github.com/linkerlin/agentscope.go/tool/json"
	webtool "github.com/linkerlin/agentscope.go/tool/web"
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

	// 3. 内置工具
	var tools []tool.Tool
	tools = append(tools, jsontool.NewParseTool(), jsontool.NewQueryTool())
	tools = append(tools, webtool.NewFetchTool(10*time.Second))

	// 4. ReAct Agent
	agent, err := react.Builder().
		Name("Assistant").
		SysPrompt("You are a helpful production assistant. Be concise and precise.").
		Model(model).
		Memory(memory.NewInMemoryMemory()).
		Tools(tools...).
		PermissionEngine(permission.NewEngine(permission.ModeExplore, []permission.Rule{
			{Target: "tool_name", Pattern: "web_fetch", Decision: permission.DecisionAsk},
		})).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 5. Gateway：认证 + 存储 + 会话管理 + 全量路由
	srv := gateway.NewServer(agent).
		WithStorage(storage).
		WithAuthenticator(jwtAuth).
		WithSessionManager(gateway.NewSessionManager().WithStorage(storage))
	srv.RegisterAuthRoutes(jwtAuth)   // /api/v1/auth/*
	srv.RegisterServiceRoutes()       // /api/v1/agents|sessions|credentials
	srv.RegisterV2Routes()            // /v2/chat|resume|ws

	addr := ":" + envOrDefault("PORT", "8080")
	fmt.Printf("=== AgentScope Production Server ===\n")
	fmt.Printf("Health:  http://localhost%s/health\n", addr)
	fmt.Printf("SSE:     http://localhost%s/v2/chat/stream\n", addr)
	fmt.Printf("WS:      ws://localhost%s/v2/chat/ws\n", addr)
	fmt.Printf("=====================================\n")
	log.Fatal(http.ListenAndServe(addr, srv))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ context.Context
