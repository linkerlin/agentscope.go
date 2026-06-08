package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// mockV2Agent streams a short deterministic reply for HTTP e2e tests.
type mockV2Agent struct{}

func (m *mockV2Agent) Name() string { return "mock-v2" }

func (m *mockV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *mockV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func (m *mockV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}

func (m *mockV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 4)
	replyID := "mock-reply-1"
	ch <- event.NewReplyStart(replyID, m.Name())
	ch <- event.NewTextBlockDelta(replyID, 0, "tenant-e2e-ok")
	ch <- event.NewReplyEnd(replyID, m.Name())
	close(ch)
	return ch, nil
}

func (m *mockV2Agent) LoadState(state *agent.AgentState) error { return nil }
func (m *mockV2Agent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestE2E_MultiTenantWorkspace(t *testing.T) {
	srv, _ := buildGateway(&mockV2Agent{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// 1. Register tenant
	regBody := `{"name":"Alice"}`
	regResp := postJSON(t, ts.URL+"/api/v1/auth/register", regBody, "")
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %s", regResp.StatusCode, readBody(regResp))
	}
	var reg registerResponse
	decodeJSON(t, regResp, &reg)
	if reg.UserID == "" || reg.APIKey == "" {
		t.Fatalf("missing register fields: %#v", reg)
	}

	// 2. /me with API key
	meResp := getAuth(t, ts.URL+"/api/v1/me", reg.APIKey)
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("/me: %d %s", meResp.StatusCode, readBody(meResp))
	}

	// 3. Login -> JWT
	loginBody := `{"user_id":"` + reg.UserID + `"}`
	loginResp := postJSON(t, ts.URL+"/api/v1/auth/login", loginBody, "")
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: %d %s", loginResp.StatusCode, readBody(loginResp))
	}
	var login loginResponse
	decodeJSON(t, loginResp, &login)
	if login.Token == "" {
		t.Fatal("expected jwt token")
	}

	// 4. Create session (JWT)
	sessResp := postJSON(t, ts.URL+"/api/v1/sessions", `{"title":"demo"}`, "Bearer "+login.Token)
	if sessResp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: %d %s", sessResp.StatusCode, readBody(sessResp))
	}
	var sess map[string]any
	decodeJSON(t, sessResp, &sess)
	sessionID, _ := sess["id"].(string)
	if sessionID == "" {
		t.Fatalf("missing session id: %#v", sess)
	}

	// 5. Store credential
	credResp := postJSON(t, ts.URL+"/api/v1/credentials",
		`{"provider":"dashscope","label":"default","value":"sk-demo"}`,
		reg.APIKey,
	)
	if credResp.StatusCode != http.StatusCreated {
		t.Fatalf("credential: %d %s", credResp.StatusCode, readBody(credResp))
	}

	// 6. V2 chat stream
	chatBody := `{"text":"hello","session_id":"` + sessionID + `"}`
	streamResp := postJSON(t, ts.URL+"/v2/chat/stream", chatBody, reg.APIKey)
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("chat stream: %d %s", streamResp.StatusCode, readBody(streamResp))
	}
	streamText := readBody(streamResp)
	if !strings.Contains(streamText, "tenant-e2e-ok") {
		t.Fatalf("expected stream content, got:\n%s", streamText)
	}

	// 7. Unauthenticated V2 should fail
	unauth := postJSON(t, ts.URL+"/v2/chat/stream", chatBody, "")
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", unauth.StatusCode)
	}
}

type registerResponse struct {
	UserID string `json:"user_id"`
	APIKey string `json:"api_key"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func postJSON(t *testing.T, url, body, auth string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func getAuth(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func setAuth(req *http.Request, auth string) {
	if auth == "" {
		return
	}
	if strings.HasPrefix(auth, "Bearer ") {
		req.Header.Set("Authorization", auth)
	} else {
		req.Header.Set("X-API-Key", auth)
	}
}

func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}
