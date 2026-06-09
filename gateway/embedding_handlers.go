package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/model"
)

// WithEmbeddingModel registers an embedding model for /api/v1/embeddings.
func (s *Server) WithEmbeddingModel(m model.EmbeddingModel) *Server {
	s.embeddingModel = m
	return s
}

// RegisterEmbeddingRoutes adds OpenAI-compatible embedding endpoints.
func (s *Server) RegisterEmbeddingRoutes() {
	if s.embeddingModel == nil {
		return
	}
	s.mux.HandleFunc("POST /api/v1/embeddings", s.requireAuth(s.handleCreateEmbeddings))
}

type embeddingRequest struct {
	Input any    `json:"input"` // string or []string
	Model string `json:"model,omitempty"`
}

func (s *Server) handleCreateEmbeddings(w http.ResponseWriter, r *http.Request) {
	if s.embeddingModel == nil {
		http.Error(w, "embedding model not configured", http.StatusServiceUnavailable)
		return
	}
	var req embeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inputs, err := normalizeEmbeddingInput(req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.embeddingModel.Embed(r.Context(), inputs)
	if err != nil {
		http.Error(w, fmt.Sprintf("embed: %v", err), http.StatusInternalServerError)
		return
	}
	if req.Model != "" {
		resp.Model = req.Model
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func normalizeEmbeddingInput(v any) ([]string, error) {
	switch t := v.(type) {
	case nil:
		return nil, fmt.Errorf("input is required")
	case string:
		return []string{t}, nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input array must contain strings")
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return t, nil
	default:
		return nil, fmt.Errorf("unsupported input type %T", v)
	}
}
