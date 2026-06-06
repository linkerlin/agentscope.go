package toolkit

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/tool"
)

// MiddlewareStage identifies the lifecycle stage for middleware interception.
type MiddlewareStage string

const (
	StageExecute     MiddlewareStage = "execute"
	StageExecuteTool MiddlewareStage = "execute_tool"
)

// Request carries the input for a middleware-wrapped execution.
type Request struct {
	Stage     MiddlewareStage
	ToolCalls []ToolCall
	ToolName  string            // set for single-tool execution
	ToolInput map[string]any    // set for single-tool execution
}

// Response carries the output from a middleware-wrapped execution.
type Response struct {
	Results  []ToolResult
	Single   *tool.Response // set for single-tool execution
}

// Handler is the core execution function wrapped by middleware.
type Handler func(ctx context.Context, req *Request) (*Response, error)

// Middleware wraps a Handler with cross-cutting concerns.
type Middleware interface {
	Wrap(next Handler) Handler
}

// chain composes multiple middleware into an onion.
func chain(handler Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i].Wrap(handler)
	}
	return handler
}

// --- Built-in middleware ---

// LoggingMiddleware logs tool execution entry/exit.
type LoggingMiddleware struct {
	Logf func(format string, args ...interface{})
}

func NewLoggingMiddleware(logf func(format string, args ...interface{})) *LoggingMiddleware {
	if logf == nil {
		logf = func(f string, a ...interface{}) { fmt.Printf(f+"\n", a...) }
	}
	return &LoggingMiddleware{Logf: logf}
}

func (m *LoggingMiddleware) Wrap(next Handler) Handler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		start := time.Now()
		if req.Stage == StageExecute {
			m.Logf("[toolkit] execute start: %d calls", len(req.ToolCalls))
		} else {
			m.Logf("[toolkit] execute_tool start: %s", req.ToolName)
		}
		resp, err := next(ctx, req)
		dur := time.Since(start)
		if err != nil {
			m.Logf("[toolkit] execute error after %s: %v", dur, err)
		} else {
			if req.Stage == StageExecute {
				m.Logf("[toolkit] execute done after %s: %d results", dur, len(resp.Results))
			} else {
				m.Logf("[toolkit] execute_tool done after %s", dur)
			}
		}
		return resp, err
	}
}

// MetricsMiddleware counts executions and records latency.
type MetricsMiddleware struct {
	OnExecute     func(stage MiddlewareStage, count int, dur time.Duration, err error)
}

func NewMetricsMiddleware(onExecute func(stage MiddlewareStage, count int, dur time.Duration, err error)) *MetricsMiddleware {
	return &MetricsMiddleware{OnExecute: onExecute}
}

func (m *MetricsMiddleware) Wrap(next Handler) Handler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		dur := time.Since(start)
		count := len(req.ToolCalls)
		if req.Stage == StageExecuteTool {
			count = 1
		}
		if m.OnExecute != nil {
			m.OnExecute(req.Stage, count, dur, err)
		}
		return resp, err
	}
}

// PermissionMiddleware intercepts tool calls before execution.
// It can block, log, or modify calls based on custom logic.
type PermissionMiddleware struct {
	Allow func(ctx context.Context, calls []ToolCall) error
}

func NewPermissionMiddleware(allow func(ctx context.Context, calls []ToolCall) error) *PermissionMiddleware {
	return &PermissionMiddleware{Allow: allow}
}

func (m *PermissionMiddleware) Wrap(next Handler) Handler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		if req.Stage == StageExecute && m.Allow != nil {
			if err := m.Allow(ctx, req.ToolCalls); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
		return next(ctx, req)
	}
}
