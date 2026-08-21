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

func TestWorkspaceSafeJoin(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		rel     string
		wantErr bool
	}{
		{rel: ""},
		{rel: "a.txt"},
		{rel: "sub/dir/a.txt"},
		{rel: "./a.txt"},
		{rel: "../escape.txt", wantErr: true},
		{rel: "sub/../../escape.txt", wantErr: true},
		{rel: "/abs/path", wantErr: true},
	}
	for _, c := range cases {
		p, err := workspaceSafeJoin(root, c.rel)
		if c.wantErr {
			if err == nil {
				t.Fatalf("rel %q: expected error, got %q", c.rel, p)
			}
			continue
		}
		if err != nil {
			t.Fatalf("rel %q: unexpected error: %v", c.rel, err)
		}
	}
}

func saveWorkspaceSession(t *testing.T, storage service.Storage, id, agentID, workspaceID string) {
	t.Helper()
	if err := storage.SaveSession(context.Background(), &service.Session{
		ID:          id,
		UserID:      "u1",
		AgentID:     agentID,
		WorkspaceID: workspaceID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
}

func workspaceReq(t *testing.T, srv http.Handler, method, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestWorkspace_FileEndpoints(t *testing.T) {
	root := t.TempDir()
	storage := service.NewMemoryStorage()
	saveWorkspaceSession(t, storage, "s1", "a1", "")

	wsMgr := NewWorkspaceManager(root, "")
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	q := "?agent_id=a1&session_id=s1"

	// list_dir on the fresh (auto-created) workspace root.
	rr := workspaceReq(t, srv, http.MethodGet, "/workspace/list_dir"+q)
	if rr.Code != http.StatusOK {
		t.Fatalf("list_dir: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Drop an artifact into the session workspace and read it back.
	wsDir := filepath.Join(root, "u1", "a1", "s1")
	if err := os.WriteFile(filepath.Join(wsDir, "hello.txt"), []byte("artifact body"), 0o644); err != nil {
		t.Fatal(err)
	}

	rr = workspaceReq(t, srv, http.MethodGet, "/workspace/list_dir"+q+"&path=")
	if rr.Code != http.StatusOK {
		t.Fatalf("list_dir: expected 200, got %d", rr.Code)
	}
	var entries []fileEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "hello.txt" || entries[0].Size != int64(len("artifact body")) {
		t.Fatalf("unexpected entries: %+v", entries)
	}

	rr = workspaceReq(t, srv, http.MethodGet, "/workspace/read_file"+q+"&path=hello.txt")
	if rr.Code != http.StatusOK || rr.Body.String() != "artifact body" {
		t.Fatalf("read_file: got %d %q", rr.Code, rr.Body.String())
	}

	// Traversal and missing paths are rejected.
	if rr = workspaceReq(t, srv, http.MethodGet, "/workspace/read_file"+q+"&path=../../etc/passwd"); rr.Code != http.StatusBadRequest {
		t.Fatalf("traversal: expected 400, got %d", rr.Code)
	}
	if rr = workspaceReq(t, srv, http.MethodGet, "/workspace/read_file"+q+"&path=nope.txt"); rr.Code != http.StatusNotFound {
		t.Fatalf("missing: expected 404, got %d", rr.Code)
	}
	if rr = workspaceReq(t, srv, http.MethodGet, "/workspace/read_file"+q); rr.Code != http.StatusBadRequest {
		t.Fatalf("no path: expected 400, got %d", rr.Code)
	}

	// status reports the working directory.
	rr = workspaceReq(t, srv, http.MethodGet, "/workspace/status"+q)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: expected 200, got %d", rr.Code)
	}
	var st workspaceStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Dir != wsDir {
		t.Fatalf("status dir = %q, want %q", st.Dir, wsDir)
	}
}

func TestWorkspace_SharedAcrossAgents(t *testing.T) {
	root := t.TempDir()
	storage := service.NewMemoryStorage()
	// Two sessions of different agents bound to the same named workspace.
	saveWorkspaceSession(t, storage, "s1", "agentA", "team-ws")
	saveWorkspaceSession(t, storage, "s2", "agentB", "team-ws")

	wsMgr := NewWorkspaceManager(root, "")
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	// Status reveals the shared directory for both sessions.
	rrA := workspaceReq(t, srv, http.MethodGet, "/workspace/status?agent_id=agentA&session_id=s1")
	rrB := workspaceReq(t, srv, http.MethodGet, "/workspace/status?agent_id=agentB&session_id=s2")
	var stA, stB workspaceStatus
	if err := json.Unmarshal(rrA.Body.Bytes(), &stA); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rrB.Body.Bytes(), &stB); err != nil {
		t.Fatal(err)
	}
	if stA.Dir != stB.Dir {
		t.Fatalf("shared dirs differ: %q vs %q", stA.Dir, stB.Dir)
	}
	if want := filepath.Join(root, "u1", "shared", "team-ws"); stA.Dir != want {
		t.Fatalf("shared dir = %q, want %q", stA.Dir, want)
	}

	// A file written by agent A's workspace is visible to agent B.
	if err := os.WriteFile(filepath.Join(stA.Dir, "shared.txt"), []byte("shared data"), 0o644); err != nil {
		t.Fatal(err)
	}
	rr := workspaceReq(t, srv, http.MethodGet, "/workspace/read_file?agent_id=agentB&session_id=s2&path=shared.txt")
	if rr.Code != http.StatusOK || rr.Body.String() != "shared data" {
		t.Fatalf("cross-agent read: got %d %q", rr.Code, rr.Body.String())
	}

	// Path-traversal-shaped workspace ids are sanitized, not escaped.
	sw, err := wsMgr.GetOrCreate(context.Background(), storage, "u1", "agentA", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if sw.Dir() != filepath.Join(root, "u1", "shared", "team-ws") {
		t.Fatalf("unexpected dir: %q", sw.Dir())
	}
}

func TestWorkspace_SharedIDSanitized(t *testing.T) {
	root := t.TempDir()
	storage := service.NewMemoryStorage()
	saveWorkspaceSession(t, storage, "s1", "a1", "../evil")

	wsMgr := NewWorkspaceManager(root, "")
	sw, err := wsMgr.GetOrCreate(context.Background(), storage, "u1", "a1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	// ".." becomes underscores; the dir must stay under root/u1/shared.
	want := filepath.Join(root, "u1", "shared", "__evil")
	if sw.Dir() != want {
		t.Fatalf("dir = %q, want %q", sw.Dir(), want)
	}
}

func TestWorkspace_AgentSkillLibraryAndSelect(t *testing.T) {
	root := t.TempDir()
	skillsRoot := filepath.Join(root, "skills")
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(skillsRoot, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: test\n---\n# " + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	storage := service.NewMemoryStorage()
	saveWorkspaceSession(t, storage, "s1", "a1", "")
	saveWorkspaceSession(t, storage, "s2", "a1", "")

	wsMgr := NewWorkspaceManager(root, skillsRoot)
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	// Add both skills via session s1.
	for _, name := range []string{"alpha", "beta"} {
		body, _ := json.Marshal(addSkillRequest{SkillPath: name})
		req := httptest.NewRequest(http.MethodPost, "/workspace/skill?agent_id=a1&session_id=s1", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("add %s: expected 201, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}

	// Agent library now lists both skills.
	rr := workspaceReq(t, srv, http.MethodGet, "/workspace/agent_skills?agent_id=a1")
	if rr.Code != http.StatusOK {
		t.Fatalf("agent_skills: expected 200, got %d", rr.Code)
	}
	var lib []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib) != 2 {
		t.Fatalf("agent library size = %d, want 2", len(lib))
	}

	// Fresh session s2 starts empty, then selects only "beta".
	rr = workspaceReq(t, srv, http.MethodGet, "/workspace/skill?agent_id=a1&session_id=s2")
	if rr.Code != http.StatusOK || rr.Body.String() != "[]\n" {
		t.Fatalf("s2 initial skills: got %d %q", rr.Code, rr.Body.String())
	}

	selBody, _ := json.Marshal(selectSkillsRequest{Names: []string{"beta"}})
	req := httptest.NewRequest(http.MethodPost, "/workspace/skill/select?agent_id=a1&session_id=s2", bytes.NewReader(selBody))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rrRec := httptest.NewRecorder()
	srv.ServeHTTP(rrRec, req)
	if rrRec.Code != http.StatusOK {
		t.Fatalf("select: expected 200, got %d: %s", rrRec.Code, rrRec.Body.String())
	}

	rr = workspaceReq(t, srv, http.MethodGet, "/workspace/skill?agent_id=a1&session_id=s2")
	var selected []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0]["Name"] != "beta" {
		t.Fatalf("s2 selection = %+v, want only beta", selected)
	}

	// Unknown skill names are rejected.
	selBody, _ = json.Marshal(selectSkillsRequest{Names: []string{"gamma"}})
	req = httptest.NewRequest(http.MethodPost, "/workspace/skill/select?agent_id=a1&session_id=s2", bytes.NewReader(selBody))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rrRec = httptest.NewRecorder()
	srv.ServeHTTP(rrRec, req)
	if rrRec.Code != http.StatusBadRequest {
		t.Fatalf("select unknown: expected 400, got %d", rrRec.Code)
	}
}
