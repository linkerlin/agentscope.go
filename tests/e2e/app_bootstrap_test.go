package e2e

import (
	"net/http"
	"strings"
	"testing"

	agentscope "github.com/linkerlin/agentscope.go"
	"github.com/linkerlin/agentscope.go/service"
)

func TestE2E_HealthReportsCurrentVersion(t *testing.T) {
	e := newEnv(t, nil)
	code, body := e.do(http.MethodGet, "/health", nil, "")
	mustStatus(t, code, http.StatusOK, body)
	var h map[string]any
	e.decode(body, &h)
	if h["status"] != "healthy" {
		t.Fatalf("status=%v", h["status"])
	}
	if h["version"] != agentscope.Version {
		t.Fatalf("version=%v want %s", h["version"], agentscope.Version)
	}
	if h["auth"] != "enabled" {
		t.Fatalf("expected auth enabled, got %v", h["auth"])
	}
}

func TestE2E_RegisterLoginMeAndChatStream(t *testing.T) {
	e := newEnv(t, nil)

	code, body := e.mustDo(http.MethodGet, "/api/v1/me", nil)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, e.userID) {
		t.Fatalf("me missing user id: %s", body)
	}

	code, body = e.do(http.MethodPost, "/v2/chat/stream", map[string]string{"text": "ping"}, "")
	mustStatus(t, code, http.StatusUnauthorized, body)

	code, body = e.mustDo(http.MethodPost, "/v2/chat/stream", map[string]string{"text": "ping"})
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "hello ping") {
		t.Fatalf("sse missing reply: %s", body)
	}
	if !contain(body, "event_type") && !contain(strings.ToLower(body), "done") {
		t.Fatalf("sse missing event framing: %s", body)
	}
}

func TestE2E_SessionIsolationAcrossUsers(t *testing.T) {
	e := newEnv(t, nil)
	aliceTok := e.token

	code, body := e.do(http.MethodPost, "/api/v1/sessions", map[string]string{
		"title": "alice-chat", "agent_id": "e2e-agent",
	}, aliceTok)
	mustStatus(t, code, http.StatusCreated, body)
	var sess service.Session
	e.decode(body, &sess)
	if sess.ID == "" {
		t.Fatalf("create session: %s", body)
	}

	e.registerAndLogin("bob")
	bobTok := e.token

	code, body = e.do(http.MethodPost, "/v2/chat/stream", map[string]string{
		"text": "from-bob", "session_id": sess.ID,
	}, bobTok)
	if code != http.StatusNotFound {
		t.Fatalf("bob hitting alice session: want 404 got %d %s", code, body)
	}

	code, body = e.do(http.MethodPost, "/v2/chat/stream", map[string]string{
		"text": "from-alice", "session_id": sess.ID,
	}, aliceTok)
	mustStatus(t, code, http.StatusOK, body)
	if !contain(body, "from-alice") {
		t.Fatalf("alice chat: %s", body)
	}
}
