package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenSandboxWorkspace_FullLifecycle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{
				"id":      "sb-1",
				"api_url": "http://" + r.Host + "/sandboxes/sb-1",
			})
			return
		}
	})
	mux.HandleFunc("/sandboxes/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/sandboxes/sb-1" && r.Method == "DELETE":
			w.WriteHeader(http.StatusNoContent)
			return
		case strings.HasSuffix(path, "/execute"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			json.NewEncoder(w).Encode(map[string]any{
				"exit_code": 0,
				"stdout":    "hello from sandbox",
				"stderr":    "",
			})
			return
		case strings.HasSuffix(path, "/files/list"):
			json.NewEncoder(w).Encode([]map[string]any{
				{"name": "a.go", "is_dir": false},
			})
			return
		case strings.HasSuffix(path, "/files/stat"):
			json.NewEncoder(w).Encode(map[string]any{
				"name": "a.go", "size": 42, "is_dir": false, "mod_time": 1700000000,
			})
			return
		case strings.HasSuffix(path, "/files/mkdir"):
			w.WriteHeader(http.StatusOK)
			return
		case strings.HasSuffix(path, "/files"):
			if r.Method == "GET" {
				w.Write([]byte("file data"))
				return
			}
			if r.Method == "PUT" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()
	ws, err := NewOpenSandboxWorkspace(ctx, OpenSandboxConfig{
		ID:        "os1",
		ServerURL: server.URL,
		APIKey:    "test-key",
		Image:     "ubuntu:24.04",
	})
	if err != nil {
		t.Fatalf("NewOpenSandboxWorkspace: %v", err)
	}
	if ws.sandboxID != "sb-1" {
		t.Fatalf("expected sandbox ID sb-1, got %s", ws.sandboxID)
	}

	// Execute
	result, err := ws.Execute(ctx, "echo hello", ExecuteOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Stdout != "hello from sandbox" {
		t.Fatalf("unexpected stdout: %s", result.Stdout)
	}

	// WriteFile + ReadFile
	if err := ws.WriteFile(ctx, "test.txt", []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := ws.ReadFile(ctx, "test.txt")
	if err != nil || string(data) != "file data" {
		t.Fatalf("ReadFile: %v data=%s", err, string(data))
	}

	// ListDir
	entries, err := ws.ListDir(ctx, "/")
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListDir: %v len=%d", err, len(entries))
	}

	// MkdirAll
	if err := ws.MkdirAll(ctx, "newdir", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Stat
	info, err := ws.Stat(ctx, "a.go")
	if err != nil || info.Name != "a.go" || info.Size != 42 {
		t.Fatalf("Stat: %v %+v", err, info)
	}

	// Close
	if err := ws.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpenSandboxWorkspace_MissingConfig(t *testing.T) {
	_, err := NewOpenSandboxWorkspace(context.Background(), OpenSandboxConfig{})
	if err == nil {
		t.Fatal("should error without ServerURL")
	}
}
