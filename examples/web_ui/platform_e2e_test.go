package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/hub/builtin"
	"github.com/linkerlin/agentscope.go/service"
)

// TestWebUI_PlatformApis locks the 21.1 console capabilities end to end
// against a real server assembly: hub browse + install (with the 18.7
// configuration-template path), channel listing, and per-session workspace
// git status. The static Platform view talks to exactly these routes.
//
// The hub cards below point at "definitely-missing-binary": the template
// value sent by the install test is a placeholder typed at the console
// prompt, never a credential (the binary is absent, so no connection is
// ever attempted).
func TestWebUI_PlatformApis(t *testing.T) {
	// ── assemble: demo hub + webhook channel + workspace manager + storage ──
	hubDir := t.TempDir()
	writeHubCard(t, hubDir, "mcps.json", `[
	  {"id":"plain","name":"Plain MCP","description":"no template","spec":{"name":"plain","command":"definitely-missing-binary"}},
	  {"id":"templated","name":"Templated MCP","description":"needs a region","required_env":["SEARCH_REGION"],"spec":{"name":"templated","command":"definitely-missing-binary","env":{"SEARCH_REGION":"${SEARCH_REGION}"}}}
	]`)
	writeHubCard(t, hubDir, "skills.json", `[
	  {"id":"helper","name":"Helper","description":"demo skill","archive_url":"https://example.invalid/helper.zip"}
	]`)
	demoHub, err := builtin.NewFSHub(hubDir, "demo", "Demo Hub", "")
	if err != nil {
		t.Fatal(err)
	}

	storage := service.NewMemoryStorage()
	ag := newDemoAgent()
	srv := gateway.NewServer(ag).
		WithStorage(storage).
		WithSessionManager(gateway.NewSessionManager().WithStorage(storage)).
		WithWorkspaceManager(gateway.NewWorkspaceManager(t.TempDir(), "")).
		WithHubs(demoHub)
	srv.RegisterHubRoutes()
	srv.RegisterWorkspaceRoutes()

	reg := channel.NewRegistry()
	reg.Register(channel.NewWebhookChannel("wh-demo"))
	srv.WithChannelGateway(reg, channel.NewGateway(webhookRouter{}, gateway.NewChannelRunner(nil, nil)))
	srv.RegisterChannelRoutes()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	// ── hub browse ──
	listRec := doGet(t, ts, "/api/v1/hubs")
	if listRec.Code != http.StatusOK {
		t.Fatalf("hubs: %d body=%s", listRec.Code, listRec.Body.String())
	}
	var hubs struct {
		Hubs []struct{ ID string } `json:"hubs"`
	}
	_ = json.Unmarshal(listRec.Body.Bytes(), &hubs)
	if len(hubs.Hubs) != 1 || hubs.Hubs[0].ID != "demo" {
		t.Fatalf("hubs = %+v", hubs.Hubs)
	}

	mcpRec := doGet(t, ts, "/api/v1/hubs/demo/mcps")
	if mcpRec.Code != http.StatusOK || !strings.Contains(mcpRec.Body.String(), "templated") {
		t.Fatalf("mcps: %d body=%s", mcpRec.Code, mcpRec.Body.String())
	}
	skillRec := doGet(t, ts, "/api/v1/hubs/demo/skills")
	if skillRec.Code != http.StatusOK || !strings.Contains(skillRec.Body.String(), "helper") {
		t.Fatalf("skills: %d body=%s", skillRec.Code, skillRec.Body.String())
	}

	// ── hub install: template values are validated BEFORE any process spawn ──
	// Missing values on a required_env card → 400 listing the variable.
	missRec := doPost(t, ts, "/api/v1/hubs/demo/mcps/templated/install", `{}`)
	if missRec.Code != http.StatusBadRequest {
		t.Fatalf("install missing values: expected 400, got %d body=%s", missRec.Code, missRec.Body.String())
	}
	if !strings.Contains(missRec.Body.String(), "SEARCH_REGION") {
		t.Fatalf("install error should name the missing variable: %s", missRec.Body.String())
	}
	// With values supplied the install proceeds to the (gracefully skipped)
	// connection attempt — a missing binary must not 5xx.
	valRec := doPost(t, ts, "/api/v1/hubs/demo/mcps/templated/install", `{"values":{"SEARCH_REGION":"value-typed-at-console-prompt"}}`)
	if valRec.Code >= 500 {
		t.Fatalf("install with values: unexpected 5xx %d body=%s", valRec.Code, valRec.Body.String())
	}

	// ── channel listing ──
	chRec := doGet(t, ts, "/api/v1/channels")
	if chRec.Code != http.StatusOK || !strings.Contains(chRec.Body.String(), `"wh-demo"`) {
		t.Fatalf("channels: %d body=%s", chRec.Code, chRec.Body.String())
	}

	// ── workspace git status: needs a stored session owned by the caller ──
	// Unknown session → 404 (no existence leak, 22.2).
	anonRec := doGet(t, ts, "/workspace/status?agent_id=web-ui&session_id=no-such-session")
	if anonRec.Code != http.StatusNotFound {
		t.Fatalf("unknown session workspace: expected 404, got %d body=%s", anonRec.Code, anonRec.Body.String())
	}
	ctx := context.Background()
	if err := storage.SaveSession(ctx, &service.Session{ID: "sess-ws", UserID: "", AgentID: "web-ui"}); err != nil {
		t.Fatal(err)
	}
	wsRec := doGet(t, ts, "/workspace/status?agent_id=web-ui&session_id=sess-ws")
	if wsRec.Code != http.StatusOK {
		t.Fatalf("workspace status: %d body=%s", wsRec.Code, wsRec.Body.String())
	}
	var st struct {
		Dir       string `json:"dir"`
		IsGitRepo bool   `json:"is_git_repo"`
	}
	if err := json.Unmarshal(wsRec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Dir == "" {
		t.Fatalf("workspace status missing dir: %s", wsRec.Body.String())
	}
	// A temp dir is not a git repo — the endpoint degrades gracefully.
	if st.IsGitRepo {
		t.Fatalf("temp workspace reported as git repo: %s", wsRec.Body.String())
	}
}

// webhookRouter routes every channel event to the demo agent.
type webhookRouter struct{}

func (webhookRouter) Resolve(_ context.Context, ev channel.ChannelEvent) (string, string, error) {
	return "web-ui", "chan-" + ev.ChannelID + "-" + ev.ChatID, nil
}

func writeHubCard(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func doGet(t *testing.T, ts *httptest.Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	_, _ = rec.Body.Write([]byte(readBody(resp)))
	rec.Code = resp.StatusCode
	return rec
}

func doPost(t *testing.T, ts *httptest.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	_, _ = rec.Body.Write([]byte(readBody(resp)))
	rec.Code = resp.StatusCode
	return rec
}

// hub compile-time guards: the demo assembly relies on these shapes.
var (
	_ hub.Hub = (*builtin.FSHub)(nil)
)
