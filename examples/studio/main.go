package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/service"
)

//go:embed templates/*
var templatesFS embed.FS

// Studio is a lightweight Go-native management UI (HTMX + templates).
// It demonstrates the new typed Credential + /schemas support (Phase 1)
// and serves as the foundation for the full pure-Go Studio (Phase 4).
//
// Run:
//   OPENAI_API_KEY=sk-... JWT_SECRET=dev go run ./examples/studio
//
// Then open http://localhost:8081
//
// The studio reuses the existing gateway + service layer.
func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// Model (for demo agent)
	model, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Agent for studio demo - use StandardTools so even base has some auto tools; dynamic sessions via AutoStandardTools get full (file+task+web+json+schedule etc.)
	demoTools := gateway.StandardTools(gateway.StandardToolsOptions{
		WorkspaceDir:    "./studio_workspaces",
		ReadOnly:        false,
		IncludeWeb:      true,
		IncludeJSON:     true,
		IncludeTask:     true,
		IncludeSchedule: false,
	})
	agent, err := react.Builder().
		Name("StudioDemo").
		SysPrompt("You are a helpful assistant in the AgentScope Go Studio demo with access to auto tools (workspace, tasks, web, json). Try commands that trigger tools.").
		Model(model).
		Tools(demoTools...).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Storage + Auth
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")

	// Gateway with FULL auto-assembly enabled so the Studio itself demonstrates the effects.
	// Agents created via the UI (or API) will automatically receive rich tools, workspace, task store (auto in-mem), etc.
	appCfg := gateway.AppConfig{
		Agent:             agent,
		Storage:           storage,
		Authenticator:     jwtAuth,
		JWTAuth:           jwtAuth,
		WorkspaceBaseDir:  "./studio_workspaces",
		AutoStandardTools: true,
		AutoToolOffload:   true,
		// Demonstrate Phase 3 gateway embedding + cache enhancement
		EmbeddingModel:    embedding.NewOpenAI(apiKey, "text-embedding-3-small"),
		EmbeddingCacheDir: "./studio_embed_cache",
	}
	srv := gateway.NewApp(appCfg)
	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterAppRoutes(jwtAuth) // includes everything + the auto features are active for this server

	// Studio UI routes (pure Go + HTMX) - now with expanded demo of auto-assembly
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "index.html", map[string]any{
			"Title": "AgentScope Go Studio (Light + Auto-Assembly)",
			"Note":  "Pure-Go lightweight Studio using HTMX. Server started with full auto-assembly (AutoStandardTools, Workspace, ToolOffload, Schedule restore).",
		})
	})

	http.HandleFunc("/credentials", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "credentials.html", map[string]any{
			"SchemasURL": "/api/v1/credentials/schemas",
			"ListURL":    "/api/v1/credentials",
			"CreateURL":  "/api/v1/credentials",
		})
	})

	http.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "agents.html", nil)
	})

	http.HandleFunc("/schedules", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "schedules.html", nil)
	})

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "chat.html", nil)
	})

	addr := ":8081"
	fmt.Printf("AgentScope Go Studio (light + auto-assembly) listening on %s\n", addr)
	fmt.Println("  - Dashboard: http://localhost:8081/")
	fmt.Println("  - Credentials (schemas-driven form): http://localhost:8081/credentials")
	fmt.Println("  - Auto-assembly active: Workspace + StandardTools (auto TaskStore) + Schedule restore + ToolOffload")
	fmt.Println("  - Full API under /api/v1/ and /v2/")
	log.Fatal(http.ListenAndServe(addr, nil))
}
