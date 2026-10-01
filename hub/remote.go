// hub/remote.go — the shared plumbing for HTTP-backed marketplaces (18.7):
// catalog fetching with retry/backoff, a small TTL cache, and the concrete
// GitHub MCP / Claw Skill hub implementations on top of the same cursor
// pagination the local FSHub uses.
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// remoteDefaults tune the retrying fetch shared by the HTTP hubs.
const (
	// defaultMaxAttempts bounds the retry sequence: 1 initial try + 2
	// retries (429/5xx only — other 4xx are permanent failures).
	defaultMaxAttempts = 3
	// defaultRetryBase is the exponential backoff seed for retries.
	defaultRetryBase = 500 * time.Millisecond
	// defaultRetryMax caps a single backoff sleep.
	defaultRetryMax = 8 * time.Second
	// defaultCatalogTTL is how long a fetched catalog stays cached; market
	//places are browsed far more often than they change, and GitHub's API
	// rate budget is not.
	defaultCatalogTTL = 60 * time.Second
	// maxCatalogBytes caps a catalog download (unbounded JSON is an attack
	// surface on any marketplace URL).
	maxCatalogBytes = 16 << 20 // 16 MiB
)

// fetcher is the retrying GET behind the remote hubs. A struct (not free
// functions) so tests can shorten the backoff and so the client is injectable.
type fetcher struct {
	client   *http.Client
	attempts int
	base     time.Duration
	max      time.Duration

	// nowSleep allows tests to observe/shorten backoff sleeps without real
	// waiting; nil uses time.Sleep.
	nowSleep func(ctx context.Context, d time.Duration) bool
}

func newFetcher() *fetcher {
	return &fetcher{
		client:   &http.Client{Timeout: 30 * time.Second},
		attempts: defaultMaxAttempts,
		base:     defaultRetryBase,
		max:      defaultRetryMax,
	}
}

// fetchJSON GETs url and decodes the body into out, retrying 429 and 5xx
// with exponential backoff (honouring Retry-After when it asks for MORE than
// the computed backoff — never sleeping less than the server demands).
// Permanent failures (other 4xx, decode errors) return immediately.
func (f *fetcher) fetchJSON(ctx context.Context, rawURL string, out any) error {
	status, body, err := f.fetch(ctx, rawURL, "application/json")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("hub: fetch %s: status %d", redactURL(rawURL), status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("hub: parse catalog from %s: %w", redactURL(rawURL), err)
	}
	return nil
}

// fetch performs the retrying GET under the catalog cap; downloads with a
// bigger budget (skill archives) call fetchLimit directly.
func (f *fetcher) fetch(ctx context.Context, rawURL, accept string) (int, []byte, error) {
	return f.fetchLimit(ctx, rawURL, accept, maxCatalogBytes)
}

// fetchLimit performs the retrying GET and returns the final status plus the
// body (nil body on non-200), reading at most limit bytes.
func (f *fetcher) fetchLimit(ctx context.Context, rawURL, accept string, limit int64) (int, []byte, error) {
	attempts := f.attempts
	if attempts < 1 {
		attempts = 1
	}
	backoff := f.base
	var status int
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if !f.sleep(ctx, backoff) {
				return 0, nil, ctx.Err()
			}
			backoff *= 2
			if backoff > f.max {
				backoff = f.max
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return 0, nil, fmt.Errorf("hub: build request: %w", err)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("hub: fetch %s: %w", redactURL(rawURL), err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, fmt.Errorf("hub: read catalog: %w", readErr)
		}
		if int64(len(body)) > limit {
			return 0, nil, fmt.Errorf("hub: download from %s exceeds %d bytes", redactURL(rawURL), limit)
		}
		status = resp.StatusCode
		switch {
		case status == http.StatusOK:
			return status, body, nil
		case status == http.StatusTooManyRequests || status >= 500:
			// Retry-After (seconds) takes precedence when it asks for more
			// than the computed backoff — the server knows its own budget.
			if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra > backoff {
				backoff = ra
			}
			continue
		default:
			// Permanent client error: no retry.
			return status, nil, fmt.Errorf("hub: fetch %s: status %d", redactURL(rawURL), status)
		}
	}
	return status, nil, fmt.Errorf("hub: fetch %s: giving up after %d attempts (last status %d)", redactURL(rawURL), attempts, status)
}

// sleep waits d (or ctx cancel); reports whether the wait elapsed.
func (f *fetcher) sleep(ctx context.Context, d time.Duration) bool {
	if f.nowSleep != nil {
		return f.nowSleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// parseRetryAfter parses the seconds form of Retry-After (the only form
// GitHub/Claw emit); unparsable values yield 0 (fall back to backoff).
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	var secs int
	if _, err := fmt.Sscanf(v, "%d", &secs); err != nil || secs <= 0 {
		return 0
	}
	if secs > 300 { // absurd asks don't get to stall the caller for minutes
		secs = 300
	}
	return time.Duration(secs) * time.Second
}

// redactURL strips query strings from URLs before they land in errors (a
// token passed as ?token=… must not leak through error chains).
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.RawQuery != "" {
		u.RawQuery = "…"
	}
	return u.String()
}

// catalogCache is a tiny TTL cache over one fetched catalog file.
type catalogCache[T any] struct {
	mu       sync.Mutex
	fetched  []T
	at       time.Time
	ttl      time.Duration
	fetchNow func() time.Time // injectable clock for tests
}

func (c *catalogCache[T]) get(load func() ([]T, error)) ([]T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.fetchNow != nil {
		now = c.fetchNow()
	}
	if c.fetched != nil && c.ttl > 0 && now.Sub(c.at) < c.ttl {
		return c.fetched, nil
	}
	loaded, err := load()
	if err != nil {
		// Serve stale on refresh failure: a flaky upstream should not blank
		// the browsable catalog.
		if c.fetched != nil {
			return c.fetched, nil
		}
		return nil, err
	}
	c.fetched = loaded
	c.at = now
	return loaded, nil
}

// remoteHub is the shared shape of GitHubMCPHub and ClawSkillHub: catalogs
// fetched over HTTP with retry + TTL cache, filtered and paginated exactly
// like the local FSHub.
type remoteHub struct {
	id, name string

	fetch *fetcher

	mcpCache   catalogCache[MCPCard]
	skillCache catalogCache[SkillCard]

	mcpURL   func() string
	skillURL func() string
}

// ID returns the hub identifier.
func (h *remoteHub) ID() string { return h.id }

// DisplayName returns the user-facing name.
func (h *remoteHub) DisplayName() string { return h.name }

// ListMCPCards returns a filtered, cursor-paginated page of MCP cards.
func (h *remoteHub) ListMCPCards(ctx context.Context, query string, cursor, limit int) ([]MCPCard, int, error) {
	if h.mcpURL == nil {
		return nil, -1, fmt.Errorf("hub: %s has no MCP catalog", h.id)
	}
	cards, err := h.mcpCache.get(func() ([]MCPCard, error) {
		var out []MCPCard
		if err := h.fetch.fetchJSON(ctx, h.mcpURL(), &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, -1, err
	}
	filtered := FilterCards(cards, query,
		func(c MCPCard) string { return c.Name },
		func(c MCPCard) string { return c.Description })
	page, next := Page(filtered, cursor, limit)
	return page, next, nil
}

// ListSkillCards returns a filtered, cursor-paginated page of skill cards.
func (h *remoteHub) ListSkillCards(ctx context.Context, query string, cursor, limit int) ([]SkillCard, int, error) {
	if h.skillURL == nil {
		return nil, -1, fmt.Errorf("hub: %s has no skill catalog", h.id)
	}
	cards, err := h.skillCache.get(func() ([]SkillCard, error) {
		var out []SkillCard
		if err := h.fetch.fetchJSON(ctx, h.skillURL(), &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, -1, err
	}
	filtered := FilterCards(cards, query,
		func(c SkillCard) string { return c.Name },
		func(c SkillCard) string { return c.Description })
	page, next := Page(filtered, cursor, limit)
	return page, next, nil
}

// GitHubMCPHub browses an MCP-server catalog published as JSON on GitHub
// (raw.githubusercontent.com or a GitHub pages URL). The catalog file has the
// same shape as the local FSHub's mcps.json, so publishing a marketplace is
// pushing one file.
type GitHubMCPHub struct {
	remoteHub
	base     string // catalog host; overridable for mirrors/tests
	basePath string // owner/repo/ref/file
}

// NewGitHubMCPHub targets raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>.
func NewGitHubMCPHub(owner, repo, ref, path string) *GitHubMCPHub {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(path, "/"), "./")
	h := &GitHubMCPHub{
		base:     "https://raw.githubusercontent.com",
		basePath: fmt.Sprintf("%s/%s/%s/%s", owner, repo, ref, trimmed),
		remoteHub: remoteHub{
			id:    "github-mcp",
			name:  "GitHub MCP",
			fetch: newFetcher(),
		},
	}
	h.mcpURL = func() string { return h.base + "/" + h.basePath }
	h.remoteHub.mcpCache.ttl = defaultCatalogTTL
	return h
}

// WithBaseURL retargets the catalog host (corporate mirror, cache, tests);
// the owner/repo/ref/file path stays.
func (h *GitHubMCPHub) WithBaseURL(base string) *GitHubMCPHub {
	h.base = strings.TrimRight(base, "/")
	return h
}

// ClawSkillHub browses a skill catalog served by a Claw instance (or any
// static host speaking the same JSON shape). accessToken, when set, is sent
// as a bearer header — never as a query parameter (query strings leak into
// logs; the redacting fetcher only guards error paths).
type ClawSkillHub struct{ remoteHub }

// NewClawSkillHub targets <base>/<skillsPath>.
func NewClawSkillHub(baseURL, skillsPath string, accessToken string) *ClawSkillHub {
	base := strings.TrimRight(baseURL, "/")
	trimmed := strings.TrimPrefix(skillsPath, "/")
	u := strings.TrimSuffix(base+"/"+trimmed, "/")
	if u == "" {
		u = base
	}
	h := &ClawSkillHub{remoteHub{
		id: "claw-skills", name: "Claw Skills",
		fetch:    newFetcher(),
		skillURL: func() string { return u },
	}}
	if accessToken != "" {
		inner := h.fetch
		h.fetch = withBearer(inner, accessToken)
	}
	h.remoteHub.skillCache.ttl = defaultCatalogTTL
	return h
}

// withBearer wraps a fetcher so every request carries the bearer token.
// http.Client accepts a RoundTripper, so the injection happens at the
// transport level.
type bearerTransport struct {
	inner http.RoundTripper
	token string
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone per the RoundTripper contract (Request is reused across retries).
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	if t.inner == nil {
		return http.DefaultTransport.RoundTrip(clone)
	}
	return t.inner.RoundTrip(clone)
}

func withBearer(f *fetcher, token string) *fetcher {
	wrapped := *f
	origClient := f.client
	if origClient == nil {
		origClient = &http.Client{Timeout: 30 * time.Second}
	}
	cl := *origClient
	cl.Transport = &bearerTransport{inner: origClient.Transport, token: token}
	wrapped.client = &cl
	return &wrapped
}

var (
	_ Hub = (*GitHubMCPHub)(nil)
	_ Hub = (*ClawSkillHub)(nil)
)
