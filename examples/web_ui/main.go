package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/model/dashscope"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	var ag agent.Agent
	if apiKey := os.Getenv("DASHSCOPE_API_KEY"); apiKey != "" {
		modelName := envOr("DASHSCOPE_MODEL", "qwen3.7-plus")
		baseURL := envOr("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		chatModel, err := dashscope.Builder().
			APIKey(apiKey).
			ModelName(modelName).
			BaseURL(baseURL).
			Build()
		if err != nil {
			panic(err)
		}
		realAgent, err := react.Builder().
			Name("WebUIAgent").
			SysPrompt("You are a helpful assistant. Be concise.").
			Model(chatModel).
			Build()
		if err != nil {
			panic(err)
		}
		ag = realAgent
		fmt.Println("Using DashScope:", modelName)
		fmt.Println("  Base URL:", baseURL)
	} else {
		ag = newDemoAgent()
		fmt.Println("DASHSCOPE_API_KEY not set — using built-in demo agent")
		fmt.Println("  Tip: send a message containing \"tool\" to preview tool-call UI")
	}

	srv := gateway.NewServer(ag)
	srv.RegisterV2Routes()

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	staticHandler := staticFileHandler(staticRoot)
	apiHandler := gateway.CORSMiddleware(srv)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})

	addr := envOr("PORT", "8080")
	fmt.Printf("AG-UI Web UI: http://localhost:%s\n", addr)
	fmt.Printf("  SSE endpoint: POST /v2/chat/stream?protocol=agui\n")
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
