package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/model"
)

// WithModelCardsDir sets a directory of YAML model cards exposed via /api/v1/models.
func (s *Server) WithModelCardsDir(dir string) *Server {
	s.modelCardsDir = dir
	return s
}

// RegisterModelRoutes adds the model discovery endpoints (18.9): the
// unified card listing (chat/tts/embedding behind one filtered, paginated
// API), single-card detail, and the download-token flow. TTS/embedding
// cards are embedded, so the routes register even without a chat cards dir.
func (s *Server) RegisterModelRoutes() {
	s.mux.HandleFunc("GET /api/v1/models", s.requireAuth(s.handleModelDiscovery))
	s.mux.HandleFunc("GET /api/v1/model-cards", s.requireAuth(s.handleModelDiscovery))
	s.mux.HandleFunc("GET /api/v1/models/{id}", s.requireAuth(s.handleGetModel))
	s.mux.HandleFunc("POST /api/v1/models/download-tokens", s.requireAuth(s.handleMintDownloadToken))
	s.mux.HandleFunc("GET /api/v1/models/{kind}/{id}/download", s.requireAuth(s.handleCardDownload))
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	cards, err := model.LoadModelCardsFromDir(s.modelCardsDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("load model cards: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cards)
}

func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := s.modelCardsDir + string(os.PathSeparator) + id + ".yaml"
	card, err := model.LoadModelCard(path)
	if err != nil {
		path = s.modelCardsDir + string(os.PathSeparator) + id + ".yml"
		card, err = model.LoadModelCard(path)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("model not found: %v", err), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(card)
}
