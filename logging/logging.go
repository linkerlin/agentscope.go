// Package logging provides the structured logging foundation for AgentScope.Go,
// built on the standard library's log/slog. It establishes the project-wide
// convention: every package logs via this interface (or *slog.Logger directly),
// passing key/value pairs — never bare fmt.Sprintf strings — so logs are
// machine-parseable and filterable in production.
//
// Configuration is driven by environment variables (zero code for ops):
//
//	LOG_LEVEL=debug|info|warn|error   (default: info)
//	LOG_FORMAT=json|text              (default: json in prod, text in dev)
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger is the structured logging contract. *slog.Logger satisfies it
// directly, so callers may pass either this interface or a concrete
// *slog.Logger interchangeably.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Standard log attribute keys used across the framework, so log queries are
// consistent (e.g. grep for agent_id=... in any backend).
const (
	KeyAgentID   = "agent_id"
	KeySessionID = "session_id"
	KeyUserID    = "user_id"
	KeyRequestID = "request_id"
	KeyTool      = "tool"
	KeyModel     = "model"
	KeyError     = "error"
	KeyDuration  = "duration_ms"
)

var (
	defaultLogger     Logger
	defaultLoggerOnce bool
)

// Default returns the package-level logger, lazily initialised from the
// LOG_LEVEL / LOG_FORMAT environment variables on first call.
func Default() Logger {
	if defaultLoggerOnce {
		return defaultLogger
	}
	defaultLogger = NewFromEnv()
	defaultLoggerOnce = true
	return defaultLogger
}

// SetDefault overrides the package-level logger (e.g. to inject a request-
// scoped logger or disable logging in tests via slog.Discard).
func SetDefault(l Logger) {
	defaultLogger = l
	defaultLoggerOnce = true
}

// New creates a configured *slog.Logger (which satisfies Logger).
func New(level slog.Level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.ToLower(format) == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// NewFromEnv builds a logger from LOG_LEVEL (default info) and LOG_FORMAT
// (default json). Falls back gracefully on bad values.
func NewFromEnv() *slog.Logger {
	return New(levelFromEnv(), formatFromEnv())
}

// Discard returns a logger that drops everything — useful in tests.
func Discard() Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 100}))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// --- context-scoped logger ---

type ctxKey struct{}

// WithLogger stores l in ctx so downstream code can retrieve it via FromContext.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger stored in ctx, falling back to Default().
// This lets request handlers carry a pre-enriched logger (with request_id,
// user_id, etc.) through the call chain.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return Default()
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func formatFromEnv() string {
	if f := strings.ToLower(os.Getenv("LOG_FORMAT")); f == "text" || f == "json" {
		return f
	}
	return "json"
}

// compile-time: *slog.Logger satisfies Logger.
var _ Logger = (*slog.Logger)(nil)
