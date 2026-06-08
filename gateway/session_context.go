package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/runcontext"
)

// ContextWithSessionID attaches a session ID to ctx for per-session middleware.
func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return runcontext.WithSessionID(ctx, sessionID)
}

// SessionIDFromContext returns the session ID from ctx.
func SessionIDFromContext(ctx context.Context) string {
	return runcontext.SessionID(ctx)
}
