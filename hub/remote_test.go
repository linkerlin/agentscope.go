package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpserver "github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// shortBackoff installs a near-zero backoff on f and returns a pointer to
// the sleeps it performed (a pointer, because the recording closure appends
// after the return — a returned slice value would stay the nil snapshot).
func shortBackoff(f *fetcher) *[]time.Duration {
	sleeps := []time.Duration{}
	f.base = time.Millisecond
	f.max = 2 * time.Millisecond
	f.nowSleep = func(ctx context.Context, d time.Duration) bool {
		sleeps = append(sleeps, d)
		return ctx.Err() == nil
	}
	return &sleeps
}

// TestFetcherRetryContract locks the 18.7 retry policy: 429/5xx retry with
// backoff, other 4xx fail fast, attempts are bounded, Retry-After can only
// lengthen (never shorten) a wait, and the body cap rejects oversize
// downloads mid-stream.
func TestFetcherRetryContract(t *testing.T) {
	t.Run("429 then success", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hits.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Write([]byte(`{"ok":true}`))
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		shortBackoff(f)
		var out map[string]bool
		if err := f.fetchJSON(context.Background(), srv.URL, &out); err != nil {
			t.Fatal(err)
		}
		if !out["ok"] || hits.Load() != 2 {
			t.Fatalf("expected retry-then-success, hits=%d", hits.Load())
		}
	})

	t.Run("5xx then success", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hits.Add(1) <= 2 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Write([]byte(`[]`))
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		shortBackoff(f)
		var out []map[string]bool
		if err := f.fetchJSON(context.Background(), srv.URL, &out); err != nil {
			t.Fatal(err)
		}
		if hits.Load() != 3 {
			t.Fatalf("expected 2 retries, hits=%d", hits.Load())
		}
	})

	t.Run("404 fails fast without retry", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			http.Error(w, "gone", http.StatusNotFound)
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		shortBackoff(f)
		var out any
		if err := f.fetchJSON(context.Background(), srv.URL, &out); err == nil {
			t.Fatal("404 must fail")
		}
		if hits.Load() != 1 {
			t.Fatalf("404 must not retry, hits=%d", hits.Load())
		}
	})

	t.Run("persistent 429 exhausts attempts", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		shortBackoff(f)
		var out any
		if err := f.fetchJSON(context.Background(), srv.URL, &out); err == nil {
			t.Fatal("persistent 429 must fail")
		}
		if hits.Load() != int32(defaultMaxAttempts) {
			t.Fatalf("expected %d attempts, hits=%d", defaultMaxAttempts, hits.Load())
		}
	})

	t.Run("Retry-After lengthens backoff only", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hits.Add(1) == 1 {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Write([]byte(`[]`))
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		sleeps := shortBackoff(f)
		var out []any
		if err := f.fetchJSON(context.Background(), srv.URL, &out); err != nil {
			t.Fatal(err)
		}
		if len(*sleeps) != 1 || (*sleeps)[0] != 2*time.Second {
			t.Fatalf("Retry-After must override short backoff, sleeps=%v", *sleeps)
		}
	})

	t.Run("oversize download rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			big := make([]byte, 512)
			w.Write(big)
		}))
		defer srv.Close()
		f := newFetcher()
		f.client = srv.Client()
		if _, _, err := f.fetchLimit(context.Background(), srv.URL, "", 256); err == nil {
			t.Fatal("oversize must fail")
		}
	})
}

// TestGitHubMCPHubPaginationAndCache: the GitHub hub serves cursor pages over
// the remote catalog and caches it for the TTL (one upstream hit).
func TestGitHubMCPHubPaginationAndCache(t *testing.T) {
	cards := make([]MCPCard, 25)
	for i := range cards {
		cards[i] = MCPCard{Card: Card{ID: fmt.Sprintf("m%d", i), Name: fmt.Sprintf("mcp-%d", i)}}
	}
	body, _ := json.Marshal(cards)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/acme/registry/main/mcps.json" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	h := NewGitHubMCPHub("acme", "registry", "main", "mcps.json").WithBaseURL(srv.URL)
	shortBackoff(h.fetch)

	ctx := context.Background()
	seen := 0
	cursor := 0
	for {
		page, next, err := h.ListMCPCards(ctx, "", cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		seen += len(page)
		if next < 0 {
			break
		}
		cursor = next
	}
	if seen != 25 {
		t.Fatalf("pagination lost entries: seen=%d", seen)
	}
	if hits.Load() != 1 {
		t.Fatalf("catalog must be cached across pages, hits=%d", hits.Load())
	}

	// TTL expiry refetches.
	h.mcpCache.fetchNow = func() time.Time {
		return time.Now().Add(defaultCatalogTTL + time.Minute)
	}
	if _, _, err := h.ListMCPCards(ctx, "", 0, 5); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expired cache must refetch, hits=%d", hits.Load())
	}
}

// TestClawSkillHubBearerAndQueryFilter: the Claw hub sends its token as a
// bearer header and filters/paginates the remote skill catalog.
func TestClawSkillHubBearerAndQueryFilter(t *testing.T) {
	skills := []SkillCard{
		{Card: Card{ID: "s1", Name: "pdf-report", Description: "builds pdf reports"}},
		{Card: Card{ID: "s2", Name: "sql-helper", Description: "sql assistance"}},
		{Card: Card{ID: "s3", Name: "pdf-sign", Description: "sign pdfs"}},
	}
	body, _ := json.Marshal(skills)
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		if r.URL.Path != "/api/skills.json" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	h := NewClawSkillHub(srv.URL, "api/skills.json", "claw-token")
	shortBackoff(h.fetch)

	page, next, err := h.ListSkillCards(context.Background(), "pdf", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || next != -1 {
		t.Fatalf("query filter wrong: page=%d next=%d", len(page), next)
	}
	if gotAuth.Load() != "Bearer claw-token" {
		t.Fatalf("bearer header missing: %v", gotAuth.Load())
	}
}

// TestClawSkillHubStaleOnRefreshFailure: a cached catalog survives an
// upstream outage (serve-stale), but the first-ever fetch failing surfaces
// the error.
func TestClawSkillHubStaleOnRefreshFailure(t *testing.T) {
	var fail atomic.Bool
	skills := []SkillCard{{Card: Card{ID: "s1", Name: "keeper"}}}
	body, _ := json.Marshal(skills)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	h := NewClawSkillHub(srv.URL, "skills.json", "")
	shortBackoff(h.fetch)
	ctx := context.Background()

	if _, _, err := h.ListSkillCards(ctx, "", 0, 10); err != nil {
		t.Fatal(err)
	}
	// Force expiry, break upstream: stale catalog still served.
	fail.Store(true)
	h.skillCache.fetchNow = func() time.Time { return time.Now().Add(2 * defaultCatalogTTL) }
	page, _, err := h.ListSkillCards(ctx, "", 0, 10)
	if err != nil {
		t.Fatalf("stale catalog must survive refresh failure: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("stale page wrong: %+v", page)
	}
}

// TestTemplateValidateAndExpand locks the 18.7 config-template contract.
func TestTemplateValidateAndExpand(t *testing.T) {
	card := MCPCard{
		Card: Card{ID: "gh", Name: "github"},
		Spec: specOf(map[string]string{
			"command": "npx",
			"arg0":    "-t",
			"arg1":    "${GITHUB_TOKEN}",
			"env_pat": "ghp_${GITHUB_TOKEN}",
			"url":     "https://${GH_HOST}/mcp",
		}),
		RequiredEnv: []string{"GITHUB_TOKEN", "REGION"},
	}

	// Missing values are listed exhaustively (spec refs + RequiredEnv),
	// sorted.
	missing := ValidateMCPCard(card, nil)
	if len(missing) != 3 || missing[0] != "GH_HOST" || missing[1] != "GITHUB_TOKEN" || missing[2] != "REGION" {
		t.Fatalf("missing list wrong: %v", missing)
	}

	// With every referenced value present the card validates (REGION comes
	// from RequiredEnv even though the spec never interpolates it).
	if missing := ValidateMCPCard(card, map[string]string{"GITHUB_TOKEN": "t", "GH_HOST": "mcp.example.com", "REGION": "eu"}); len(missing) != 0 {
		t.Fatalf("complete values must validate, missing=%v", missing)
	}

	// Expand fills every string position.
	expanded := ExpandSpec(card.Spec, map[string]string{"GITHUB_TOKEN": "t", "GH_HOST": "mcp.example.com"})
	if expanded.Args[1] != "t" {
		t.Fatalf("arg placeholder not expanded: %q", expanded.Args[1])
	}
	if expanded.Env["GITHUB_PAT"] != "ghp_t" {
		t.Fatalf("env placeholder not expanded: %v", expanded.Env)
	}
	if expanded.URL != "https://mcp.example.com/mcp" {
		t.Fatalf("url placeholder not expanded: %q", expanded.URL)
	}

	// Unvalued placeholders stay verbatim (never half-filled).
	partial := ExpandSpec(card.Spec, map[string]string{"GH_HOST": "mcp.example.com"})
	if partial.Args[1] != "${GITHUB_TOKEN}" {
		t.Fatalf("unvalued placeholder must stay verbatim, got %q", partial.Args[1])
	}
}

// specOf builds a ServerSpec from shorthand strings for template tests.
func specOf(m map[string]string) mcpserver.ServerSpec {
	return mcpserver.ServerSpec{
		Command: m["command"],
		Args:    []string{m["arg0"], m["arg1"]},
		Env:     map[string]string{"GITHUB_PAT": m["env_pat"]},
		URL:     m["url"],
	}
}

// TestInstallMCPsWithValuesValidatesFirst: one invalid card fails the whole
// install BEFORE any connection is attempted.
func TestInstallMCPsWithValuesValidatesFirst(t *testing.T) {
	bad := MCPCard{
		Card: Card{ID: "bad"},
		Spec: mcpserver.ServerSpec{Command: "npx", Args: []string{"${MISSING_TOKEN}"}},
	}
	_, _, err := InstallMCPsWithValues(context.Background(), []MCPCard{bad}, nil)
	if err == nil {
		t.Fatal("missing template values must fail the install")
	}
	if want := "MISSING_TOKEN"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error must name the missing variable %q: %v", want, err)
	}
}

// TestInstallSkillRetriesThenExtracts: the skill download retries 429 and
// still lands in the hardened extractor (safeJoin stays in force).
func TestInstallSkillRetriesThenExtracts(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{"skill/SKILL.md": "# hello"}, false)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write(zipBytes)
	}))
	defer srv.Close()

	restore := installFetcher
	installFetcher = newFetcher()
	sleeps := shortBackoff(installFetcher)
	defer func() { installFetcher = restore }()

	dir := t.TempDir()
	if err := InstallSkill(context.Background(), SkillCard{Card: Card{ID: "sk"}, ArchiveURL: srv.URL}, dir); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("download must retry once, hits=%d", hits.Load())
	}
	if len(*sleeps) == 0 {
		t.Fatal("retry must have backed off")
	}
	data, err := os.ReadFile(filepath.Join(dir, "skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# hello") {
		t.Fatalf("extracted content wrong: %q", data)
	}
}
