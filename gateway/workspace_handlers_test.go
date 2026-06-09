package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

func TestWorkspace_SkillCRUD(t *testing.T) {
	root := t.TempDir()
	skillsRoot := filepath.Join(root, "skills")
	skillDir := filepath.Join(skillsRoot, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: demo skill\n---\n# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID:        "s1",
		UserID:    "u1",
		AgentID:   "a1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(root, skillsRoot)
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	q := "?agent_id=a1&session_id=s1"
	addBody, _ := json.Marshal(addSkillRequest{SkillPath: "demo"})
	req := httptest.NewRequest(http.MethodPost, "/workspace/skill"+q, bytes.NewReader(addBody))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("add skill: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/workspace/skill"+q, nil)
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list skills: expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/workspace/skill/demo"+q, nil)
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete skill: expected 204, got %d", rr.Code)
	}
}

func TestBackgroundTasks_ListAndCancel(t *testing.T) {
	mgr := NewToolOffloadManager()
	taskID := mgr.registerTask("sess1", "slow_tool")

	srv := NewServer(&mockAgent{name: "test"})
	srv.WithToolOffloadManager(mgr).RegisterBackgroundTaskRoutes()

	req := httptest.NewRequest(http.MethodGet, "/background-tasks/sess1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}
	var resp listBackgroundTasksResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || resp.Tasks[0].ID != taskID {
		t.Fatalf("unexpected tasks: %+v", resp)
	}

	req = httptest.NewRequest(http.MethodDelete, "/background-tasks/sess1/"+taskID, nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("cancel: expected 204, got %d", rr.Code)
	}
}
