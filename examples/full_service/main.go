package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
)

// Full-featured production service with MAXIMUM auto-assembly.
//
// This example demonstrates the power of the enhanced NewApp + AppConfig auto features
// (Phase 2+): schedule restore on startup, auto WorkspaceManager, auto StandardTools
// (including auto in-memory TaskStore), auto ToolOffload, default permission, etc.
//
// Both the initial static base agent AND all dynamic per-session agents (via registry)
// benefit from the auto tools.
//
// Run:
//   OPENAI_API_KEY=sk-xxx JWT_SECRET=secret go run ./examples/full_service
//
// Then use the Studio (examples/studio) or API to create agents/credentials/sessions/schedules.
// Schedules will survive restarts thanks to auto BTM + restore.

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// 1. Model
	m, _ := openai.Builder().APIKey(apiKey).ModelName("gpt-4o-mini").Build()

	// 2. Storage + Auth (in prod use Redis + proper secrets)
	storage := service.NewMemoryStorage()
	jwt := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")

	// 3. Build the INITIAL STATIC base agent ALSO using auto tools (via StandardTools).
	//    When AutoStandardTools is enabled below, the framework will also auto-provide
	//    the same rich tool set (file ops, glob/grep, web, json, tasks with auto in-mem store,
	//    schedules, tool-offload, workspace, permission) to dynamic session agents.
	baseTools := gateway.StandardTools(gateway.StandardToolsOptions{
		WorkspaceDir:    "./workspaces",
		ReadOnly:        false,
		IncludeWeb:      true,
		IncludeJSON:     true,
		IncludeTask:     true,  // will auto-create in-memory TaskStore inside StandardTools
		IncludeSchedule: false,
	})

	base, err := react.Builder().
		Name("FullServiceBase").
		SysPrompt("You are a powerful assistant with automatic access to workspace tools, tasks, web, json etc.").
		Model(m).
		Memory(memory.NewInMemoryMemory()).
		Tools(baseTools...).
		PermissionEngine(permission.NewEngine(permission.ModeExplore, nil)).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 4. One-liner rich config with heavy auto-assembly.
	//    - Auto BTM (schedule persistence + restore on Start)
	//    - Auto WorkspaceManager from base dir
	//    - Auto StandardTools + auto in-mem TaskStore for both static and dynamic agents
	//    - Auto ToolOffload
	//    - Default permission
	//    - (Optional) Embeddings via the new top-level embedding/ package:
	//      emb := embedding.NewOpenAI(apiKey, "text-embedding-3-small")
	//      // or with cache: emb = embedding.WithFileCache(emb, ".cache/embed")
	//      appCfg.EmbeddingModel = emb
	appCfg := gateway.AppConfig{
		Agent:                 base,
		Storage:               storage,
		Authenticator:         jwt,
		JWTAuth:               jwt,
		WorkspaceBaseDir:      "./workspaces",
		AutoStandardTools:     true,
		AutoToolOffload:       true,
		DefaultPermissionMode: permission.ModeExplore,
		// Phase 3: enable embedding with auto cache from new embedding/ package (uses FileCache for repeated calls)
		EmbeddingModel:    embedding.WithFileCache(embedding.NewOpenAI(apiKey, "text-embedding-3-small"), "./workspaces/.embed_cache"),
	}
	srv := gateway.NewApp(appCfg)
	srv.RegisterAppRoutes(jwt)

	srv.Start()
	defer srv.Close()

	addr := ":" + envOrDefault("PORT", "8080")
	fmt.Printf("=== AgentScope Full-Service (heavy auto-assembly) ===\n")
	fmt.Printf("Health: http://localhost%s/health\n", addr)
	fmt.Printf("Auto: workspace + standard tools (auto TaskStore) + schedule restore + tool offload\n")
	fmt.Printf("SSE:    http://localhost%s/v2/chat/stream\n", addr)
	fmt.Printf("====================================================\n")
	log.Fatal(http.ListenAndServe(addr, srv))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
