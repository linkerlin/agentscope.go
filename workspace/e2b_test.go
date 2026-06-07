package workspace

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestE2BClient_CreateSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/sandboxes" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sandbox_id": "sb-123",
			"client_id":  "cli-456",
		})
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	id, err := client.CreateSandbox(context.Background(), "tpl-1", 300*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "sb-123-cli-456" {
		t.Fatalf("expected sb-123-cli-456, got %s", id)
	}
}

func TestE2BClient_DeleteSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/sandboxes/sb-123" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.DeleteSandbox(context.Background(), "sb-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/sandboxes" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"sandbox_id": "sb-789"})
			return
		}
		if r.Method == "DELETE" && r.URL.Path == "/sandboxes/sb-789" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	ws, err := CreateE2BWorkspace(context.Background(), "ws-1", "tpl-1", client, 300*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.SandboxID() != "sb-789" {
		t.Fatalf("expected sb-789, got %s", ws.SandboxID())
	}
	if err := ws.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_Execute_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	_, err := ws.Execute(context.Background(), "ls", ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for unimplemented Execute")
	}
}

func TestE2BClient_CreateSandbox_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewE2BClient("wrong-key").WithBaseURL(server.URL)
	_, err := client.CreateSandbox(context.Background(), "tpl-1", 300*time.Second)
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestE2BClient_CreateSandbox_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	_, err := client.CreateSandbox(context.Background(), "tpl-1", 300*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestE2BClient_RefreshSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/sandboxes/sb-123/refreshes" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.RefreshSandbox(context.Background(), "sb-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BClient_RefreshSandbox_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.RefreshSandbox(context.Background(), "sb-bad"); err == nil {
		t.Fatal("expected error for refresh failure")
	}
}

func TestE2BClient_SetSandboxTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/sandboxes/sb-123/timeout" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.SetSandboxTimeout(context.Background(), "sb-123", 600*time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BClient_SetSandboxTimeout_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.SetSandboxTimeout(context.Background(), "sb-bad", 600*time.Second); err == nil {
		t.Fatal("expected error for set timeout failure")
	}
}

func TestE2BClient_DeleteSandbox_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	if err := client.DeleteSandbox(context.Background(), "sb-missing"); err == nil {
		t.Fatal("expected error for delete not found")
	}
}

func TestE2BWorkspace_Refresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/sandboxes/sb-456/refreshes" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	ws := NewE2BWorkspace("ws-1", "sb-456", client)
	if err := ws.Refresh(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_Refresh_NoClient(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	if err := ws.Refresh(context.Background()); err == nil {
		t.Fatal("expected error when no client")
	}
}

func TestE2BWorkspace_Close_NoClient(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	if err := ws.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_Close_EmptySandboxID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call server when sandboxID is empty")
	}))
	defer server.Close()

	client := NewE2BClient("test-key").WithBaseURL(server.URL)
	ws := NewE2BWorkspace("ws-1", "", client)
	if err := ws.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_ReadFile(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/files" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("path") != "/home/user/test.txt" {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		w.Write([]byte("hello world"))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	data, err := ws.ReadFile(context.Background(), "/home/user/test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %s", string(data))
	}
}

func TestE2BWorkspace_ReadFile_NotFound(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":"not_found","message":"file not found"}`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	_, err := ws.ReadFile(context.Background(), "/missing.txt")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if err != fs.ErrNotExist {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestE2BWorkspace_WriteFile(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/files" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("path") != "/home/user/test.txt" {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		// Verify multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "hello world" {
			http.Error(w, "bad data", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`[{"name":"test.txt","type":"FILE_TYPE_FILE","path":"/home/user/test.txt"}]`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	if err := ws.WriteFile(context.Background(), "/home/user/test.txt", []byte("hello world"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_ListDir(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/filesystem.Filesystem/ListDir" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"entries":[
			{"name":"foo","type":"FILE_TYPE_FILE","path":"/home/user/foo"},
			{"name":"bar","type":"FILE_TYPE_DIRECTORY","path":"/home/user/bar"}
		]}`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	entries, err := ws.ListDir(context.Background(), "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "foo" || entries[0].IsDir {
		t.Fatalf("expected foo file, got %+v", entries[0])
	}
	if entries[1].Name != "bar" || !entries[1].IsDir {
		t.Fatalf("expected bar dir, got %+v", entries[1])
	}
}

func TestE2BWorkspace_MkdirAll(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/filesystem.Filesystem/MakeDir" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	if err := ws.MkdirAll(context.Background(), "/home/user/newdir", 0755); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2BWorkspace_MkdirAll_AlreadyExists(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"code":"already_exists","message":"directory already exists"}`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	if err := ws.MkdirAll(context.Background(), "/home/user/exists", 0755); err != nil {
		t.Fatalf("unexpected error for already_exists: %v", err)
	}
}

func TestE2BWorkspace_Stat(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/filesystem.Filesystem/Stat" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"entry":{"name":"test.txt","type":"FILE_TYPE_FILE","path":"/home/user/test.txt"}}`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	fi, err := ws.Stat(context.Background(), "/home/user/test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi.Name != "test.txt" || fi.IsDir {
		t.Fatalf("expected test.txt file, got %+v", fi)
	}
}

func TestE2BWorkspace_Stat_NotFound(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":"not_found","message":"file not found"}`))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	_, err := ws.Stat(context.Background(), "/missing.txt")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if err != fs.ErrNotExist {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestE2BWorkspace_Execute(t *testing.T) {
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/process.Process/Start" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// NDJSON stream
		w.Write([]byte(`{"event":{"start":{"pid":123}}}` + "\n"))
		w.Write([]byte(`{"event":{"data":{"stdout":"aGVsbG8=","stderr":""}}}` + "\n"))
		w.Write([]byte(`{"event":{"end":{"exitCode":0,"error":""}}}` + "\n"))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	result, err := ws.Execute(context.Background(), "echo hello", ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello" {
		t.Fatalf("expected 'hello', got %s", result.Stdout)
	}
}

func TestE2BWorkspace_Execute_WithEnvAndCwd(t *testing.T) {
	var captured map[string]any
	envd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/process.Process/Start" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"event":{"start":{"pid":456}}}` + "\n"))
		w.Write([]byte(`{"event":{"end":{"exitCode":0,"error":""}}}` + "\n"))
	}))
	defer envd.Close()

	ws := NewE2BWorkspace("ws-1", "sb-1", NewE2BClient(""))
	ws.envdURL = envd.URL
	ws.envdHTTP = envd.Client()

	_, err := ws.Execute(context.Background(), "echo $FOO", ExecuteOptions{
		WorkingDir: "/home/user",
		Env:        map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	proc := captured["process"].(map[string]any)
	if proc["cwd"] != "/home/user" {
		t.Fatalf("expected cwd /home/user, got %v", proc["cwd"])
	}
	envs := proc["envs"].(map[string]any)
	if envs["FOO"] != "bar" {
		t.Fatalf("expected env FOO=bar, got %v", envs["FOO"])
	}
}
