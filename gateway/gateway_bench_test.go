package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// BenchmarkGateway_Chat measures the latency of a single non-streaming chat request.
func BenchmarkGateway_Chat(b *testing.B) {
	a := &mockAgent{name: "bench", resp: message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build()}
	srv := NewServer(a)
	body, _ := json.Marshal(chatRequest{Text: "ping"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rr.Code)
		}
	}
}

// BenchmarkGateway_ChatStream measures the latency of an SSE streaming chat request.
func BenchmarkGateway_ChatStream(b *testing.B) {
	stream := []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("he").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("llo").Build(),
	}
	a := &mockAgent{name: "bench", stream: stream}
	srv := NewServer(a)
	body, _ := json.Marshal(chatRequest{Text: "hi"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rr.Code)
		}
	}
}
