package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// BenchmarkGateway_ChatConcurrent measures throughput under high concurrency.
func BenchmarkGateway_ChatConcurrent(b *testing.B) {
	a := &mockAgent{name: "bench", resp: message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build()}
	srv := NewServer(a)
	body, _ := json.Marshal(chatRequest{Text: "ping"})

	for _, concurrency := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			var idx int64
			b.ResetTimer()
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := atomic.AddInt64(&idx, 1)
					req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
					rr := httptest.NewRecorder()
					srv.ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						b.Fatalf("unexpected status %d (iter %d)", rr.Code, i)
					}
				}
			})
		})
	}
}

// BenchmarkGateway_HealthConcurrent measures health endpoint throughput.
func BenchmarkGateway_HealthConcurrent(b *testing.B) {
	srv := NewServer(&mockAgent{name: "bench"})

	for _, concurrency := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req := httptest.NewRequest(http.MethodGet, "/health", nil)
					rr := httptest.NewRecorder()
					srv.ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						b.Fatalf("unexpected status %d", rr.Code)
					}
				}
			})
		})
	}
}

// BenchmarkGateway_ChatMixed simulates a mixed workload of chat and health requests.
func BenchmarkGateway_ChatMixed(b *testing.B) {
	a := &mockAgent{name: "bench", resp: message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build()}
	srv := NewServer(a)
	chatBody, _ := json.Marshal(chatRequest{Text: "ping"})

	b.ResetTimer()
	b.SetParallelism(50)
	b.RunParallel(func(pb *testing.PB) {
		var counter int64
		for pb.Next() {
			counter++
			if counter%5 == 0 {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				rr := httptest.NewRecorder()
				srv.ServeHTTP(rr, req)
			} else {
				req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatBody))
				rr := httptest.NewRecorder()
				srv.ServeHTTP(rr, req)
			}
		}
	})
}

// BenchmarkGateway_ChatStreamConcurrent measures SSE streaming under concurrency.
func BenchmarkGateway_ChatStreamConcurrent(b *testing.B) {
	stream := []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("he").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("llo").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("!").Build(),
	}
	a := &mockAgent{name: "bench", stream: stream}
	srv := NewServer(a)
	body, _ := json.Marshal(chatRequest{Text: "hi"})

	for _, concurrency := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(body))
					rr := httptest.NewRecorder()
					srv.ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						b.Fatalf("unexpected status %d", rr.Code)
					}
				}
			})
		})
	}
}

// BenchmarkGateway_RealServer starts a real HTTP server and benchmarks end-to-end
// latency including TCP connection + HTTP parsing overhead.
func BenchmarkGateway_RealServer(b *testing.B) {
	a := &mockAgent{name: "bench", resp: message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build()}
	srv := NewServer(a)
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	chatBody, _ := json.Marshal(chatRequest{Text: "ping"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Post(ts.URL+"/chat", "application/json", bytes.NewReader(chatBody))
			if err != nil {
				b.Fatalf("request error: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status %d", resp.StatusCode)
			}
		}
	})
}

// BenchmarkGateway_RealServerHealth measures health check on a real server.
func BenchmarkGateway_RealServerHealth(b *testing.B) {
	srv := NewServer(&mockAgent{name: "bench"})
	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(ts.URL + "/health")
			if err != nil {
				b.Fatalf("request error: %v", err)
			}
			resp.Body.Close()
		}
	})
}

// SessionCreate*/BroadcastRoom benchmarks were removed with the WS registry:
// the map/lock internals they poked moved to gateway/sessionapi (16.2), and
// they never exercised real WebSocket behavior.
