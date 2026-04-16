package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// chatRequest is the expected JSON body for /chat and /chat/stream.
type chatRequest struct {
	Text string `json:"text"`
}

// streamEvent is a single SSE event sent to the client.
type streamEvent struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
}

// Server exposes an agent over HTTP with non-streaming and SSE streaming endpoints.
type Server struct {
	agent agent.Agent
	mux   *http.ServeMux
}

// NewServer creates a gateway HTTP server for the given agent.
func NewServer(a agent.Agent) *Server {
	s := &Server{agent: a, mux: http.NewServeMux()}
	s.mux.HandleFunc("/chat", s.handleChat)
	s.mux.HandleFunc("/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/chat/ws", s.handleChatWS)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := parseChatRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
	resp, err := s.agent.Call(r.Context(), msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"role":    "assistant",
		"content": resp.GetTextContent(),
	})
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := parseChatRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
	ch, err := s.agent.CallStream(r.Context(), msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent stream error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for chunk := range ch {
		if chunk == nil {
			continue
		}
		ev := streamEvent{Delta: chunk.GetTextContent()}
		data, _ := json.Marshal(ev)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	// Final done event
	data, _ := json.Marshal(streamEvent{Done: true})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func parseChatRequest(body io.ReadCloser) (*chatRequest, error) {
	defer body.Close()
	var req chatRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}
	if req.Text == "" {
		return nil, errors.New("text is required")
	}
	return &req, nil
}
