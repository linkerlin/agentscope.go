package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

//go:embed static/*
var staticFiles embed.FS

// stubEmbedder is a deterministic offline embedder so the KB panel works
// without an API key. Replace with embedding.NewOpenAI(...) in production.
type stubEmbedder struct{}

func (stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, 8)
	for i := range v {
		v[i] = float32(len(text) % (i*5 + 1))
	}
	return v, nil
}

func buildKBService() *gateway.KBService {
	bs, err := blob.NewLocalBlobStore(".webui-blobs")
	if err != nil {
		panic(err)
	}
	reg := parser.NewRegistry(parser.NewTextParser(), parser.NewPPTXParser())
	ch := chunker.NewApproxTokenChunker()
	mgr := kb.NewCollectionPerKBManager(kb.NewInMemoryVectorStore(), func(string) (kb.Embedder, error) {
		return stubEmbedder{}, nil
	})
	return gateway.NewKBService(mgr, bs, reg, ch)
}

func main() {
	_, srv := buildApp()
	srv.RegisterV2Routes()
	srv.WithKBService(buildKBService())
	srv.RegisterKBRoutes()
	srv.RegisterModelRoutes()
	srv.RegisterProjectionRoutes()

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	staticHandler := staticFileHandler(staticRoot)
	apiHandler := gateway.CORSMiddleware(srv)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/health" || r.URL.Path == "/chat" || strings.HasPrefix(r.URL.Path, "/chat/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})

	addr := envOr("PORT", "8080")
	fmt.Printf("AgentScope Go Console: http://localhost:%s\n", addr)
	fmt.Printf("  Chat (AG-UI SSE):  POST/GET /v2/chat?protocol=agui\n")
	fmt.Printf("  Knowledge Bases:   GET/POST /api/v1/knowledge-bases\n")
	fmt.Printf("  Models:            GET /api/v1/models\n")
	if err := http.ListenAndServe(":"+addr, handler); err != nil {
		panic(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func staticFileHandler(root fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", staticContentType(path))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func staticContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
