package gateway

import (
	"net/http"
	"strconv"

	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// WithHubs registers marketplace hubs for the hub HTTP API.
func (s *Server) WithHubs(hubs ...hub.Hub) *Server {
	s.hubs = hubs
	return s
}

// RegisterHubRoutes registers the hub browsing + install endpoints. No-op when
// no hubs are registered.
func (s *Server) RegisterHubRoutes() {
	if len(s.hubs) == 0 {
		return
	}
	s.mux.HandleFunc("GET /api/v1/hubs", s.requireAuth(s.handleListHubs))
	s.mux.HandleFunc("GET /api/v1/hubs/{id}/mcps", s.requireAuth(s.handleListHubMCPs))
	s.mux.HandleFunc("GET /api/v1/hubs/{id}/skills", s.requireAuth(s.handleListHubSkills))
	s.mux.HandleFunc("POST /api/v1/hubs/{id}/mcps/{card}/install", s.requireAuth(s.handleInstallMCP))
	s.mux.HandleFunc("POST /api/v1/hubs/{id}/skills/{card}/install", s.requireAuth(s.handleInstallSkill))
}

func (s *Server) findHub(id string) hub.Hub {
	for _, h := range s.hubs {
		if h.ID() == id {
			return h
		}
	}
	return nil
}

func parsePaging(r *http.Request) (cursor, limit int) {
	cursor, _ = strconv.Atoi(r.URL.Query().Get("cursor"))
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	return cursor, limit
}

func (s *Server) handleListHubs(w http.ResponseWriter, r *http.Request) {
	type hubInfo struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	out := make([]hubInfo, 0, len(s.hubs))
	for _, h := range s.hubs {
		out = append(out, hubInfo{ID: h.ID(), DisplayName: h.DisplayName()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"hubs": out})
}

func (s *Server) handleListHubMCPs(w http.ResponseWriter, r *http.Request) {
	h := s.findHub(r.PathValue("id"))
	if h == nil {
		http.Error(w, "hub not found", http.StatusNotFound)
		return
	}
	cursor, limit := parsePaging(r)
	cards, next, err := h.ListMCPCards(r.Context(), r.URL.Query().Get("q"), cursor, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards, "next_cursor": next})
}

func (s *Server) handleListHubSkills(w http.ResponseWriter, r *http.Request) {
	h := s.findHub(r.PathValue("id"))
	if h == nil {
		http.Error(w, "hub not found", http.StatusNotFound)
		return
	}
	cursor, limit := parsePaging(r)
	cards, next, err := h.ListSkillCards(r.Context(), r.URL.Query().Get("q"), cursor, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards, "next_cursor": next})
}

func (s *Server) handleInstallMCP(w http.ResponseWriter, r *http.Request) {
	h := s.findHub(r.PathValue("id"))
	if h == nil {
		http.Error(w, "hub not found", http.StatusNotFound)
		return
	}
	cardID := r.PathValue("card")
	// find the card
	page, _, err := h.ListMCPCards(r.Context(), "", 0, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var target *hub.MCPCard
	for i := range page {
		if page[i].ID == cardID {
			target = &page[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "card not found", http.StatusNotFound)
		return
	}
	mgr, results := hub.InstallMCPs(r.Context(), []hub.MCPCard{*target})
	defer mcp.CloseManager(mgr)
	if len(results) > 0 && results[0].Err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"card_id": cardID, "error": results[0].Err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"card_id": cardID, "installed": true, "tools": results[0].Tools,
	})
}

func (s *Server) handleInstallSkill(w http.ResponseWriter, r *http.Request) {
	h := s.findHub(r.PathValue("id"))
	if h == nil {
		http.Error(w, "hub not found", http.StatusNotFound)
		return
	}
	cardID := r.PathValue("card")
	page, _, err := h.ListSkillCards(r.Context(), "", 0, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var target *hub.SkillCard
	for i := range page {
		if page[i].ID == cardID {
			target = &page[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "card not found", http.StatusNotFound)
		return
	}
	destDir := r.URL.Query().Get("dest")
	if destDir == "" {
		destDir = ".hubs/" + h.ID() + "/skills"
	}
	if err := hub.InstallSkill(r.Context(), *target, destDir); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"card_id": cardID, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{cardID: cardID, "installed": true, "dest": destDir})
}
