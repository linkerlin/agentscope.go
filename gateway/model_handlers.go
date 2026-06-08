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

// RegisterModelRoutes adds model card listing endpoints.
func (s *Server) RegisterModelRoutes() {
	if s.modelCardsDir == "" {
		return
	}
	s.mux.HandleFunc("GET /api/v1/models", s.requireAuth(s.handleListModels))
	s.mux.HandleFunc("GET /api/v1/models/{id}", s.requireAuth(s.handleGetModel))
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
