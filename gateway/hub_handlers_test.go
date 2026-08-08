package gateway

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/hub/builtin"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// mockHub is an in-memory Hub for tests.
type mockHub struct{}

func (mockHub) ID() string          { return "mock" }
func (mockHub) DisplayName() string { return "Mock Hub" }

func (mockHub) ListMCPCards(ctx context.Context, query string, cursor, limit int) ([]hub.MCPCard, int, error) {
	cards := []hub.MCPCard{
		{Card: hub.Card{ID: "ghost", Name: "ghost-mcp", Description: "missing binary"}, Spec: mcp.ServerSpec{Name: "ghost", Command: "no-such-binary-xyz"}},
	}
	page, next := hub.Page(cards, cursor, limit)
	return page, next, nil
}

func (mockHub) ListSkillCards(ctx context.Context, query string, cursor, limit int) ([]hub.SkillCard, int, error) {
	return nil, -1, nil
}

func newHubTestServer(t *testing.T, skillArchiveURL string) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	// skill card with a real archive URL (local httptest file)
	skillCard := `[{"id":"demo-skill","name":"Demo","description":"d","archive_url":"` + skillArchiveURL + `"}]`
	os.WriteFile(filepath.Join(dir, "skills.json"), []byte(skillCard), 0o644)
	// mcp card catalog
	os.WriteFile(filepath.Join(dir, "mcps.json"), []byte(`[{"id":"ghost","name":"Ghost","description":"missing binary","spec":{"name":"ghost","command":"no-such-binary-xyz"}}]`), 0o644)

	fsHub, err := builtin.NewFSHub(dir, "demo", "Demo Hub", "")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(&mockAgent{})
	srv.WithHubs(fsHub, mockHub{})
	srv.RegisterHubRoutes()
	return srv, dir
}

func makeSkillZip(t *testing.T) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("skill/SKILL.md")
	w.Write([]byte("# Demo skill"))
	zw.Close()
	return buf.Bytes()
}

func TestHubRoutes_ListHubs(t *testing.T) {
	srv, _ := newHubTestServer(t, "")
	req := httptest.NewRequest("GET", "/api/v1/hubs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Hubs []struct {
			ID string `json:"id"`
		} `json:"hubs"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Hubs) != 2 {
		t.Fatalf("expected 2 hubs, got %d", len(resp.Hubs))
	}
}

func TestHubRoutes_ListMCPCards(t *testing.T) {
	srv, _ := newHubTestServer(t, "")
	req := httptest.NewRequest("GET", "/api/v1/hubs/demo/mcps", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Cards      []hub.MCPCard `json:"cards"`
		NextCursor int           `json:"next_cursor"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Cards) != 1 || resp.Cards[0].ID != "ghost" {
		t.Fatalf("cards: %+v", resp.Cards)
	}
}

func TestHubRoutes_MCPInstallMissingBinary(t *testing.T) {
	srv, _ := newHubTestServer(t, "")
	req := httptest.NewRequest("POST", "/api/v1/hubs/demo/mcps/ghost/install", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// gracefully reports 422 (binary not installed) — not a 500
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d %s", w.Code, w.Body.String())
	}
}

func TestHubRoutes_SkillInstall(t *testing.T) {
	// serve the skill zip over httptest
	zipData := makeSkillZip(t)
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipData)
	}))
	defer fileSrv.Close()

	srv, _ := newHubTestServer(t, fileSrv.URL+"/skill.zip")
	dest := filepath.Join(t.TempDir(), "skills")
	req := httptest.NewRequest("POST", "/api/v1/hubs/demo/skills/demo-skill/install?dest="+dest, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("install status = %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "skill", "SKILL.md")); err != nil {
		t.Fatal("SKILL.md not installed")
	}
}

func TestHubRoutes_NotFound(t *testing.T) {
	srv, _ := newHubTestServer(t, "")
	req := httptest.NewRequest("GET", "/api/v1/hubs/ghost-hub/mcps", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	// card not found
	req2 := httptest.NewRequest("POST", "/api/v1/hubs/demo/skills/no-such-card/install", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing card, got %d", w2.Code)
	}
}

func TestHubRoutes_NoHubsNoRoutes(t *testing.T) {
	srv := NewServer(&mockAgent{})
	srv.RegisterHubRoutes()
	req := httptest.NewRequest("GET", "/api/v1/hubs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without hubs, got %d", w.Code)
	}
}
