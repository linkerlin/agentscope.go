package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// RegisterAuthRoutes registers authentication and user-management endpoints.
// These routes are public (no auth required) for registration and login.
// Idempotent: repeated calls (directly or via RegisterAppRoutes) register
// once. Register and /me never need a JWT authenticator; login is only
// mounted when one is available — the explicit argument, else the one
// remembered by NewApp / WithJWTAuth. Register and login sit behind the
// rate limiter (18.11): they are the two unauthenticated write endpoints a
// flooding client would otherwise park for free.
func (s *Server) RegisterAuthRoutes(jwtAuth *service.JWTAuthenticator) {
	if s.storage == nil || s.authRoutesRegistered {
		return
	}
	if jwtAuth == nil {
		jwtAuth = s.jwtAuth
	}
	s.authRoutesRegistered = true
	s.mux.HandleFunc("/api/v1/auth/register", s.withRateLimit(s.handleRegister))
	if jwtAuth != nil {
		s.mux.HandleFunc("/api/v1/auth/login", s.withRateLimit(s.handleLogin(jwtAuth)))
	}
	s.mux.HandleFunc("/api/v1/me", s.requireAuth(s.handleMe))
}

// withRateLimit wraps a public write endpoint with the server's rate limiter
// (18.11). Limiter selection at assembly time: the bus's shared CoordCounter
// when available (exact cross-replica budget), else a process-local limiter
// (per-replica budget — with N replicas the effective ceiling is N× the
// configured one, the CoordCounter path exists to avoid exactly that).
//
// The limiter is read per request, NOT at registration: auto-selection runs
// in Start(), which by contract happens after routes are registered
// (NewApp / RegisterAppRoutes order), so a registration-time nil would
// permanently bake "no throttling" into the wrapped handler and production
// logins would run unthrottled. A nil (or disabled) limiter passes through.
func (s *Server) withRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := s.rateLimiter
		if l == nil || s.rateLimitDisabled {
			next(w, r)
			return
		}
		RateLimitMiddleware(l, next)(w, r)
	}
}

type registerRequest struct {
	Name string `json:"name"`
}

type registerResponse struct {
	UserID string `json:"user_id"`
	APIKey string `json:"api_key"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req registerRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		writeBodyLimitError(w, err)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user := &service.User{
		ID:        generateID("u"),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}
	if err := s.storage.SaveUser(ctx, user); err != nil {
		http.Error(w, fmt.Sprintf("save user failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate a random API key credential for the new user. Only the SHA-256
	// digest is stored; the plaintext is returned exactly once, here (22.1).
	apiKey, err := service.GenerateAPIKey()
	if err != nil {
		http.Error(w, fmt.Sprintf("generate api key failed: %v", err), http.StatusInternalServerError)
		return
	}
	cred := &service.Credential{
		ID:        generateID("cred"),
		UserID:    user.ID,
		Provider:  "api_key",
		Label:     "default",
		Encrypted: service.HashAPIKey(apiKey),
	}
	if err := s.storage.SaveCredential(ctx, cred); err != nil {
		http.Error(w, fmt.Sprintf("save credential failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(registerResponse{
		UserID: user.ID,
		APIKey: apiKey,
	})
}

type loginRequest struct {
	APIKey string `json:"api_key"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// handleLogin exchanges an API key for a JWT. The caller must prove
// possession of the key (22.1): knowing a user ID is not a credential.
func (s *Server) handleLogin(jwtAuth *service.JWTAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req loginRequest
		if err := decodeJSONLimit(w, r, &req); err != nil {
			writeBodyLimitError(w, err)
			return
		}
		if req.APIKey == "" {
			http.Error(w, "api_key is required", http.StatusBadRequest)
			return
		}

		user, err := service.FindUserByAPIKey(r.Context(), s.storage, req.APIKey)
		if err != nil {
			http.Error(w, "invalid api_key", http.StatusUnauthorized)
			return
		}

		token, err := jwtAuth.GenerateToken(user.ID, 24*time.Hour)
		if err != nil {
			http.Error(w, fmt.Sprintf("token generation failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResponse{Token: token})
	}
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := s.storage.GetUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("get user failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         user.ID,
		"name":       user.Name,
		"created_at": user.CreatedAt,
	})
}
