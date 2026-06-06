package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

type mockFailingModel struct {
	name      string
	failCount int
	calls     int
}

func (m *mockFailingModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	m.calls++
	if m.failCount > 0 {
		m.failCount--
		return nil, &RetryableError{Cause: errors.New("transient error")}
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *mockFailingModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	m.calls++
	if m.failCount > 0 {
		m.failCount--
		return nil, &RetryableError{Cause: errors.New("transient error")}
	}
	ch := make(chan *StreamChunk, 1)
	ch <- &StreamChunk{Delta: "ok", Done: true}
	close(ch)
	return ch, nil
}

func (m *mockFailingModel) ModelName() string { return m.name }

type mockPermanentFailModel struct{ name string }

func (m *mockPermanentFailModel) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	return nil, errors.New("permanent failure")
}

func (m *mockPermanentFailModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	return nil, errors.New("permanent failure")
}

func (m *mockPermanentFailModel) ModelName() string { return m.name }

func TestRouter_RetrySuccess(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 2}
	r := NewRouter(primary, WithMaxRetries(3), WithBackoff(10*time.Millisecond))

	resp, err := r.Chat(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	if primary.calls != 3 {
		t.Fatalf("expected 3 calls (2 fails + 1 success), got %d", primary.calls)
	}
}

func TestRouter_RetryExhaustedWithFallback(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 10}
	fallback := &mockFailingModel{name: "fallback", failCount: 0}
	r := NewRouter(primary, WithMaxRetries(2), WithBackoff(10*time.Millisecond), WithFallback(fallback))

	resp, err := r.Chat(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected fallback success: %v", err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	if primary.calls != 3 { // initial + 2 retries
		t.Fatalf("expected 3 primary calls, got %d", primary.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected 1 fallback call, got %d", fallback.calls)
	}
}

func TestRouter_RetryExhaustedNoFallback(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 10}
	r := NewRouter(primary, WithMaxRetries(1), WithBackoff(10*time.Millisecond))

	_, err := r.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when retries exhausted and no fallback")
	}
}

func TestRouter_ChatStreamRetry(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 1}
	r := NewRouter(primary, WithMaxRetries(2), WithBackoff(10*time.Millisecond))

	ch, err := r.ChatStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected stream success: %v", err)
	}
	var chunks []string
	for c := range ch {
		if c.Delta != "" {
			chunks = append(chunks, c.Delta)
		}
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestRouter_ModelName(t *testing.T) {
	primary := &mockFailingModel{name: "gpt-4"}
	r := NewRouter(primary)
	if r.ModelName() != "gpt-4" {
		t.Fatalf("model name mismatch: %s", r.ModelName())
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(&RetryableError{Cause: errors.New("x")}) {
		t.Fatal("RetryableError should be retryable")
	}
	if IsRetryable(errors.New("normal")) {
		t.Fatal("normal error should not be retryable")
	}
	if IsRetryable(nil) {
		t.Fatal("nil should not be retryable")
	}
}

func TestRouter_ContextCancel(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 10}
	r := NewRouter(primary, WithMaxRetries(10), WithBackoff(1*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := r.Chat(ctx, nil)
	if err == nil {
		t.Fatal("expected error when context cancelled")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRouter_ChatStreamFallback(t *testing.T) {
	primary := &mockFailingModel{name: "primary", failCount: 10}
	fallback := &mockFailingModel{name: "fallback", failCount: 0}
	r := NewRouter(primary, WithMaxRetries(0), WithBackoff(0), WithFallback(fallback))

	ch, err := r.ChatStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected fallback stream success: %v", err)
	}
	var chunks []string
	for c := range ch {
		if c.Delta != "" {
			chunks = append(chunks, c.Delta)
		}
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Fatalf("unexpected chunks from fallback: %v", chunks)
	}
}
