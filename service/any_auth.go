package service

import (
	"context"
	"fmt"
	"net/http"
)

// AnyAuthenticator tries multiple authenticators in order and succeeds on the first match.
type AnyAuthenticator struct {
	auths []Authenticator
}

// NewAnyAuthenticator creates an authenticator that accepts any configured method.
func NewAnyAuthenticator(auths ...Authenticator) *AnyAuthenticator {
	return &AnyAuthenticator{auths: auths}
}

func (a *AnyAuthenticator) Authenticate(r *http.Request) (context.Context, error) {
	var lastErr error
	for _, auth := range a.auths {
		if auth == nil {
			continue
		}
		ctx, err := auth.Authenticate(r)
		if err == nil {
			return ctx, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		return r.Context(), fmt.Errorf("no authenticator configured")
	}
	return r.Context(), lastErr
}
