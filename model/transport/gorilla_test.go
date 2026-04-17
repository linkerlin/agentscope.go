package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func echoServer(t *testing.T) *httptest.Server {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if err := ws.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	}))
}

func TestGorillaWebSocketTransport_ConnectAndEcho(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	tr := NewGorillaWebSocketTransport()
	ctx := context.Background()

	conn, err := tr.Connect(ctx, wsURL, map[string]string{"X-Custom": "value"})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	if !conn.IsOpen() {
		t.Fatal("expected connection to be open")
	}

	msg := []byte("hello websocket")
	if err := conn.Send(ctx, msg); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	recvCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := conn.Receive(recvCtx)
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if resp.Type != MessageText {
		t.Fatalf("expected text message, got %d", resp.Type)
	}
	if string(resp.Data) != string(msg) {
		t.Fatalf("expected %q, got %q", msg, resp.Data)
	}
}

func TestGorillaWebSocketTransport_BinaryEcho(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	tr := NewGorillaWebSocketTransport()
	conn, err := tr.Connect(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	data := []byte{0x01, 0x02, 0x03}
	if err := conn.SendBinary(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	resp, err := conn.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Type != MessageBinary {
		t.Fatalf("expected binary message, got %d", resp.Type)
	}
	if string(resp.Data) != string(data) {
		t.Fatalf("expected %v, got %v", data, resp.Data)
	}
}

func TestGorillaWebSocketTransport_ReceiveContextCancel(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	tr := NewGorillaWebSocketTransport()
	conn, err := tr.Connect(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = conn.Receive(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled receive")
	}
}

func TestGorillaWebSocketTransport_ConnectFailure(t *testing.T) {
	tr := NewGorillaWebSocketTransport()
	_, err := tr.Connect(context.Background(), "ws://localhost:1/nope", nil)
	if err == nil {
		t.Fatal("expected connect error")
	}
	wsErr, ok := err.(*WebSocketTransportException)
	if !ok {
		t.Fatalf("expected WebSocketTransportException, got %T", err)
	}
	if wsErr.Op != "connect" {
		t.Fatalf("expected op=connect, got %s", wsErr.Op)
	}
}

func TestGorillaConn_SendAfterClose(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	tr := NewGorillaWebSocketTransport()
	conn, err := tr.Connect(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if conn.IsOpen() {
		t.Fatal("expected connection closed")
	}
	if err := conn.Send(context.Background(), []byte("x")); err == nil {
		t.Fatal("expected error sending after close")
	}
}
