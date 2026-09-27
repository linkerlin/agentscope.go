package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
	"github.com/linkerlin/agentscope.go/service"
	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// env is a production-shaped gateway: NewApp + RegisterAppRoutes + real HTTP.
type env struct {
	t       *testing.T
	srv     *gateway.Server
	http    *httptest.Server
	jwt     *service.JWTAuthenticator
	storage service.Storage
	wsDir   string
	userID  string
	token   string
	agent   *streamAgent
}

type streamAgent struct {
	mu       sync.Mutex
	lastText string
	delay    time.Duration
	name     string
}

func (a *streamAgent) Name() string {
	if a.name != "" {
		return a.name
	}
	return "e2e-agent"
}
func (a *streamAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok:" + msg.GetTextContent()).Build(), nil
}
func (a *streamAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	out, err := a.Call(ctx, msg)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- out
	close(ch)
	return ch, nil
}
func (a *streamAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return a.Call(ctx, msg)
}
func (a *streamAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	a.mu.Lock()
	a.lastText = msg.GetTextContent()
	delay := a.delay
	a.mu.Unlock()
	ch := make(chan event.AgentEvent, 8)
	go func() {
		defer close(ch)
		if delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		text := "hello " + msg.GetTextContent()
		ch <- event.NewTextBlockDelta("e2e", 0, text)
		ch <- event.NewReplyEnd("e2e", a.Name())
	}()
	return ch, nil
}
func (a *streamAgent) LoadState(*agent.AgentState) error                   { return nil }
func (a *streamAgent) SaveState() (*agent.AgentState, error)               { return nil, nil }
func (a *streamAgent) InjectEvent(context.Context, event.AgentEvent) error { return nil }

var _ agent.V2Agent = (*streamAgent)(nil)

type stubEmbedder struct{}

func (stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	v := make([]float32, 4)
	for i := range v {
		v[i] = float32((len(text) + i*3) % 17)
	}
	return v, nil
}

type memHub struct{ id, name string }

func (h memHub) ID() string          { return h.id }
func (h memHub) DisplayName() string { return h.name }
func (h memHub) ListMCPCards(context.Context, string, int, int) ([]hub.MCPCard, int, error) {
	return []hub.MCPCard{{
		Card: hub.Card{ID: "echo", Name: "echo-mcp", Description: "demo"},
		Spec: mcpserver.ServerSpec{Name: "echo", Command: "echo"},
	}}, -1, nil
}
func (h memHub) ListSkillCards(context.Context, string, int, int) ([]hub.SkillCard, int, error) {
	return []hub.SkillCard{{
		Card:       hub.Card{ID: "demo-skill", Name: "demo", Description: "skill"},
		ArchiveURL: "http://127.0.0.1/missing.zip",
	}}, -1, nil
}

func newEnv(t *testing.T, extra func(*gateway.AppConfig, *streamAgent)) *env {
	t.Helper()
	wsDir := t.TempDir()
	blobDir := t.TempDir()
	bs, err := blob.NewLocalBlobStore(blobDir)
	if err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry(parser.NewTextParser())
	ch := chunker.NewApproxTokenChunker()
	mgr := kb.NewCollectionPerKBManager(kb.NewInMemoryVectorStore(), func(string) (kb.Embedder, error) {
		return stubEmbedder{}, nil
	})
	kbSvc := gateway.NewKBService(mgr, bs, reg, ch)

	ag := &streamAgent{}
	jwt := service.NewJWTAuthenticator([]byte("e2e-secret"), "e2e-issuer")
	storage := service.NewMemoryStorage()
	cfg := gateway.AppConfig{
		Agent:             ag,
		Storage:           storage,
		Authenticator:     jwt,
		JWTAuth:           jwt,
		WorkspaceBaseDir:  wsDir,
		KBService:         kbSvc,
		ControlPlane:      controlplane.NewKernel(nil, nil, nil),
		AutoStandardTools: false,
	}
	if extra != nil {
		extra(&cfg, ag)
	}
	srv := gateway.NewApp(cfg)
	srv.WithHubs(memHub{id: "demo", name: "Demo Hub"})
	srv.RegisterAppRoutes(jwt)
	srv.Start()
	hs := httptest.NewServer(srv)
	t.Cleanup(func() {
		hs.Close()
		_ = srv.Close()
	})
	e := &env{t: t, srv: srv, http: hs, jwt: jwt, storage: storage, wsDir: wsDir, agent: ag}
	e.registerAndLogin("e2e-user")
	return e
}

func (e *env) registerAndLogin(name string) {
	e.t.Helper()
	code, body := e.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"name": name}, "")
	if code != http.StatusCreated {
		e.t.Fatalf("register: %d %s", code, body)
	}
	var reg struct {
		UserID string `json:"user_id"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal([]byte(body), &reg); err != nil {
		e.t.Fatal(err)
	}
	code, body = e.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"api_key": reg.APIKey}, "")
	if code != http.StatusOK {
		e.t.Fatalf("login: %d %s", code, body)
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &login); err != nil {
		e.t.Fatal(err)
	}
	e.userID = reg.UserID
	e.token = login.Token
}

func (e *env) do(method, path string, payload any, token string) (int, string) {
	e.t.Helper()
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			e.t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.http.URL+path, rdr)
	if err != nil {
		e.t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *env) mustDo(method, path string, payload any) (int, string) {
	e.t.Helper()
	return e.do(method, path, payload, e.token)
}

func (e *env) decode(body string, dest any) {
	e.t.Helper()
	if err := json.Unmarshal([]byte(body), dest); err != nil {
		e.t.Fatalf("decode %s: %v", body, err)
	}
}

func writeWorkspaceFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contain(hay, needle string) bool { return strings.Contains(hay, needle) }

func mustStatus(t *testing.T, got, want int, body string) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d want %d body=%s", got, want, body)
	}
}

func waitFor(t *testing.T, d time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", d)
}
