package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// Router wraps one or more ChatModels with retry, fallback, and circuit-breaker logic.
type Router struct {
	primary   ChatModel
	fallback  ChatModel
	maxRetries int
	backoff    time.Duration
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithFallback sets the fallback model used when the primary fails.
func WithFallback(m ChatModel) RouterOption {
	return func(r *Router) { r.fallback = m }
}

// WithMaxRetries sets the maximum number of retries on the primary model.
func WithMaxRetries(n int) RouterOption {
	return func(r *Router) { r.maxRetries = n }
}

// WithBackoff sets the backoff duration between retries.
func WithBackoff(d time.Duration) RouterOption {
	return func(r *Router) { r.backoff = d }
}

// NewRouter creates a new model router.
func NewRouter(primary ChatModel, opts ...RouterOption) *Router {
	r := &Router{
		primary:    primary,
		maxRetries: 1,
		backoff:    500 * time.Millisecond,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Router) ModelName() string { return r.primary.ModelName() }

// Chat calls the primary model with retries, falling back to the fallback model if configured.
func (r *Router) Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 && r.backoff > 0 {
			select {
			case <-time.After(r.backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, err := r.primary.Chat(ctx, messages, options...)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	if r.fallback != nil {
		resp, err := r.fallback.Chat(ctx, messages, options...)
		if err != nil {
			return nil, fmt.Errorf("router: primary failed (%w), fallback also failed (%w)", lastErr, err)
		}
		return resp, nil
	}
	return nil, fmt.Errorf("router: primary failed after %d retries: %w", r.maxRetries, lastErr)
}

// ChatStream calls the primary model with retries, falling back if needed.
func (r *Router) ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 && r.backoff > 0 {
			select {
			case <-time.After(r.backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		ch, err := r.primary.ChatStream(ctx, messages, options...)
		if err == nil {
			return ch, nil
		}
		lastErr = err
	}
	if r.fallback != nil {
		ch, err := r.fallback.ChatStream(ctx, messages, options...)
		if err != nil {
			return nil, fmt.Errorf("router: primary stream failed (%w), fallback also failed (%w)", lastErr, err)
		}
		return ch, nil
	}
	return nil, fmt.Errorf("router: primary stream failed after %d retries: %w", r.maxRetries, lastErr)
}

// Ensure Router implements ChatModel at compile time.
var _ ChatModel = (*Router)(nil)

// RetryableError can be returned by model implementations to indicate that the error is transient and a retry may succeed.
type RetryableError struct {
	Cause error
}

func (e *RetryableError) Error() string { return fmt.Sprintf("retryable: %v", e.Cause) }
func (e *RetryableError) Unwrap() error { return e.Cause }

// IsRetryable reports whether an error should trigger a retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var re *RetryableError
	if errors.As(err, &re) {
		return true
	}
	return false
}
