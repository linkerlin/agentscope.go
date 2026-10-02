// gateway/discovery.go — unified model discovery (18.9): chat / TTS /
// embedding model cards behind one filtered, cursor-paginated API, plus the
// download-token flow that gates card detail behind a per-user, per-resource,
// time-bounded grant.
package gateway

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tts"
)

// ModelCardEntry is the unified discovery DTO: the fields every card kind
// shares, plus the kind discriminator. Full per-kind detail stays behind the
// download endpoint.
type ModelCardEntry struct {
	Kind        string `json:"kind"` // "chat" | "tts" | "embedding"
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	DisplayName string `json:"display_name"`
}

// discoverCards aggregates all three card families, applies the filters and
// returns a stable ordering (kind, provider, id) so cursors behave.
func (s *Server) discoverCards(kind, provider, query string) []ModelCardEntry {
	kinds := map[string]bool{}
	switch kind {
	case "":
		kinds["chat"], kinds["tts"], kinds["embedding"] = true, true, true
	case "chat", "tts", "embedding":
		kinds[kind] = true
	}
	var entries []ModelCardEntry

	if kinds["chat"] {
		cards, err := model.LoadModelCardsFromDir(s.modelCardsDir)
		if err == nil {
			for _, c := range cards {
				entries = append(entries, ModelCardEntry{Kind: "chat", ID: c.ID, Provider: c.Provider, DisplayName: c.DisplayName})
			}
		}
	}
	if kinds["tts"] {
		if cards, err := tts.ListModelCards(); err == nil {
			for _, c := range cards {
				entries = append(entries, ModelCardEntry{Kind: "tts", ID: c.ID, Provider: c.Provider, DisplayName: c.DisplayName})
			}
		}
	}
	if kinds["embedding"] {
		if cards, err := embedding.ListModelCards(); err == nil {
			for _, c := range cards {
				entries = append(entries, ModelCardEntry{Kind: "embedding", ID: c.ID, Provider: c.Provider, DisplayName: c.DisplayName})
			}
		}
	}

	// Filters: provider exact match, query substring on id/name.
	filtered := make([]ModelCardEntry, 0, len(entries))
	for _, e := range entries {
		if provider != "" && e.Provider != provider {
			continue
		}
		if query != "" {
			q := strings.ToLower(query)
			if !strings.Contains(strings.ToLower(e.ID), q) && !strings.Contains(strings.ToLower(e.DisplayName), q) {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Kind != filtered[j].Kind {
			return filtered[i].Kind < filtered[j].Kind
		}
		if filtered[i].Provider != filtered[j].Provider {
			return filtered[i].Provider < filtered[j].Provider
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered
}

// handleModelDiscovery serves GET /api/v1/model-cards: unified filtering
// (kind/provider/q) and cursor pagination. Also mounted at /api/v1/models
// with a compatibility "models" key for the pre-18.9 console shape.
func (s *Server) handleModelDiscovery(w http.ResponseWriter, r *http.Request) {
	entries := s.discoverCards(
		strings.ToLower(r.URL.Query().Get("kind")),
		r.URL.Query().Get("provider"),
		r.URL.Query().Get("q"),
	)
	limit := parseQueryLimit(r, 50)
	cursor := parseQueryCursor(r)
	start := 0
	if cursor > start {
		start = cursor
	}
	end := start + limit
	if end > len(entries) {
		end = len(entries)
	}
	next := -1
	if end < len(entries) {
		next = end
	}
	page := entries[start:end]
	if page == nil {
		page = []ModelCardEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cards":       page,
		"models":      page, // compatibility key (pre-18.9 consumers)
		"next_cursor": next,
		"total":       len(entries),
	})
}

// handleMintDownloadToken serves POST /api/v1/models/download-tokens: the
// authenticated user mints a short-lived grant for exactly one card. The
// signer must be configured at assembly (AppConfig.DownloadTokenSecret /
// WithDownloadTokenSigner); without it the endpoint fails closed.
func (s *Server) handleMintDownloadToken(w http.ResponseWriter, r *http.Request) {
	if s.downloadSigner == nil || !s.downloadSigner.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "download tokens not configured (set the signing secret at assembly)",
		})
		return
	}
	var req struct {
		Kind       string `json:"kind"`
		ResourceID string `json:"resource_id"`
		TTLSeconds int    `json:"ttl_seconds,omitempty"`
	}
	if err := decodeJSONLimit(w, r, &req); err != nil {
		writeBodyLimitError(w, err)
		return
	}
	switch req.Kind {
	case "chat", "tts", "embedding":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "kind must be chat, tts or embedding"})
		return
	}
	if req.ResourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "resource_id is required"})
		return
	}
	// The granted card must exist (404 before minting anything).
	entries := s.discoverCards(req.Kind, "", "")
	found := false
	for _, e := range entries {
		if e.ID == req.ResourceID {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model card not found"})
		return
	}

	userID := service.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required"})
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	token, err := s.downloadSigner.Mint(userID, req.Kind, req.ResourceID, ttl)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":       token,
		"kind":        req.Kind,
		"resource_id": req.ResourceID,
		"ttl_seconds": int(ttl.Seconds()),
	})
}

// handleCardDownload serves GET /api/v1/models/{kind}/{id}/download?token=…:
// full card detail for the exact card the grant names, to the exact user it
// was minted for, while it is still valid. Every 18.9 refusal is
// distinguishable: 401 unauthenticated, 400 malformed/forged, 403 expired /
// cross-tenant / resource mismatch, 404 unknown card.
func (s *Server) handleCardDownload(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(r.PathValue("kind"))
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")

	userID := service.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required"})
		return
	}
	if s.downloadSigner == nil || !s.downloadSigner.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "download tokens not configured"})
		return
	}

	// Token BEFORE card lookup: the refusal verdicts (403 mismatch) must not
	// leak whether a resource exists (404) — the 22.x existence discipline.
	// A holder of a grant for resource A probing resource B learns "mismatch",
	// never "not found".
	err := s.downloadSigner.Verify(token, userID, kind, id, time.Now())
	switch {
	case err == nil:
	case err == service.ErrDownloadTokenMalformed:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	case err == service.ErrDownloadTokenExpired,
		err == service.ErrDownloadTokenForbidden,
		err == service.ErrDownloadTokenMismatch:
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	card, ok := s.loadCardDetail(kind, id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model card not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "card": card})
}

// loadCardDetail resolves the full card for one kind/id.
func (s *Server) loadCardDetail(kind, id string) (any, bool) {
	switch kind {
	case "chat":
		card, err := model.LoadModelCard(s.modelCardsDir + "/" + id + ".yaml")
		if err != nil {
			card, err = model.LoadModelCard(s.modelCardsDir + "/" + id + ".yml")
		}
		if err != nil {
			return nil, false
		}
		return card, true
	case "tts":
		cards, err := tts.ListModelCards()
		if err != nil {
			return nil, false
		}
		for _, c := range cards {
			if c.ID == id {
				return c, true
			}
		}
		return nil, false
	case "embedding":
		cards, err := embedding.ListModelCards()
		if err != nil {
			return nil, false
		}
		for _, c := range cards {
			if c.ID == id {
				return c, true
			}
		}
		return nil, false
	}
	return nil, false
}

// parseQueryLimit reads ?limit= with a default and a sane ceiling.
func parseQueryLimit(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			return n
		}
	}
	return def
}

// parseQueryCursor reads ?cursor= (-1 and garbage both mean "from the top").
func parseQueryCursor(r *http.Request) int {
	if v := r.URL.Query().Get("cursor"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// WithDownloadTokenSigner arms the model download-token flow (18.9). Call
// before RegisterModelRoutes; without it the mint endpoint fails closed
// (503) rather than issuing unsigned grants.
func (s *Server) WithDownloadTokenSigner(signer *service.DownloadTokenSigner) *Server {
	s.downloadSigner = signer
	return s
}
