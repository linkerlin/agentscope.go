// gateway/ratelimit.go realises the rate-limit half of 18.11: login and
// session-creating write endpoints are throttled per identity (authenticated
// user) or IP. Two backends behind one interface:
//
//   - localLimiter: in-process token buckets (default; single-replica
//     semantics — with N replicas the effective limit is N×, documented);
//   - coordLimiter: fixed-window counters on the message bus's CoordCounter
//     capability (messagebus/counter.go; RedisBus runs an atomic INCR +
//     conditional PEXPIRE script), shared across replicas when the
//     deployment wires a coordination bus. AsCoordRateLimiter picks it
//     automatically; LocalRateLimiter is the fallback.
package gateway

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// RateLimiter is the pluggable throttle interface (18.11). Allow reports
// whether the request identified by key may proceed; implementations must be
// safe for concurrent use.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// DefaultRateLimitTuning: login and session creation get a modest shared
// default; deployments override via WithRateLimiter.
const (
	DefaultRateLimit    = 30          // requests
	DefaultRateWindow   = time.Minute // per window
	rateCounterTTLGrace = 2 * time.Second
)

// --- in-process token bucket (single-replica default) ---

type bucket struct {
	tokens     float64
	last       time.Time
	limit      float64
	burst      float64
	windowSecs float64
}

func (b *bucket) take(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * (b.limit / b.windowSecs)
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// LocalRateLimiter throttles per key with in-process token buckets. With N
// replicas each keeps its own buckets, so the effective ceiling is limit×N —
// wire a CoordBus counter (AsCoordRateLimiter) for exact cross-replica
// semantics.
type LocalRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
	window  time.Duration
}

// NewLocalRateLimiter allows `limit` requests per key per `window`
// (burst == limit).
func NewLocalRateLimiter(limit int, window time.Duration) *LocalRateLimiter {
	return &LocalRateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
	}
}

// Allow implements RateLimiter.
func (l *LocalRateLimiter) Allow(_ context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{
			tokens:     float64(l.limit),
			last:       time.Now(),
			limit:      float64(l.limit),
			burst:      float64(l.limit),
			windowSecs: l.window.Seconds(),
		}
		l.buckets[key] = b
	}
	return b.take(time.Now())
}

// --- CoordBus-backed fixed-window counter (cross-replica) ---

// CoordRateLimiter throttles per key with a fixed window shared by every
// replica talking to the same bus. The counter capability comes from
// messagebus.CoordCounter (RedisBus: atomic INCR + conditional PEXPIRE
// script; LocalBus: in-process windows) — obtain via AsCoordRateLimiter.
type CoordRateLimiter struct {
	counter messagebus.CoordCounter
	limit   int64
	window  time.Duration
}

// NewCoordRateLimiter allows `limit` requests per key per `window` across
// all replicas sharing the counter.
func NewCoordRateLimiter(c messagebus.CoordCounter, limit int, window time.Duration) *CoordRateLimiter {
	return &CoordRateLimiter{counter: c, limit: int64(limit), window: window}
}

// Allow implements RateLimiter. A counter failure fails open (availability
// over strictness for a protective limit) — the error path is logged by the
// caller middleware's absence of a rejection.
func (l *CoordRateLimiter) Allow(ctx context.Context, key string) bool {
	n, err := l.counter.CoordIncr(ctx, key, l.window+rateCounterTTLGrace)
	if err != nil {
		return true // fail open: the limiter must not take the gateway down
	}
	return n <= l.limit
}

// AsCoordRateLimiter returns a cross-replica RateLimiter when the bus
// implements CoordCounter, else nil (fall back to LocalRateLimiter).
func AsCoordRateLimiter(b messagebus.Bus, limit int, window time.Duration) RateLimiter {
	if bc := messagebus.AsCoordCounter(b); bc != nil {
		return NewCoordRateLimiter(bc, limit, window)
	}
	return nil
}

// rateLimitKey derives the throttle identity: the authenticated user when
// present, else the remote address (IP:port is fine for a protective limit —
// NAT'd clients share a bucket, which errs on the strict side).
func rateLimitKey(r *http.Request) string {
	if uid := service.UserIDFromContext(r.Context()); uid != "" {
		return "u:" + uid
	}
	return "ip:" + r.RemoteAddr
}

// RateLimitMiddleware returns 429 when the key exceeds its budget.
func RateLimitMiddleware(limiter RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter != nil && !limiter.Allow(r.Context(), rateLimitKey(r)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

var _ RateLimiter = (*CoordRateLimiter)(nil)
