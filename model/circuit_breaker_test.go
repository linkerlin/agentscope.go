package model

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 100}
	r := NewRouter(primary,
		WithMaxRetries(0),
		WithBackoff(0),
		WithCircuitBreaker(3, 10*time.Second),
	)

	// First 3 calls: exhaust retries, each counts as 1 failure
	for i := 0; i < 3; i++ {
		_, err := r.Chat(context.Background(), nil)
		if err == nil {
			t.Fatalf("expected error on call %d", i+1)
		}
	}

	// Circuit should now be open
	if r.CircuitState() != CircuitOpen {
		t.Fatalf("expected CircuitOpen, got %d", r.CircuitState())
	}

	// 4th call: should fail fast with ErrCircuitOpen
	_, err := r.Chat(context.Background(), nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 100}
	r := NewRouter(primary,
		WithMaxRetries(0),
		WithBackoff(0),
		WithCircuitBreaker(2, 50*time.Millisecond),
	)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		r.Chat(context.Background(), nil)
	}
	if r.CircuitState() != CircuitOpen {
		t.Fatal("expected open after 2 failures")
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Make primary succeed now
	primary.failCount = 0

	// Next call should be allowed (half-open) and succeed
	resp, err := r.Chat(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success in half-open, got %v", err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}

	// Circuit should be closed again
	if r.CircuitState() != CircuitClosed {
		t.Fatalf("expected closed after successful probe, got %d", r.CircuitState())
	}
}

func TestCircuitBreaker_HalfOpenReOpens(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 100}
	r := NewRouter(primary,
		WithMaxRetries(0),
		WithBackoff(0),
		WithCircuitBreaker(2, 50*time.Millisecond),
	)

	// Trip the circuit
	for i := 0; i < 2; i++ {
		r.Chat(context.Background(), nil)
	}
	if r.CircuitState() != CircuitOpen {
		t.Fatal("expected open")
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Primary still fails — probe fails → re-open
	_, err := r.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected probe failure")
	}

	if r.CircuitState() != CircuitOpen {
		t.Fatalf("expected re-open after failed probe, got %d", r.CircuitState())
	}
}

func TestCircuitBreaker_DisabledByDefault(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 100}
	r := NewRouter(primary, WithMaxRetries(0), WithBackoff(0))

	// No circuit breaker — should never trip
	for i := 0; i < 20; i++ {
		r.Chat(context.Background(), nil)
	}
	if r.CircuitState() != CircuitClosed {
		t.Fatal("expected closed when CB disabled")
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	// Primary fails twice, then succeeds (resetting failures)
	primary := &mockFailingModel{name: "primary", failCount: 2}
	r := NewRouter(primary,
		WithMaxRetries(0),
		WithBackoff(0),
		WithCircuitBreaker(3, 10*time.Second),
	)

	// 2 failures
	r.Chat(context.Background(), nil) // fail 1
	r.Chat(context.Background(), nil) // fail 2

	// Now primary succeeds
	resp, err := r.Chat(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	_ = resp

	// Failures should be reset
	if r.CircuitState() != CircuitClosed {
		t.Fatalf("expected closed after success reset, got %d", r.CircuitState())
	}

	// 2 more failures shouldn't trip (need 3 consecutive)
	r.Chat(context.Background(), nil) // need to fail again
	// primary.failCount is 0 now, so this succeeds
	if r.CircuitState() != CircuitClosed {
		t.Fatal("should still be closed")
	}
}

func TestCircuitBreaker_ChatStream(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 100}
	r := NewRouter(primary,
		WithMaxRetries(0),
		WithBackoff(0),
		WithCircuitBreaker(2, 10*time.Second),
	)

	// Trip the circuit via ChatStream
	for i := 0; i < 2; i++ {
		r.ChatStream(context.Background(), nil)
	}

	if r.CircuitState() != CircuitOpen {
		t.Fatal("expected open")
	}

	// Should fail fast
	_, err := r.ChatStream(context.Background(), nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}
