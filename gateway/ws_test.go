package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	resp, err := http.Post(server.URL+"/chat/ws", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestGateway_ChatWS_SessionParam(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test", stream: []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	}})
	server := httptest.NewServer(srv)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/chat/ws?session=my-sess"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for i := 0; i < 50; i++ {
		if srv.SessionCount() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if srv.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", srv.SessionCount())
	}
	conn.Close()
	// Allow unregister to run.
	time.Sleep(50 * time.Millisecond)
	if srv.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions after close, got %d", srv.SessionCount())
	}
}

func TestGateway_BroadcastToRoom(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	server := httptest.NewServer(srv)
	defer server.Close()

	baseWS := strings.Replace(server.URL, "http", "ws", 1) + "/chat/ws?room=room1&session="
	conn1, _, err := websocket.DefaultDialer.Dial(baseWS+"a", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(baseWS+"b", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	// Give registration time.
	time.Sleep(50 * time.Millisecond)

	srv.BroadcastToRoom("room1", map[string]string{"notice": "hello room"})

	var msg1 map[string]string
	if err := conn1.ReadJSON(&msg1); err != nil {
		t.Fatal(err)
	}
	if msg1["notice"] != "hello room" {
		t.Fatalf("conn1 unexpected msg: %v", msg1)
	}

	var msg2 map[string]string
	if err := conn2.ReadJSON(&msg2); err != nil {
		t.Fatal(err)
	}
	if msg2["notice"] != "hello room" {
		t.Fatalf("conn2 unexpected msg: %v", msg2)
	}
}
