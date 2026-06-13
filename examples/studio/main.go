package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/evolver"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/observability"
	"github.com/linkerlin/agentscope.go/service"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed templates/*
var templatesFS embed.FS

// metricsHandler pilot for real metrics panel + e2e
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<html><body><h1>Studio Metrics Panel (P3 pilot)</h1>
<p>Integrate with gateway metrics (e.g. /debug/vars or custom MetricsHandler).</p>
<p>e2e: see gateway/e2e_integration_test.go for SSE/studio flow extension.</p>
<ul><li>Active Agents: (demo)</li><li>SSE connections: (demo)</li></ul>
</body></html>`)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// modelEmbeddingAdapter adapts model.EmbeddingModel to memory.EmbeddingModel.
type modelEmbeddingAdapter struct {
	inner model.EmbeddingModel
}

func (a *modelEmbeddingAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := a.inner.Embed(ctx, []string{text})
	if err != nil || len(resp.Data) == 0 {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

func (a *modelEmbeddingAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := a.inner.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

// Studio is a lightweight Go-native management UI (HTMX + templates).
// It demonstrates the new typed Credential + /schemas support (Phase 1)
// and serves as the foundation for the full pure-Go Studio (Phase 4).
//
// P3 改进：可扩展 metrics 展示（对接 gateway metrics）；e2e 结合 gateway e2e_integration_test 验证 SSE + 工具结果流。
//
// Run:
//
//	OPENAI_API_KEY=sk-... JWT_SECRET=dev go run ./examples/studio
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

	// Phase 5: add tracing middleware demo in studio too
	tracingMW := &observability.TracingMiddlewareAdapter{
		Tracer: observability.NoopTracer,
		Name:   "StudioDemo",
	}

	agent, err := react.Builder().
		Name("StudioDemo").
		SysPrompt("You are a helpful assistant in the AgentScope Go Studio demo with access to auto tools (workspace, tasks, web, json). Try commands that trigger tools.").
		Model(model).
		Tools(demoTools...).
		Middlewares(tracingMW).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Phase 5 demo: RecordingTracer to show traced spans (visible in console before server start)
	rec := &observability.RecordingTracer{}
	demoTraced := observability.NewTracedAgent("studio-phase5-demo", agent).WithTracer(rec)
	_, _ = demoTraced.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("studio tracing demo").Build())
	fmt.Println("Phase 5 tracing demo recorded spans in studio:", rec.Spans)

	// Storage + Auth
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte(jwtSecret), "agentscope-go")

	// Gateway with FULL auto-assembly enabled so the Studio itself demonstrates the effects.
	// Agents created via the UI (or API) will automatically receive rich tools, workspace, task store (auto in-mem), etc.
	// Demonstrate Phase 3 gateway embedding + cache enhancement
	embedModel := embedding.NewOpenAI(apiKey, "text-embedding-3-small")

	appCfg := gateway.AppConfig{
		Agent:             agent,
		Storage:           storage,
		Authenticator:     jwtAuth,
		JWTAuth:           jwtAuth,
		WorkspaceBaseDir:  "./studio_workspaces",
		AutoStandardTools: true,
		AutoToolOffload:   true,
		ModelCardsDir:     "../../model/cards",
		EmbeddingModel:    embedModel,
		EmbeddingCacheDir: "./studio_embed_cache",
	}
	srv := gateway.NewApp(appCfg)
	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterAppRoutes(jwtAuth) // includes everything + the auto features are active for this server

	// In-memory ReMe vector memory for the Studio debug panel.
	localVectorStore := memory.NewLocalVectorStore(&modelEmbeddingAdapter{inner: embedModel})
	remeCfg := memory.DefaultReMeFileConfig()
	remeCfg.WorkingDir = "./studio_reme"
	studioMemory, err := memory.NewReMeVectorMemory(remeCfg, nil, localVectorStore, &modelEmbeddingAdapter{inner: embedModel})
	if err != nil {
		log.Fatal(err)
	}

	// A2A Registry for the Studio browser panel.
	a2aRegistry := a2a.NewRegistry()
	_ = a2aRegistry.Register(a2a.AgentCard{
		Name:         "demo-coder",
		Description:  "Demo coding assistant agent",
		URL:          "http://localhost:9001",
		Version:      "1.0.0",
		Capabilities: []string{"streaming"},
	})
	a2aStopHealth := a2aRegistry.StartBackgroundHealthCheck(context.Background(), 30*time.Second)
	defer a2aStopHealth()

	// Evolver client + flow for the Studio gene/capsule panel.
	evolverClient := evolver.NewMockEvolver()
	evolverFlow := evolver.NewGEPFlow(evolverClient)

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

	http.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "models.html", nil)
	})

	http.HandleFunc("/memory", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "memory.html", nil)
	})

	http.HandleFunc("/a2a", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "a2a.html", nil)
	})

	http.HandleFunc("/evolver", func(w http.ResponseWriter, r *http.Request) {
		_ = tmpl.ExecuteTemplate(w, "evolver.html", nil)
	})

	// Studio Evolver API.
	http.HandleFunc("/api/studio/evolver/genes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		genes, err := evolverClient.ListGenes(r.Context(), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"genes": genes})
	})

	http.HandleFunc("/api/studio/evolver/capsules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		capsules, err := evolverClient.ListCapsules(r.Context(), 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"capsules": capsules})
	})

	http.HandleFunc("/api/studio/evolver/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Context  string `json:"context"`
			Strategy string `json:"strategy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Strategy == "" {
			req.Strategy = "balanced"
		}
		runRes, _, err := evolverFlow.RunAndSolidify(r.Context(), evolver.RunConfig{
			Context:  req.Context,
			Strategy: req.Strategy,
		}, true) // dry-run for demo safety
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"run_id":        runRes.RunID,
			"signals":       runRes.Signals,
			"selected_gene": runRes.SelectedGene,
			"gep_prompt":    runRes.GEPPrompt,
			"dry_run":       true,
		})
	})

	http.HandleFunc("/api/studio/evolver/solidify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req evolver.SolidifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := evolverClient.Solidify(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
	})

	// Studio A2A Registry API.
	http.HandleFunc("/api/studio/a2a/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			entries := a2aRegistry.List()
			writeJSON(w, map[string]any{"agents": entries})
		case http.MethodDelete:
			url := r.URL.Query().Get("url")
			a2aRegistry.Remove(url)
			writeJSON(w, map[string]string{"status": "removed"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/studio/a2a/discover", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a2aRegistry.Discover(r.Context(), req.URL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "discovered"})
	})

	http.HandleFunc("/api/studio/a2a/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var card a2a.AgentCard
		if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a2aRegistry.Register(card); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "registered"})
	})

	http.HandleFunc("/api/studio/a2a/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a2aRegistry.HealthCheck(r.Context())
		writeJSON(w, map[string]string{"status": "ok", "count": fmt.Sprintf("%d", len(a2aRegistry.List()))})
	})

	// Studio memory debug API.
	http.HandleFunc("/api/studio/memory/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		node := memory.NewMemoryNode(memory.MemoryTypePersonal, "studio-debug", req.Text)
		if err := studioMemory.AddMemory(r.Context(), node); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/studio/memory/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query().Get("q")
		nodes, err := studioMemory.RetrieveMemory(r.Context(), query, memory.RetrieveOptions{TopK: 5})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"query": query, "results": nodes})
	})

	// Delegate Gateway API routes to the Server so /api/v1/*, /v2/* and /health work.
	http.Handle("/api/v1/", srv)
	http.Handle("/v2/", srv)
	http.Handle("/health", srv)

	addr := ":8081"
	fmt.Printf("AgentScope Go Studio (light + auto-assembly) listening on %s\n", addr)
	fmt.Println("  - Dashboard: http://localhost:8081/")
	fmt.Println("  - Credentials (schemas-driven form): http://localhost:8081/credentials")
	fmt.Println("  - Models: http://localhost:8081/models")
	fmt.Println("  - Memory debug: http://localhost:8081/memory")
	fmt.Println("  - Auto-assembly active: Workspace + StandardTools (auto TaskStore) + Schedule restore + ToolOffload")
	fmt.Println("  - Full API under /api/v1/ and /v2/")
	http.HandleFunc("/studio/metrics", metricsHandler)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(addr, nil))
}
