package a2a

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAuthMiddleware_APIKey 测试 API Key 认证
func TestAuthMiddleware_APIKey(t *testing.T) {
	auth := NewAuthMiddleware()
	auth.AddAPIKey("test-key-123", "test-user")

	// 测试公开路径
	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	rr := httptest.NewRecorder()

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for public path, got %d", rr.Code)
	}

	// 测试未认证请求
	req2 := httptest.NewRequest("POST", "/task/send", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr2.Code)
	}

	// 测试 API Key 认证
	req3 := httptest.NewRequest("POST", "/task/send", nil)
	req3.Header.Set("X-API-Key", "test-key-123")
	rr3 := httptest.NewRecorder()

	var user string
	handler2 := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user = GetAuthUser(r)
		w.WriteHeader(http.StatusOK)
	}))
	handler2.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr3.Code)
	}
	if user != "test-user" {
		t.Errorf("expected user 'test-user', got '%s'", user)
	}
}

// TestAuthMiddleware_Bearer 测试 Bearer Token 认证
func TestAuthMiddleware_Bearer(t *testing.T) {
	auth := NewAuthMiddleware()
	auth.SetBearerValidator(func(token string) (string, error) {
		if token == "valid-token" {
			return "bearer-user", nil
		}
		return "", http.ErrNoCookie
	})

	req := httptest.NewRequest("POST", "/task/send", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	var user string
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user = GetAuthUser(r)
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if user != "bearer-user" {
		t.Errorf("expected user 'bearer-user', got '%s'", user)
	}
}

// TestRateLimiter 测试限流器
func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2, 3) // 每秒2个，突发3个

	key := "test-client"

	// 前3个请求应该成功（突发容量）
	for i := 0; i < 3; i++ {
		if !limiter.Allow(key) {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 第4个请求应该失败（超出突发容量）
	if limiter.Allow(key) {
		t.Error("request 4 should be rate limited")
	}

	// 等待令牌补充
	time.Sleep(600 * time.Millisecond)

	// 应该允许1个新请求（约1个令牌补充）
	if !limiter.Allow(key) {
		t.Error("request after wait should be allowed")
	}
}

// TestRateLimiter_GetRemaining 测试剩余令牌查询
func TestRateLimiter_GetRemaining(t *testing.T) {
	limiter := NewRateLimiter(10, 10)
	key := "test-client"

	// 初始应该有10个
	if limiter.GetRemaining(key) != 10 {
		t.Errorf("expected 10 remaining, got %d", limiter.GetRemaining(key))
	}

	// 使用5个
	for i := 0; i < 5; i++ {
		limiter.Allow(key)
	}

	// 应该剩下5个
	if limiter.GetRemaining(key) != 5 {
		t.Errorf("expected 5 remaining, got %d", limiter.GetRemaining(key))
	}
}

// TestRateLimitMiddleware 测试限流中间件
func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewRateLimiter(1, 1) // 每秒1个，突发1个

	middleware := RateLimitMiddleware(limiter, func(r *http.Request) string {
		return "test-key"
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 第一个请求应该成功
	req1 := httptest.NewRequest("GET", "/test", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr1.Code)
	}

	// 第二个请求应该被限流
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr2.Code)
	}
}

// TestCorsMiddleware 测试 CORS 中间件
func TestCorsMiddleware(t *testing.T) {
	middleware := CorsMiddleware([]string{"https://example.com", "*"})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 测试预检请求
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("expected CORS header")
	}

	// 测试实际请求
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Origin", "https://example.com")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("expected CORS header for GET")
	}
}

// TestLoggingMiddleware 测试日志中间件
func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/task/send", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}
}

// TestChainMiddleware 测试中间件链
func TestChainMiddleware(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "base")
		w.WriteHeader(http.StatusOK)
	})

	handler := ChainMiddleware(base, m1, m2)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expected := []string{"m1-before", "m2-before", "base", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %s, got %s", i, v, order[i])
		}
	}
}

// TestSecureServer 测试安全服务器
func TestSecureServer(t *testing.T) {
	card := AgentCard{
		Name: "test-agent",
		URL:  "http://localhost:8080",
	}
	server := NewSecureServer(card, nil, nil)
	server.auth.AddAPIKey("api-key", "test-user")

	// 测试公开路径
	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for agent card, got %d", rr.Code)
	}

	// 测试未认证请求被限流
	req2 := httptest.NewRequest("POST", "/task/send", nil)
	rr2 := httptest.NewRecorder()
	server.ServeHTTP(rr2, req2)
	// 应该先触发限流（因为匿名用户共享 IP）或认证失败
	if rr2.Code != http.StatusTooManyRequests && rr2.Code != http.StatusUnauthorized {
		t.Errorf("expected 429 or 401, got %d", rr2.Code)
	}
}

// TestWebSocketMessage 测试 WebSocket 消息格式
func TestWebSocketMessage(t *testing.T) {
	msg := WebSocketMessage{
		Type:      "task_update",
		TaskID:    "task-123",
		Data:      map[string]any{"status": "completed"},
		Timestamp: time.Now(),
	}

	if msg.Type != "task_update" {
		t.Errorf("expected type 'task_update', got '%s'", msg.Type)
	}
	if msg.TaskID != "task-123" {
		t.Errorf("expected task_id 'task-123', got '%s'", msg.TaskID)
	}
}

// TestWebSocketServer_Creation 测试 WebSocket 服务器创建
func TestWebSocketServer_Creation(t *testing.T) {
	card := AgentCard{Name: "test", URL: "http://localhost:8080"}
	base := NewServer(card, nil, nil)
	ws := NewWebSocketServer(base)

	if ws == nil {
		t.Fatal("expected non-nil WebSocket server")
	}
	if ws.server != base {
		t.Error("expected WebSocket server to reference base server")
	}
	if ws.GetClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", ws.GetClientCount())
	}

	// 清理
	_ = ws.Close()
}

// TestWebSocketEnabledServer 测试 WebSocket 增强服务器
func TestWebSocketEnabledServer(t *testing.T) {
	card := AgentCard{Name: "test", URL: "http://localhost:8080"}
	server := NewWebSocketEnabledServer(card, nil, nil)

	if server.SecureServer == nil {
		t.Error("expected secure server")
	}
	if server.wsServer == nil {
		t.Error("expected WebSocket server")
	}
	if server.GetWebSocketServer() == nil {
		t.Error("expected non-nil WebSocket server")
	}

	// 测试 HTTP 请求
	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// 清理
	_ = server.wsServer.Close()
}

// TestDefaultRateLimitKey 测试默认限流键
func TestDefaultRateLimitKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/task/send", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	key := DefaultRateLimitKey(req)
	if !strings.Contains(key, "192.168.1.1") {
		t.Errorf("expected key to contain IP, got %s", key)
	}
	if !strings.Contains(key, "/task/send") {
		t.Errorf("expected key to contain path, got %s", key)
	}
}

// TestAuthRateLimitKey 测试认证限流键
func TestAuthRateLimitKey(t *testing.T) {
	// 无认证用户
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	key := AuthRateLimitKey(req)
	if key != "192.168.1.1:1234" {
		t.Errorf("expected IP for anonymous, got %s", key)
	}
}

// BenchmarkRateLimiter_Allow 基准测试限流器
func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000, 1000)
	key := "bench-client"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(key)
	}
}

// BenchmarkAuthMiddleware 基准测试认证中间件
func BenchmarkAuthMiddleware(b *testing.B) {
	auth := NewAuthMiddleware()
	auth.AddAPIKey("bench-key", "bench-user")

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/task/send", nil)
	req.Header.Set("X-API-Key", "bench-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
