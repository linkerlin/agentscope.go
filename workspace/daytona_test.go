package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDaytonaWorkspace_CreateExecuteClose(t *testing.T) {
	var createdID string

	mux := http.NewServeMux()

	// POST /workspace → create
	mux.HandleFunc("/workspace", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		createdID = "ws-test-123"
		json.NewEncoder(w).Encode(map[string]any{
			"id":      createdID,
			"nodeUrl": "http://" + r.Host,
		})
	})

	// DELETE /workspace/{id}
	mux.HandleFunc("/workspace/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// POST /toolbox/{id}/execute
	mux.HandleFunc("/toolbox/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		cmd, _ := body["command"].(string)
		json.NewEncoder(w).Encode(map[string]any{
			"exitCode": 0,
			"stdout":   "output: " + cmd,
			"stderr":   "",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create workspace
	ws, err := NewDaytonaWorkspace(context.Background(), DaytonaConfig{
		ID:        "d1",
		ServerURL: server.URL,
		APIKey:    "test-key",
	})
	if err != nil {
		t.Fatalf("NewDaytonaWorkspace: %v", err)
	}
	if ws.workspaceID != "ws-test-123" {
		t.Fatalf("expected workspace ID ws-test-123, got %s", ws.workspaceID)
	}

	// Execute
	result, err := ws.Execute(context.Background(), "echo hello", ExecuteOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ExitCode != 0 || result.Stdout != "output: echo hello" {
		t.Fatalf("unexpected result: %+v", result)
	}

	// Close
	if err := ws.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDaytonaWorkspace_FileOps(t *testing.T) {
	mux := http.NewServeMux()

	// Create
	mux.HandleFunc("/workspace", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "ws-files",
			"nodeUrl": "http://" + r.Host,
		})
	})

	// Delete
	mux.HandleFunc("/workspace/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Toolbox: handle all /toolbox/ routes in one handler
	mux.HandleFunc("/toolbox/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/file"):
			if r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == "GET" {
				w.Write([]byte("file contents"))
				return
			}
		case strings.HasSuffix(path, "/dir"):
			json.NewEncoder(w).Encode([]map[string]any{
				{"name": "a.txt", "isDir": false},
				{"name": "b", "isDir": true},
			})
			return
		case strings.HasSuffix(path, "/mkdir"):
			w.WriteHeader(http.StatusOK)
			return
		case strings.HasSuffix(path, "/stat"):
			json.NewEncoder(w).Encode(map[string]any{
				"name":    "test.txt",
				"size":    100,
				"isDir":   false,
				"modTime": 1700000000,
			})
			return
		case strings.HasSuffix(path, "/execute"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			cmd, _ := body["command"].(string)
			json.NewEncoder(w).Encode(map[string]any{
				"exitCode": 0,
				"stdout":   "output: " + cmd,
				"stderr":   "",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	ws, err := NewDaytonaWorkspace(context.Background(), DaytonaConfig{
		ID:        "d1",
		ServerURL: server.URL,
		APIKey:    "test-key",
	})
	if err != nil {
		t.Fatalf("NewDaytonaWorkspace: %v", err)
	}
	defer ws.Close()

	ctx := context.Background()

	// WriteFile
	if err := ws.WriteFile(ctx, "test.txt", []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// ReadFile
	data, err := ws.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "file contents" {
		t.Fatalf("expected 'file contents', got %s", string(data))
	}

	// ListDir
	entries, err := ws.ListDir(ctx, "/")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// MkdirAll
	if err := ws.MkdirAll(ctx, "newdir", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Stat
	info, err := ws.Stat(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name != "test.txt" || info.Size != 100 {
		t.Fatalf("unexpected stat: %+v", info)
	}
}

func TestDaytonaWorkspace_MissingConfig(t *testing.T) {
	_, err := NewDaytonaWorkspace(context.Background(), DaytonaConfig{})
	if err == nil {
		t.Fatal("should error without ServerURL")
	}

	_, err = NewDaytonaWorkspace(context.Background(), DaytonaConfig{ServerURL: "http://x"})
	if err == nil {
		t.Fatal("should error without APIKey")
	}
}

func TestDaytonaWorkspace_CreateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	}))
	defer server.Close()

	_, err := NewDaytonaWorkspace(context.Background(), DaytonaConfig{
		ServerURL: server.URL,
		APIKey:    "key",
	})
	if err == nil {
		t.Fatal("should error on 500")
	}
}

func TestDaytonaWorkspace_AuthHeaders(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/workspace" && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{"id": "ws1", "nodeUrl": "http://" + r.Host})
		}
		if r.URL.Path == "/workspace/ws1" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	ws, _ := NewDaytonaWorkspace(context.Background(), DaytonaConfig{
		ServerURL: server.URL,
		APIKey:    "my-secret-key",
	})
	defer ws.Close()

	if gotAuth != "Bearer my-secret-key" {
		t.Fatalf("expected Bearer auth, got %s", gotAuth)
	}
}

// ensure body is read to avoid connection reuse issues
var _ = io.EOF
