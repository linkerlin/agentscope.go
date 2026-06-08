package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInjectOffloadHints(t *testing.T) {
	mgr := NewToolOffloadManager()
	mgr.PushResult("s1", "hint-1")
	srv := &Server{toolOffload: mgr}

	got := injectOffloadHints(srv, "s1", "hello")
	if got != "hint-1\n\nhello" {
		t.Fatalf("unexpected text: %q", got)
	}
	if injectOffloadHints(srv, "s1", "again") != "again" {
		t.Fatal("expected hints cleared after pop")
	}
}

func TestModelHandlers_ListAndGet(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	cardsDir := filepath.Join(filepath.Dir(file), "..", "model", "cards")

	srv := NewServer(nil).WithModelCardsDir(cardsDir)
	srv.RegisterModelRoutes()

	t.Run("list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var cards []map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&cards); err != nil {
			t.Fatal(err)
		}
		if len(cards) < 2 {
			t.Fatalf("expected at least 2 cards, got %d", len(cards))
		}
	})

	t.Run("get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/models/qwen3.7-plus", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var card map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&card); err != nil {
			t.Fatal(err)
		}
		if card["id"] != "qwen3.7-plus" {
			t.Fatalf("unexpected id: %v", card["id"])
		}
	})
}

func TestTaskStopTool(t *testing.T) {
	mgr := NewToolOffloadManager()
	mgr.registerTask("sess", "slow_tool")

	tasks := mgr.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	taskID := tasks[0].ID

	stop := NewTaskStopTool(mgr)
	resp, err := stop.Execute(t.Context(), map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("empty response")
	}
}
