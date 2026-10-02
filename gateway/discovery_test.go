package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// discoveryTestServer assembles a server with real chat cards (repo's own
// model/cards), embedded tts/embedding cards, and a download signer.
func discoveryTestServer(t *testing.T) *Server {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	cardsDir := filepath.Join(filepath.Dir(file), "..", "model", "cards")
	srv := NewServer(nil).
		WithModelCardsDir(cardsDir).
		WithDownloadTokenSigner(service.NewDownloadTokenSigner([]byte("test-download-secret")))
	srv.RegisterModelRoutes()
	return srv
}

// authenticatedRequest stamps a user onto the request context.
func authenticatedRequest(method, target string, body string, userID string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	return r.WithContext(context.WithValue(r.Context(), service.ContextKeyUserID, userID))
}

// TestModelDiscovery_UnifiedFilteringAndPagination: all three card families
// surface through one API; kind/provider/q filters and cursor pagination
// behave.
func TestModelDiscovery_UnifiedFilteringAndPagination(t *testing.T) {
	srv := discoveryTestServer(t)

	get := func(q string) (cards []ModelCardEntry, next int, total int) {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, authenticatedRequest(http.MethodGet, "/api/v1/model-cards"+q, "", "u1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Cards      []ModelCardEntry `json:"cards"`
			NextCursor int              `json:"next_cursor"`
			Total      int              `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp.Cards, resp.NextCursor, resp.Total
	}

	// All three kinds present.
	cards, _, total := get("")
	kinds := map[string]bool{}
	for _, c := range cards {
		kinds[c.Kind] = true
	}
	for _, k := range []string{"chat", "tts", "embedding"} {
		if !kinds[k] {
			t.Fatalf("kind %s missing from unified discovery (total=%d)", k, total)
		}
	}

	// kind filter.
	ttsCards, _, ttsTotal := get("?kind=tts")
	if ttsTotal != len(ttsCards) || ttsTotal == 0 {
		t.Fatalf("kind filter broken: total=%d len=%d", ttsTotal, len(ttsCards))
	}
	for _, c := range ttsCards {
		if c.Kind != "tts" {
			t.Fatalf("kind filter leaked %q", c.Kind)
		}
	}

	// provider filter.
	provCards, _, _ := get("?kind=embedding&provider=openai")
	if len(provCards) == 0 {
		t.Fatal("provider filter returned nothing for openai embeddings")
	}
	for _, c := range provCards {
		if c.Provider != "openai" {
			t.Fatalf("provider filter leaked %q", c.Provider)
		}
	}

	// q filter.
	qCards, _, _ := get("?kind=chat&q=omni")
	if len(qCards) == 0 {
		t.Fatal("q filter returned nothing for omni")
	}

	// Cursor pagination walks every card exactly once.
	seen := map[string]bool{}
	cursor := 0
	pages := 0
	for {
		page, next, _ := get(fmt.Sprintf("?kind=tts&limit=2&cursor=%d", cursor))
		if len(page) == 0 {
			break
		}
		for _, c := range page {
			key := c.Kind + "/" + c.ID
			if seen[key] {
				t.Fatalf("card %s served twice", key)
			}
			seen[key] = true
		}
		pages++
		if next < 0 {
			break
		}
		cursor = next
	}
	if pages < 2 {
		t.Fatalf("pagination never paged (pages=%d, tts cards=%d)", pages, ttsTotal)
	}
	if len(seen) != ttsTotal {
		t.Fatalf("pagination lost cards: seen=%d total=%d", len(seen), ttsTotal)
	}

	// The legacy /api/v1/models shape keeps its compatibility key.
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, authenticatedRequest(http.MethodGet, "/api/v1/models?kind=tts", "", "u1"))
	var legacy struct {
		Models []ModelCardEntry `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &legacy); err != nil || len(legacy.Models) == 0 {
		t.Fatalf("legacy models key missing: err=%v body=%s", err, rec.Body.String())
	}
}

// TestDownloadTokenFlow: mint → download round trip plus every 18.9
// refusal — expired, cross-tenant, resource mismatch, tampered token,
// unconfigured signer.
func TestDownloadTokenFlow(t *testing.T) {
	srv := discoveryTestServer(t)

	mint := func(user, kind, res string, ttl int) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"kind":%q,"resource_id":%q}`, kind, res)
		if ttl > 0 {
			body = fmt.Sprintf(`{"kind":%q,"resource_id":%q,"ttl_seconds":%d}`, kind, res, ttl)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, authenticatedRequest(http.MethodPost, "/api/v1/models/download-tokens", body, user))
		return rec
	}
	tokenOf := func(rec *httptest.ResponseRecorder) string {
		var resp struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("mint response: %v (%s)", err, rec.Body.String())
		}
		return resp.Token
	}
	download := func(user, kind, res, token string) int {
		rec := httptest.NewRecorder()
		req := authenticatedRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/models/%s/%s/download?token=%s", kind, res, token), "", user)
		srv.ServeHTTP(rec, req)
		return rec.Code
	}

	// Find a real tts card id.
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, authenticatedRequest(http.MethodGet, "/api/v1/model-cards?kind=tts&limit=1", "", "u1"))
	var list struct {
		Cards []ModelCardEntry `json:"cards"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Cards) == 0 {
		t.Fatal("no tts cards for the flow test")
	}
	res := list.Cards[0].ID

	// Happy path.
	m := mint("alice", "tts", res, 0)
	if m.Code != http.StatusCreated {
		t.Fatalf("mint status=%d body=%s", m.Code, m.Body.String())
	}
	if code := download("alice", "tts", res, tokenOf(m)); code != http.StatusOK {
		t.Fatalf("download status=%d", code)
	}

	// Cross-tenant: bob presenting alice's token.
	if code := download("bob", "tts", res, tokenOf(m)); code != http.StatusForbidden {
		t.Fatalf("cross-tenant must 403, got %d", code)
	}

	// Resource mismatch: valid token used for another card.
	if code := download("alice", "tts", "some-other-card", tokenOf(m)); code != http.StatusForbidden {
		t.Fatalf("resource mismatch must 403, got %d", code)
	}

	// Kind mismatch counts as resource mismatch too.
	if code := download("alice", "embedding", res, tokenOf(m)); code != http.StatusForbidden {
		t.Fatalf("kind mismatch must 403, got %d", code)
	}

	// Expired: ttl=1s, verify after it lapses.
	short := mint("alice", "tts", res, 1)
	time.Sleep(1100 * time.Millisecond)
	if code := download("alice", "tts", res, tokenOf(short)); code != http.StatusForbidden {
		t.Fatalf("expired must 403, got %d", code)
	}

	// Tampered token.
	if code := download("alice", "tts", res, tokenOf(m)+"x"); code != http.StatusBadRequest {
		t.Fatalf("tampered token must 400, got %d", code)
	}

	// Unauthenticated caller (no user on the context).
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/models/tts/"+res+"/download?token=x", nil)
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated download must 401, got %d", rec2.Code)
	}

	// Minting a card that does not exist: 404 before any grant.
	if code := mint("alice", "tts", "no-such-card", 0).Code; code != http.StatusNotFound {
		t.Fatalf("unknown card mint must 404, got %d", code)
	}
}

// TestDownloadTokenSignerUnconfigured: without a signer the mint endpoint
// fails closed (503) instead of issuing unsigned grants.
func TestDownloadTokenSignerUnconfigured(t *testing.T) {
	srv := NewServer(nil)
	srv.RegisterModelRoutes()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, authenticatedRequest(http.MethodPost, "/api/v1/models/download-tokens",
		`{"kind":"tts","resource_id":"x"}`, "u1"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured signer must 503, got %d body=%s", rec.Code, rec.Body.String())
	}
}
