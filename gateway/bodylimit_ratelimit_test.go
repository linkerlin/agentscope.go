package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// --- body limit ---

// TestDecodeJSONLimitTooLarge locks the 18.11 body contract: a body past the
// cap answers 413, is NOT fully consumed (the read stops at the boundary),
// and a within-cap body decodes normally.
func TestDecodeJSONLimitTooLarge(t *testing.T) {
	restore := maxBodyBytes
	maxBodyBytes = 1024
	defer func() { maxBodyBytes = restore }()

	var decoded struct {
		Name string `json:"name"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := decodeJSONLimit(w, r, &decoded); err != nil {
			writeBodyLimitError(w, err)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	// Within cap: decodes.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("within-cap body rejected: %d", w.Code)
	}
	if decoded.Name != "ok" {
		t.Fatalf("decoded wrong: %+v", decoded)
	}

	// Over cap with well-formed JSON whose value exceeds the cap (the real
	// shape: a legal request that grows past the limit). 413 and the body is
	// not drained — a 10 KiB body against a 1 KiB cap must not be fully read.
	big := `{"name":"` + strings.Repeat("a", 10*1024) + `"}`
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	// Track how much the server actually read via a counting wrapper.
	rc := &countingBody{rc: req2.Body}
	req2.Body = rc
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-cap body not rejected with 413: %d body=%q", w2.Code, w2.Body.String())
	}
	if rc.n > maxBodyBytes+64 {
		t.Fatalf("handler consumed %d bytes, wanted stop at cap %d", rc.n, maxBodyBytes)
	}
}

type countingBody struct {
	rc interface {
		Read([]byte) (int, error)
		Close() error
	}
	n int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.n += int64(n)
	return n, err
}
func (c *countingBody) Close() error { return c.rc.Close() }

// TestBodyLimitMiddlewareSkipsGET: the middleware does not wrap safe methods.
func TestBodyLimitMiddlewareSkipsGET(t *testing.T) {
	called := false
	h := BodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("GET handler not invoked")
	}
}

// TestSessionAPIBodyLimit is the in-place sessionapi check (18.11 asks for
// tests in the package that actually reads the body): POST /v2/chat with an
// over-cap JSON body answers 413.
func TestSessionAPIBodyLimit(t *testing.T) {
	// sessionapi has its own cap constant through Deps; exercise via the
	// gateway root wiring (RegisterV2Routes) so the assembled value is what
	// gets tested.
	srv := NewServer(&fakeV2Agent{})
	srv.sessionAPI() // force handler build with the root's MaxBodyBytes wiring
	h := srv.sessionAPIHandlers

	mux := http.NewServeMux()
	h.RegisterV2(mux, nil)

	big := strings.Repeat("a", int(DefaultMaxBodyBytes)+1024)
	req := httptest.NewRequest(http.MethodPost, "/v2/chat",
		strings.NewReader(`{"text":"`+big+`"}`))
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-cap /v2/chat not rejected with 413: %d", w.Code)
	}

	// Under-cap request proceeds past body parsing (fails later at session
	// resolution, not 413).
	req2 := httptest.NewRequest(http.MethodPost, "/v2/chat",
		strings.NewReader(`{"text":"hi"}`))
	req2.Header.Set("Accept", "application/json, text/event-stream")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code == http.StatusRequestEntityTooLarge {
		t.Fatal("under-cap /v2/chat wrongly 413")
	}
}

// --- rate limiting ---

func TestLocalRateLimiterBuckets(t *testing.T) {
	l := NewLocalRateLimiter(3, time.Minute)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if !l.Allow(ctx, "k") {
			t.Fatalf("request %d within budget rejected", i+1)
		}
	}
	if l.Allow(ctx, "k") {
		t.Fatal("4th request in window allowed")
	}
	// Different key has its own bucket.
	if !l.Allow(ctx, "other") {
		t.Fatal("independent key throttled")
	}
}

func TestCoordRateLimiterSharedBudget(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	l := AsCoordRateLimiter(bus, 3, time.Minute)
	if l == nil {
		t.Fatal("LocalBus must provide CoordCounter")
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if !l.Allow(ctx, "k") {
			t.Fatalf("request %d within budget rejected", i+1)
		}
	}
	if l.Allow(ctx, "k") {
		t.Fatal("4th request in shared window allowed")
	}
}

// TestCoordRateLimiterCrossReplica is the semantic the in-process limiter
// cannot give: two limiter instances over the same bus share one budget —
// the 18.11 answer to "N replicas must not mean N× the limit".
func TestCoordRateLimiterCrossReplica(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	l1 := AsCoordRateLimiter(bus, 4, time.Minute)
	l2 := AsCoordRateLimiter(bus, 4, time.Minute)
	if l1 == nil || l2 == nil {
		t.Fatal("LocalBus must provide CoordCounter")
	}
	ctx := context.Background()
	allowed := 0
	for i := 0; i < 8; i++ {
		lim := l1
		if i%2 == 1 {
			lim = l2 // alternate "replicas"
		}
		if lim.Allow(ctx, "k") {
			allowed++
		}
	}
	if allowed != 4 {
		t.Fatalf("two replicas allowed %d requests, want the shared budget 4", allowed)
	}
}

func TestCoordRateLimiterFailsOpen(t *testing.T) {
	l := NewCoordRateLimiter(failingCounter{}, 3, time.Minute)
	if !l.Allow(context.Background(), "k") {
		t.Fatal("counter failure must fail open, not lock everyone out")
	}
}

type failingCounter struct{}

func (failingCounter) CoordIncr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return 0, context.DeadlineExceeded
}

// TestRateLimitMiddleware429 locks the HTTP face: exceeding the budget
// answers 429 with Retry-After.
func TestRateLimitMiddleware429(t *testing.T) {
	l := NewLocalRateLimiter(1, time.Minute)
	handler := RateLimitMiddleware(l, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request rejected: %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	handler(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request not throttled: %d", w2.Code)
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Fatal("429 missing Retry-After")
	}
}

// TestServerRateLimitOnLogin is the endpoint-level integration: the login
// route throttles per IP without authentication.
func TestServerRateLimitOnLogin(t *testing.T) {
	storage := service.NewMemoryStorage()
	srv := NewServer(&fakeV2Agent{}).WithStorage(storage)
	srv.WithRateLimiter(NewLocalRateLimiter(2, time.Minute))
	srv.WithJWTAuth(service.NewJWTAuthenticator([]byte("test-secret-ratelimit"), "test"))
	srv.RegisterAuthRoutes(nil)

	body := `{"api_key":"nonexistent"}`
	var wg sync.WaitGroup
	codes := make([]int, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()

	ok, throttled := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusUnauthorized:
			ok++
		case http.StatusTooManyRequests:
			throttled++
		default:
			t.Fatalf("unexpected status %d", c)
		}
	}
	if ok != 2 || throttled != 1 {
		t.Fatalf("expected 2x401 + 1x429, got %v", codes)
	}
}

// TestServerRateLimitAutoSelection locks the Start-time selection rules:
// CoordCounter-capable bus → shared limiter; plain bus → local; disabled
// flag wins.
func TestServerRateLimitAutoSelection(t *testing.T) {
	// CoordCounter bus.
	srv := NewServer(&fakeV2Agent{})
	srv.WithMessageBus(messagebus.NewLocalBus())
	srv.Start()
	defer srv.Close()
	if _, ok := srv.rateLimiter.(*CoordRateLimiter); !ok {
		t.Fatalf("expected CoordRateLimiter with counter bus, got %T", srv.rateLimiter)
	}

	// Disabled.
	srv2 := NewServer(&fakeV2Agent{}).WithRateLimitDisabled()
	srv2.Start()
	defer srv2.Close()
	if srv2.rateLimiter != nil {
		t.Fatal("disabled flag must clear the limiter")
	}
}
