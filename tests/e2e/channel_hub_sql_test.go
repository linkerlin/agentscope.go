package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/service"
)

type recChannel struct {
	id   string
	mu   sync.Mutex
	sent []string
}

func (r *recChannel) ID() string { return r.id }
func (r *recChannel) Start(context.Context, func(channel.ChannelEvent) error) error {
	return nil
}
func (r *recChannel) SendText(_ context.Context, _, text string) error {
	r.mu.Lock()
	r.sent = append(r.sent, text)
	r.mu.Unlock()
	return nil
}
func (r *recChannel) Close() error { return nil }

func TestE2E_ChannelWebhookToAgentReply(t *testing.T) {
	e := newEnv(t, nil)
	wh := channel.NewWebhookChannel("wh-e2e")
	reg := channel.NewRegistry()
	if err := reg.Register(wh); err != nil {
		t.Fatal(err)
	}
	agents := gateway.NewAgentRegistry()
	agents.Register("echo-bot", e.agent)
	rec := &recChannel{id: "wh-e2e"}
	runner := gateway.NewChannelRunner(agents, gateway.NewSessionManager()).
		WithLookup(func(string) channel.Channel { return rec })
	router := channel.NewChatRouter(channel.RouteTable{
		ChannelID: "wh-e2e",
		Bindings:  []channel.Binding{{ChatIDPrefix: "dev-", AgentID: "echo-bot", SessionPrefix: "dev-"}},
	})
	gw := channel.NewGateway(router, runner)
	e.srv.WithChannelGateway(reg, gw)
	e.srv.RegisterChannelRoutes()
	e.srv.StartChannels()
	time.Sleep(30 * time.Millisecond)

	code, body := e.do(http.MethodPost, "/api/v1/channels/wh-e2e/webhook", map[string]any{
		"chat_id": "dev-42", "user_id": "u1", "text": "webhook-hi",
	}, "")
	mustStatus(t, code, http.StatusOK, body)

	waitFor(t, 2*time.Second, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.sent) > 0
	})
	rec.mu.Lock()
	got := rec.sent[0]
	rec.mu.Unlock()
	if !contain(got, "webhook-hi") && !contain(got, "hello") {
		t.Fatalf("channel reply %q", got)
	}

	code, body = e.mustDo(http.MethodGet, "/api/v1/channels", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "wh-e2e") {
		t.Fatalf("list channels: %s", body)
	}
}

func TestE2E_HubBrowse(t *testing.T) {
	e := newEnv(t, nil)
	code, body := e.mustDo(http.MethodGet, "/api/v1/hubs", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "demo") {
		t.Fatalf("hubs: %s", body)
	}
	code, body = e.mustDo(http.MethodGet, "/api/v1/hubs/demo/mcps", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "echo-mcp") {
		t.Fatalf("mcps: %s", body)
	}
	code, body = e.mustDo(http.MethodGet, "/api/v1/hubs/demo/skills", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "demo-skill") {
		t.Fatalf("skills: %s", body)
	}
}

func TestE2E_SQLStorageSurvivesReopen(t *testing.T) {
	db := filepath.Join(t.TempDir(), "e2e.db")
	ctx := context.Background()
	st, err := service.NewSQLStorage(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	jwt := service.NewJWTAuthenticator([]byte("sql-secret"), "sql-issuer")
	srv := gateway.NewApp(gateway.AppConfig{
		Agent:         &streamAgent{},
		Storage:       st,
		Authenticator: jwt,
		JWTAuth:       jwt,
	})
	srv.RegisterAppRoutes(jwt)
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)

	tmp := &env{t: t, http: hs}
	code, body := tmp.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"name": "sql-user"}, "")
	mustStatus(t, code, http.StatusCreated, body)
	var reg struct {
		UserID string `json:"user_id"`
	}
	tmp.decode(body, &reg)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := service.NewSQLStorage(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.Close() })
	u, err := st2.GetUser(ctx, reg.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "sql-user" {
		t.Fatalf("reopened user name=%q", u.Name)
	}
}
