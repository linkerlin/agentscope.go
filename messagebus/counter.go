package messagebus

import (
	"context"
	"sync/atomic"
	"time"
)

// --- CoordCounter (gateway rate limiting, 18.11) ---

// CoordCounter is the OPTIONAL fixed-window counter extension of a bus.
// Rate limiters use it to share one window across replicas: RedisBus
// implements it with an atomic INCR + conditional PEXPIRE script; LocalBus
// with in-process windows. Obtain via AsCoordCounter; nil means the bus has
// no shared counters and callers fall back to process-local throttling.
type CoordCounter interface {
	// CoordIncr increments the windowed counter and reports its new value.
	// The first increment in a window starts the ttl countdown; when the
	// window expires the counter restarts from 1.
	CoordIncr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// AsCoordCounter returns a CoordCounter view of b if it implements shared
// windowed counters, else nil.
func AsCoordCounter(b Bus) CoordCounter {
	if cc, ok := b.(CoordCounter); ok {
		return cc
	}
	return nil
}

// localCounterWindow is one fixed-window counter slot (LocalBus).
type localCounterWindow struct {
	val       atomic.Int64
	expiresAt time.Time
}
