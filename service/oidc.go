package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OIDCConfig holds OpenID Connect provider configuration.
type OIDCConfig struct {
	Issuer       string // e.g. "https://accounts.google.com"
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g. "http://localhost:8080/api/v1/auth/oidc/callback"
	Scopes       []string
}

// OIDCAuthenticator implements SSO via OpenID Connect.
type OIDCAuthenticator struct {
	cfg     OIDCConfig
	jwtAuth *JWTAuthenticator
	storage Storage

	// cached discovery document
	discovery *oidcDiscovery
	discAt    time.Time
}

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// NewOIDCAuthenticator creates an OIDC authenticator.
func NewOIDCAuthenticator(cfg OIDCConfig, jwtAuth *JWTAuthenticator, storage Storage) *OIDCAuthenticator {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email"}
	}
	return &OIDCAuthenticator{
		cfg:     cfg,
		jwtAuth: jwtAuth,
		storage: storage,
	}
}

func (a *OIDCAuthenticator) fetchDiscovery(ctx context.Context) (*oidcDiscovery, error) {
	if a.discovery != nil && time.Since(a.discAt) < 1*time.Hour {
		return a.discovery, nil
	}
	wellKnown := strings.TrimSuffix(a.cfg.Issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc discovery failed: %d", resp.StatusCode)
	}
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, err
	}
	a.discovery = &disc
	a.discAt = time.Now()
	return a.discovery, nil
}

// BuildAuthURL generates the OIDC authorization URL with PKCE.
func (a *OIDCAuthenticator) BuildAuthURL(state, codeChallenge string) (string, error) {
	disc, err := a.fetchDiscovery(context.Background())
	if err != nil {
		return "", err
	}
	u, err := url.Parse(disc.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", a.cfg.ClientID)
	q.Set("redirect_uri", a.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(a.cfg.Scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode exchanges the authorization code for an ID token and access token.
func (a *OIDCAuthenticator) ExchangeCode(ctx context.Context, code, codeVerifier string) (*oidcTokenResponse, error) {
	disc, err := a.fetchDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", a.cfg.ClientID)
	data.Set("client_secret", a.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", a.cfg.RedirectURL)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %d %s", resp.StatusCode, string(body))
	}
	var tr oidcTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

// FetchUserInfo retrieves user info from the OIDC userinfo endpoint.
func (a *OIDCAuthenticator) FetchUserInfo(ctx context.Context, accessToken string) (*oidcUserInfo, error) {
	disc, err := a.fetchDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, disc.UserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo failed: %d", resp.StatusCode)
	}
	var ui oidcUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return nil, err
	}
	return &ui, nil
}

// Authenticate implements Authenticator (for Bearer token validation after OIDC login).
func (a *OIDCAuthenticator) Authenticate(r *http.Request) (context.Context, error) {
	// Delegate to JWT authenticator for Bearer tokens issued by our system
	return a.jwtAuth.Authenticate(r)
}

// GeneratePKCE generates a code verifier and S256 challenge for PKCE.
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	// S256 challenge = base64url(sha256(verifier))
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

type oidcUserInfo struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture string `json:"picture,omitempty"`
}
