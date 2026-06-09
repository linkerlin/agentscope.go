package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/service"
)

func TestServer_WithRegistry(t *testing.T) {
	a := &mockAgent{name: "test"}
	srv := NewServer(a)
	reg := NewAgentRegistry()
	srv.WithRegistry(reg)
	if srv.registry != reg {
		t.Fatal("expected registry to be set")
	}
}

func TestServer_WithSessionManager(t *testing.T) {
	a := &mockAgent{name: "test"}
	srv := NewServer(a)
	sm := NewSessionManager()
	srv.WithSessionManager(sm)
	if srv.sessionMgr != sm {
		t.Fatal("expected session manager to be set")
	}
}

func TestServer_WithBackgroundTaskManager(t *testing.T) {
	a := &mockAgent{name: "test"}
	srv := NewServer(a)
	btm := NewBackgroundTaskManager(NewAgentRegistry(), nil)
	srv.WithBackgroundTaskManager(btm)
	if srv.backgroundTaskMgr != btm {
		t.Fatal("expected background task manager to be set")
	}
}

func TestServer_ResolveAgent_Default(t *testing.T) {
	a := &mockAgent{name: "default"}
	srv := NewServer(a)
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	got, err := srv.resolveAgent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != a {
		t.Fatal("expected default agent")
	}
}

func TestServer_ResolveAgent_FromRegistry(t *testing.T) {
	defaultAgent := &mockAgent{name: "default"}
	srv := NewServer(defaultAgent)
	custom := &mockAgent{name: "custom"}
	reg := NewAgentRegistry()
	reg.Register("custom-1", custom)
	srv.WithRegistry(reg)

	req := httptest.NewRequest(http.MethodGet, "/chat?agent_id=custom-1", nil)
	got, err := srv.resolveAgent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatal("expected custom agent from registry")
	}
}

func TestServer_ResolveAgent_MissingRegistry(t *testing.T) {
	defaultAgent := &mockAgent{name: "default"}
	srv := NewServer(defaultAgent)

	req := httptest.NewRequest(http.MethodGet, "/chat?agent_id=missing", nil)
	_, err := srv.resolveAgent(req, "")
	if err == nil {
		t.Fatal("expected error when agent_id specified but no registry")
	}
}

func TestServer_ScheduleRoutes_NotConfigured(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	srv.RegisterScheduleRoutes()

	body, _ := json.Marshal(scheduleRequest{ID: "j1", AgentID: "a1", CronExpr: "* * * * *"})
	req := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestServer_ScheduleCreateAndDelete(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("a1", &smMockAgent{
		events: []event.AgentEvent{
			event.NewReplyStart("r1", "mock"),
			event.NewReplyEnd("r1", "mock"),
		},
	})
	btm := NewBackgroundTaskManager(reg, nil)

	srv := NewServer(&mockAgent{name: "test"})
	srv.WithBackgroundTaskManager(btm)
	srv.RegisterScheduleRoutes()

	// Create
	body, _ := json.Marshal(scheduleRequest{ID: "j1", AgentID: "a1", CronExpr: "*/5 * * * *", Payload: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp scheduleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "j1" {
		t.Fatalf("expected id j1, got %q", resp.ID)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/schedule/delete?id=j1", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", rr.Code)
	}
}

func TestServer_ScheduleCreate_Validation(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithBackgroundTaskManager(NewBackgroundTaskManager(NewAgentRegistry(), nil))
	srv.RegisterScheduleRoutes()

	cases := []struct {
		body   string
		status int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"id":"j1","agent_id":""}`, http.StatusBadRequest},
		{`{"id":"j1","agent_id":"a1"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader([]byte(c.body)))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != c.status {
			t.Fatalf("expected %d for %s, got %d", c.status, c.body, rr.Code)
		}
	}
}

func TestServer_ScheduleDelete_MissingID(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithBackgroundTaskManager(NewBackgroundTaskManager(NewAgentRegistry(), nil))
	srv.RegisterScheduleRoutes()

	req := httptest.NewRequest(http.MethodDelete, "/schedule/delete", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestServer_ResolveAgent_PostBodyAgentID(t *testing.T) {
	defaultAgent := &mockAgent{name: "default"}
	custom := &mockAgent{name: "custom"}
	reg := NewAgentRegistry()
	reg.Register("custom-1", custom)

	srv := NewServer(defaultAgent)
	srv.WithRegistry(reg)

	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	got, err := srv.resolveAgent(req, "custom-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatal("expected custom agent from post body agent_id")
	}
}

func TestServer_WithStorage_CreatesSessionState(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	st := service.NewMemoryStorage()
	srv.WithStorage(st)
	if srv.storage != st {
		t.Fatal("expected storage to be set")
	}
	if srv.sessionState == nil {
		t.Fatal("expected sessionState to be created")
	}
}
