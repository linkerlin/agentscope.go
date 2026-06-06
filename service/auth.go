package service

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ContextKey is the type for context keys used by the auth middleware.
type ContextKey string

const (
	// ContextKeyUserID stores the authenticated user ID in the request context.
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeyUser stores the authenticated user in the request context.
	ContextKeyUser ContextKey = "user"
)

// Authenticator validates incoming requests and extracts the user context.
type Authenticator interface {
	// Authenticate validates the request and returns a context enriched with user info.
	Authenticate(r *http.Request) (context.Context, error)
}

// APIKeyAuthenticator validates requests using an API key header.
type APIKeyAuthenticator struct {
	storage Storage
	header  string
}

// NewAPIKeyAuthenticator creates an API key authenticator.
// Header defaults to "X-API-Key" if empty.
func NewAPIKeyAuthenticator(storage Storage, header string) *APIKeyAuthenticator {
	if header == "" {
		header = "X-API-Key"
	}
	return &APIKeyAuthenticator{storage: storage, header: header}
}

// Authenticate extracts the API key, looks up the credential, and validates it.
func (a *APIKeyAuthenticator) Authenticate(r *http.Request) (context.Context, error) {
	key := r.Header.Get(a.header)
	if key == "" {
		return r.Context(), fmt.Errorf("missing API key header: %s", a.header)
	}

	ctx := r.Context()
	// Look up credential by provider "api_key" and label matching the key.
	// In production, credentials should be indexed by key hash for O(1) lookup.
	// This implementation does a linear scan (suitable for MemoryStorage dev/test).
	users, err := a.storage.ListUsers(ctx)
	if err != nil {
		return ctx, fmt.Errorf("auth: list users failed: %w", err)
	}
	for _, u := range users {
		creds, err := a.storage.ListCredentialsByUser(ctx, u.ID)
		if err != nil {
			continue
		}
		for _, c := range creds {
			if c.Provider == "api_key" && subtle.ConstantTimeCompare([]byte(c.Encrypted), []byte(key)) == 1 {
				ctx = context.WithValue(ctx, ContextKeyUserID, u.ID)
				ctx = context.WithValue(ctx, ContextKeyUser, u)
				return ctx, nil
			}
		}
	}
	return ctx, fmt.Errorf("invalid API key")
}

// JWTAuthenticator validates requests using a JWT Bearer token.
type JWTAuthenticator struct {
	secret []byte
	issuer string
}

// NewJWTAuthenticator creates a JWT authenticator.
func NewJWTAuthenticator(secret []byte, issuer string) *JWTAuthenticator {
	return &JWTAuthenticator{secret: secret, issuer: issuer}
}

// GenerateToken creates a new JWT for the given user ID with an optional expiry.
func (a *JWTAuthenticator) GenerateToken(userID string, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iss": a.issuer,
		"iat": time.Now().Unix(),
	}
	if expiry > 0 {
		claims["exp"] = time.Now().Add(expiry).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

// Authenticate extracts the Bearer token and validates the JWT.
func (a *JWTAuthenticator) Authenticate(r *http.Request) (context.Context, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return r.Context(), fmt.Errorf("missing Authorization header")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return r.Context(), fmt.Errorf("invalid Authorization header format")
	}
	tokenStr := parts[1]

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secret, nil
	}, jwt.WithIssuer(a.issuer))
	if err != nil {
		return r.Context(), fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return r.Context(), fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return r.Context(), fmt.Errorf("invalid token claims")
	}

	userID, _ := claims["sub"].(string)
	ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
	return ctx, nil
}

// AuthMiddleware wraps an http.Handler with authentication.
func AuthMiddleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, err := auth.Authenticate(r)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"unauthorized","message":"%s"}`, err.Error()), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyUserID).(string); ok {
		return id
	}
	return ""
}

// UserFromContext extracts the user from the request context.
func UserFromContext(ctx context.Context) *User {
	if u, ok := ctx.Value(ContextKeyUser).(*User); ok {
		return u
	}
	return nil
}
