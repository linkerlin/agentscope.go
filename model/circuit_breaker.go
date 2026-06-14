package model

import (
	"errors"
	"sync/atomic"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int32

const (
	CircuitClosed   CircuitState = iota // normal operation
	CircuitOpen                         // tripped, requests fail fast
	CircuitHalfOpen                     // allowing probe requests
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("router: circuit breaker is open")

// circuitBreaker implements a three-state circuit breaker.
// States: Closed → (failures ≥ threshold) → Open → (after cooldown) → HalfOpen → (success) → Closed
//
//	└→ (failure) → Open
type circuitBreaker struct {
	state       atomic.Int32 // CircuitState
	failures    atomic.Int32
	lastFailure atomic.Int64 // unix nano timestamp

	threshold   int           // consecutive failures before opening
	cooldown    time.Duration // how long to stay open before half-open
	halfOpenN   atomic.Int32  // remaining probe slots in half-open state
	halfOpenMax int           // max concurrent probes in half-open
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	cb := &circuitBreaker{
		threshold:   threshold,
		cooldown:    cooldown,
		halfOpenMax: 1,
	}
	cb.state.Store(int32(CircuitClosed))
	return cb
}

func (cb *circuitBreaker) getState() CircuitState {
	return CircuitState(cb.state.Load())
}

// allowRequest checks if a request should proceed.
// Returns ErrCircuitOpen if the circuit is open.
// Returns nil if the request is allowed (closed or half-open probe).
func (cb *circuitBreaker) allowRequest() error {
	state := cb.getState()

	switch state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		// Check if cooldown has elapsed
		lastFail := time.Unix(0, cb.lastFailure.Load())
		if time.Since(lastFail) >= cb.cooldown {
			// Try to transition to half-open
			if cb.state.CompareAndSwap(int32(CircuitOpen), int32(CircuitHalfOpen)) {
				cb.halfOpenN.Store(int32(cb.halfOpenMax))
			}
			// Fall through to half-open check
		} else {
			return ErrCircuitOpen
		}
		fallthrough

	case CircuitHalfOpen:
		// Try to acquire a probe slot
		if cb.halfOpenN.Add(-1) >= 0 {
			return nil
		}
		// No probe slots available
		cb.halfOpenN.Add(1) // restore
		return ErrCircuitOpen
	}

	return nil
}

// onSuccess records a successful request, potentially closing the circuit.
func (cb *circuitBreaker) onSuccess() {
	state := cb.getState()
	if state == CircuitHalfOpen {
		// Successful probe → close circuit
		cb.failures.Store(0)
		cb.state.Store(int32(CircuitClosed))
	} else if state == CircuitClosed {
		cb.failures.Store(0)
	}
}

// onFailure records a failed request, potentially opening the circuit.
func (cb *circuitBreaker) onFailure() {
	state := cb.getState()

	if state == CircuitHalfOpen {
		// Failed probe → re-open circuit
		cb.lastFailure.Store(time.Now().UnixNano())
		cb.state.Store(int32(CircuitOpen))
		return
	}

	if state == CircuitClosed {
		f := cb.failures.Add(1)
		cb.lastFailure.Store(time.Now().UnixNano())
		if int(f) >= cb.threshold {
			cb.state.Store(int32(CircuitOpen))
		}
	}
}

// WithCircuitBreaker enables circuit breaker protection on the Router.
// threshold: consecutive failures before opening (e.g. 5)
// cooldown: how long to stay open before allowing a probe (e.g. 30s)
func WithCircuitBreaker(threshold int, cooldown time.Duration) RouterOption {
	return func(r *Router) {
		if threshold <= 0 {
			threshold = 5
		}
		if cooldown <= 0 {
			cooldown = 30 * time.Second
		}
		r.cb = newCircuitBreaker(threshold, cooldown)
	}
}

// CircuitState returns the current circuit breaker state, or CircuitClosed if disabled.
func (r *Router) CircuitState() CircuitState {
	if r.cb == nil {
		return CircuitClosed
	}
	return r.cb.getState()
}
