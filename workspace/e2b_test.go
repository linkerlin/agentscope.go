package workspace

import (
	"context"
	"encoding/json"
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
	if id != "sb-123" {
		t.Fatalf("expected sb-123, got %s", id)
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

func TestE2BWorkspace_ReadFile_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	_, err := ws.ReadFile(context.Background(), "/tmp/test.txt")
	if err == nil {
		t.Fatal("expected error for unimplemented ReadFile")
	}
}

func TestE2BWorkspace_WriteFile_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	err := ws.WriteFile(context.Background(), "/tmp/test.txt", []byte("hello"), 0644)
	if err == nil {
		t.Fatal("expected error for unimplemented WriteFile")
	}
}

func TestE2BWorkspace_ListDir_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	_, err := ws.ListDir(context.Background(), "/tmp")
	if err == nil {
		t.Fatal("expected error for unimplemented ListDir")
	}
}

func TestE2BWorkspace_MkdirAll_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	err := ws.MkdirAll(context.Background(), "/tmp/dir", 0755)
	if err == nil {
		t.Fatal("expected error for unimplemented MkdirAll")
	}
}

func TestE2BWorkspace_Stat_NotImplemented(t *testing.T) {
	ws := NewE2BWorkspace("ws-1", "sb-1", nil)
	_, err := ws.Stat(context.Background(), "/tmp/test.txt")
	if err == nil {
		t.Fatal("expected error for unimplemented Stat")
	}
}
