package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/message"
)

func TestGateway_ChatWS_Success(t *testing.T) {
	stream := []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
	}
	a := &mockAgent{name: "test", stream: stream}
	srv := NewServer(a)

	server := httptest.NewServer(srv)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/chat/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v (status %d)", err, resp.StatusCode)
	}
	defer conn.Close()

	if err := conn.WriteJSON(chatRequest{Text: "hi"}); err != nil {
		t.Fatal(err)
	}

	var ev streamEvent
	if err := conn.ReadJSON(&ev); err != nil {
		t.Fatal(err)
	}
	if ev.Delta != "hello" {
		t.Fatalf("expected delta 'hello', got %q", ev.Delta)
	}

	if err := conn.ReadJSON(&ev); err != nil {
		t.Fatal(err)
	}
	if !ev.Done {
		t.Fatal("expected final done event")
	}
}

func TestGateway_ChatWS_MissingText(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	server := httptest.NewServer(srv)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(chatRequest{Text: ""}); err != nil {
		t.Fatal(err)
	}

	var resp map[string]string
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if _, ok := resp["error"]; !ok {
		t.Fatalf("expected error field, got %v", resp)
	}
}

func TestGateway_ChatWS_MethodNotAllowed(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	server := httptest.NewServer(srv)
	defer server.Close()

	// POST should be rejected before upgrade
	resp, err := http.Post(server.URL+"/chat/ws", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}
