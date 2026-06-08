package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/linkerlin/agentscope.go/gateway"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	_, srv := buildApp()
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
	fmt.Printf("  Streamable HTTP: POST/GET/DELETE /v2/chat?protocol=agui\n")
	fmt.Printf("  Page load: GET /v2/chat auto-reconnect (session id in localStorage)\n")
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
