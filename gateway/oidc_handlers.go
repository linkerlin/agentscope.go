package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// RegisterOIDCRoutes registers OIDC SSO endpoints.
func (s *Server) RegisterOIDCRoutes(oidcAuth *service.OIDCAuthenticator, jwtAuth *service.JWTAuthenticator) {
	s.mux.HandleFunc("/api/v1/auth/oidc/login", s.handleOIDCLogin(oidcAuth))
	s.mux.HandleFunc("/api/v1/auth/oidc/callback", s.handleOIDCCallback(oidcAuth, jwtAuth))
}

func (s *Server) handleOIDCLogin(oidcAuth *service.OIDCAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		verifier, challenge, err := service.GeneratePKCE()
		if err != nil {
			http.Error(w, fmt.Sprintf("pkce generation failed: %v", err), http.StatusInternalServerError)
			return
		}
		state := generateID("state")
		// Store verifier in cookie or short-lived cache (simplified: cookie)
		http.SetCookie(w, &http.Cookie{
			Name:     "oidc_verifier",
			Value:    verifier,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		authURL, err := oidcAuth.BuildAuthURL(state, challenge)
		if err != nil {
			http.Error(w, fmt.Sprintf("auth url failed: %v", err), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func (s *Server) handleOIDCCallback(oidcAuth *service.OIDCAuthenticator, jwtAuth *service.JWTAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		cookie, err := r.Cookie("oidc_verifier")
		if err != nil {
			http.Error(w, "missing verifier cookie", http.StatusBadRequest)
			return
		}
		verifier := cookie.Value

		ctx := r.Context()
		tr, err := oidcAuth.ExchangeCode(ctx, code, verifier)
		if err != nil {
			http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusUnauthorized)
			return
		}
		ui, err := oidcAuth.FetchUserInfo(ctx, tr.AccessToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("userinfo failed: %v", err), http.StatusUnauthorized)
			return
		}

		// Find or create user
		user, err := s.storage.GetUserByEmail(ctx, ui.Email)
		if err != nil {
			// Create new user
			user = &service.User{
				ID:        generateID("u"),
				Name:      ui.Name,
				Email:     ui.Email,
				CreatedAt: time.Now(),
			}
			if err := s.storage.SaveUser(ctx, user); err != nil {
				http.Error(w, fmt.Sprintf("save user failed: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Issue our own JWT
		token, err := jwtAuth.GenerateToken(user.ID, 24*time.Hour)
		if err != nil {
			http.Error(w, fmt.Sprintf("token generation failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":     token,
			"user_id":   user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"id_token":  tr.IDToken,
		})
	}
}
