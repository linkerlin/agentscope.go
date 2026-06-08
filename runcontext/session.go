package runcontext

import "context"

type sessionKey struct{}

// WithSessionID attaches a session ID to ctx for per-session middleware.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionKey{}, sessionID)
}

// SessionID returns the session ID from ctx, or empty if unset.
func SessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(sessionKey{}).(string)
	return v
}
