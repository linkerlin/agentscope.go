package a2a

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	// API Keys 认证
	apiKeys map[string]string // key -> name
	mu      sync.RWMutex

	// JWT 认证（简化版，使用预共享密钥验证）
	jwtSecret string

	// 可选：OAuth2 / Bearer Token 验证回调
	bearerValidator func(token string) (string, error) // 返回用户名或错误

	// 匿名访问白名单路径
	publicPaths []string
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{
		apiKeys:     make(map[string]string),
		publicPaths: []string{"/.well-known/agent.json", "/health"},
	}
}

// AddAPIKey 添加 API Key
func (a *AuthMiddleware) AddAPIKey(key, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.apiKeys[key] = name
}

// RemoveAPIKey 移除 API Key
func (a *AuthMiddleware) RemoveAPIKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.apiKeys, key)
}

// SetJWTSecret 设置 JWT 密钥
func (a *AuthMiddleware) SetJWTSecret(secret string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.jwtSecret = secret
}

// SetBearerValidator 设置 Bearer Token 验证器
func (a *AuthMiddleware) SetBearerValidator(fn func(token string) (string, error)) {
	a.bearerValidator = fn
}

// AddPublicPath 添加公开路径
func (a *AuthMiddleware) AddPublicPath(path string) {
	a.publicPaths = append(a.publicPaths, path)
}

// Middleware 返回 HTTP 中间件
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否是公开路径
		for _, path := range a.publicPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 尝试 API Key 认证
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			if name, ok := a.validateAPIKey(apiKey); ok {
				r = r.WithContext(context.WithValue(r.Context(), "auth_user", name))
				next.ServeHTTP(w, r)
				return
			}
		}

		// 尝试 Bearer Token 认证
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if a.bearerValidator != nil {
				if user, err := a.bearerValidator(token); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), "auth_user", user))
					next.ServeHTTP(w, r)
					return
				}
			}
			// 简化 JWT 验证（仅检查密钥匹配）
			if a.validateJWT(token) {
				r = r.WithContext(context.WithValue(r.Context(), "auth_user", "jwt_user"))
				next.ServeHTTP(w, r)
				return
			}
		}

		// 认证失败
		w.Header().Set("WWW-Authenticate", `Bearer realm="a2a"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// validateAPIKey 验证 API Key（使用常数时间比较防止时序攻击）
func (a *AuthMiddleware) validateAPIKey(key string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for k, name := range a.apiKeys {
		if subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
			return name, true
		}
	}
	return "", false
}

// validateJWT 简化 JWT 验证（仅演示结构）
func (a *AuthMiddleware) validateJWT(token string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.jwtSecret == "" {
		return false
	}
	// 简化：检查 token 是否包含 secret 的哈希（实际应使用 jwt 库）
	// 这里仅做演示，生产环境应使用 github.com/golang-jwt/jwt
	expected := fmt.Sprintf("%x", a.jwtSecret)
	return strings.Contains(token, expected[:8])
}

// GetAuthUser 从请求上下文获取认证用户
func GetAuthUser(r *http.Request) string {
	if user, ok := r.Context().Value("auth_user").(string); ok {
		return user
	}
	return ""
}

// RateLimiter 限流器（令牌桶算法）
type RateLimiter struct {
	mu         sync.RWMutex
	buckets    map[string]*tokenBucket
	limit      int           // 每秒令牌数
	burst      int           // 突发容量
	windowSize time.Duration // 窗口大小
}

// tokenBucket 令牌桶
type tokenBucket struct {
	tokens    float64
	lastUpdate time.Time
	mu        sync.Mutex
}

// NewRateLimiter 创建限流器
func NewRateLimiter(limit, burst int) *RateLimiter {
	return &RateLimiter{
		buckets:    make(map[string]*tokenBucket),
		limit:      limit,
		burst:      burst,
		windowSize: time.Second,
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		bucket = &tokenBucket{
			tokens:     float64(rl.burst),
			lastUpdate: time.Now(),
		}
		rl.buckets[key] = bucket
		rl.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * float64(rl.limit)
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastUpdate = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

// GetRemaining 获取剩余令牌数
func (rl *RateLimiter) GetRemaining(key string) int {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		return rl.burst
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	tokens := bucket.tokens + elapsed*float64(rl.limit)
	if tokens > float64(rl.burst) {
		tokens = float64(rl.burst)
	}
	return int(tokens)
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter *RateLimiter, keyFunc func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				key = r.RemoteAddr
			}

			if !limiter.Allow(key) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// 设置剩余配额头
			remaining := limiter.GetRemaining(key)
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultRateLimitKey 默认限流键提取函数（按 IP + 路径）
func DefaultRateLimitKey(r *http.Request) string {
	return r.RemoteAddr + ":" + r.URL.Path
}

// AuthRateLimitKey 按认证用户限流
func AuthRateLimitKey(r *http.Request) string {
	user := GetAuthUser(r)
	if user != "" {
		return user
	}
	return r.RemoteAddr
}

// SecureServer 增强版 A2A 服务器（带认证和限流）
type SecureServer struct {
	*Server
	auth     *AuthMiddleware
	limiter  *RateLimiter
	mux      *http.ServeMux
}

// NewSecureServer 创建安全服务器
func NewSecureServer(card AgentCard, runner AgentRunner, store TaskStore) *SecureServer {
	base := NewServer(card, runner, store)
	return &SecureServer{
		Server:  base,
		auth:    NewAuthMiddleware(),
		limiter: NewRateLimiter(100, 200),
		mux:     http.NewServeMux(),
	}
}

// WithAuth 配置认证
func (s *SecureServer) WithAuth(auth *AuthMiddleware) *SecureServer {
	s.auth = auth
	return s
}

// WithRateLimit 配置限流
func (s *SecureServer) WithRateLimit(limiter *RateLimiter) *SecureServer {
	s.limiter = limiter
	return s
}

// ServeHTTP 实现带中间件的 HTTP 处理
func (s *SecureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 应用限流中间件
	rateLimited := RateLimitMiddleware(s.limiter, DefaultRateLimitKey)(s.Server)
	// 应用认证中间件
	authed := s.auth.Middleware(rateLimited)
	authed.ServeHTTP(w, r)
}

// CorsMiddleware CORS 中间件
func CorsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 使用自定义 ResponseWriter 捕获状态码
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(lw, r)
		
		duration := time.Since(start)
		user := GetAuthUser(r)
		if user == "" {
			user = "anonymous"
		}
		
		fmt.Printf("[A2A] %s %s %s %d %s\n", 
			user, r.Method, r.URL.Path, lw.statusCode, duration)
	})
}

// loggingResponseWriter 包装 ResponseWriter 以捕获状态码
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// ChainMiddleware 链式组合中间件
func ChainMiddleware(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Errors
var (
	ErrUnauthorized = errors.New("a2a: unauthorized")
	ErrRateLimited = errors.New("a2a: rate limited")
)
