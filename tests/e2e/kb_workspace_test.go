package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

func TestE2E_KBCreateUploadSearchChunks(t *testing.T) {
	e := newEnv(t, nil)

	code, body := e.mustDo(http.MethodPost, "/api/v1/knowledge-bases", map[string]any{
		"name": "policies", "description": "hr", "embedder_id": "stub",
	})
	mustStatus(t, code, http.StatusCreated, body)

	code, body = e.mustDo(http.MethodPost, "/api/v1/knowledge-bases/policies/documents", map[string]any{
		"content":    "The PTO policy grants 15 days of paid time off per year.",
		"media_type": "text/plain",
		"source":     "pto.txt",
	})
	mustStatus(t, code, http.StatusCreated, body)
	var up struct {
		DocID  string `json:"doc_id"`
		Chunks int    `json:"chunks"`
	}
	e.decode(body, &up)
	if up.DocID == "" || up.Chunks == 0 {
		t.Fatalf("upload: %+v body=%s", up, body)
	}

	code, body = e.mustDo(http.MethodPost, "/api/v1/knowledge-bases/policies/search", map[string]any{
		"query": "PTO policy",
	})
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "PTO") && !contain(body, "15 days") {
		t.Fatalf("search miss: %s", body)
	}

	code, body = e.mustDo(http.MethodGet, "/api/v1/knowledge-bases/policies/documents/"+up.DocID+"/chunks", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "chunk") && !contain(body, "text") && !contain(body, "PTO") {
		t.Fatalf("chunks: %s", body)
	}
}

func TestE2E_WorkspaceListReadAndRejectTraversal(t *testing.T) {
	e := newEnv(t, nil)
	now := time.Now()
	if err := e.storage.SaveSession(t.Context(), &service.Session{
		ID: "ws-1", UserID: e.userID, AgentID: "e2e-agent",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	q := url.Values{"agent_id": {"e2e-agent"}, "session_id": {"ws-1"}}

	code, body := e.mustDo(http.MethodGet, "/workspace/status?"+q.Encode(), nil)
	mustStatus(t, code, http.StatusOK, body)
	var st struct {
		Dir string `json:"dir"`
	}
	e.decode(body, &st)
	if st.Dir == "" {
		t.Fatalf("empty workspace dir: %s", body)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, "hello.txt"), []byte("workspace-ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, body = e.mustDo(http.MethodGet, "/workspace/list_dir?"+q.Encode(), nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "hello.txt") {
		t.Fatalf("list_dir: %s", body)
	}

	q.Set("path", "hello.txt")
	code, body = e.mustDo(http.MethodGet, "/workspace/read_file?"+q.Encode(), nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "workspace-ok") {
		t.Fatalf("read_file: %s", body)
	}

	q.Set("path", "../secret.txt")
	code, body = e.mustDo(http.MethodGet, "/workspace/read_file?"+q.Encode(), nil)
	if code == http.StatusOK {
		t.Fatalf("traversal must not succeed: %s", body)
	}
}

func TestE2E_ControlPlaneGoalLifecycleAndTenantFilter(t *testing.T) {
	e := newEnv(t, nil)

	code, body := e.mustDo(http.MethodPost, "/api/v1/controlplane/goals", map[string]any{
		"objective": "ship v2.6 e2e coverage",
	})
	mustStatus(t, code, http.StatusCreated, body)
	var g struct {
		ID          string `json:"id"`
		OwnerUserID string `json:"owner_user_id"`
	}
	e.decode(body, &g)
	if g.ID == "" || g.OwnerUserID != e.userID {
		t.Fatalf("goal: %+v body=%s", g, body)
	}

	code, body = e.mustDo(http.MethodGet, "/api/v1/controlplane/goals/"+g.ID+"/should-run", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "should_run") && !contain(body, "ShouldRun") && !contain(body, "state") {
		t.Fatalf("should-run: %s", body)
	}

	aliceTok := e.token
	e.registerAndLogin("eve")
	code, body = e.mustDo(http.MethodGet, "/api/v1/controlplane/goals/"+g.ID, nil)
	if code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("cross-tenant get goal: want 403/404 got %d %s", code, body)
	}

	code, body = e.do(http.MethodGet, "/api/v1/controlplane/goals", nil, aliceTok)
	mustStatus(t, code, http.StatusOK, body)
	var list struct {
		Goals []json.RawMessage `json:"goals"`
	}
	e.decode(body, &list)
	if len(list.Goals) == 0 {
		t.Fatalf("alice should still see her goal: %s", body)
	}
}
