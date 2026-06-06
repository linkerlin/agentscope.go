package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// RegisterServiceRoutes registers management CRUD endpoints for agents, sessions, and credentials.
// All routes require authentication and are scoped to the current user.
func (s *Server) RegisterServiceRoutes() {
	if s.storage == nil {
		return
	}

	// Agent configs
	s.mux.HandleFunc("GET /api/v1/agents", s.requireAuth(s.handleListAgents))
	s.mux.HandleFunc("POST /api/v1/agents", s.requireAuth(s.handleCreateAgent))
	s.mux.HandleFunc("GET /api/v1/agents/{id}", s.requireAuth(s.handleGetAgent))
	s.mux.HandleFunc("DELETE /api/v1/agents/{id}", s.requireAuth(s.handleDeleteAgent))

	// Sessions
	s.mux.HandleFunc("GET /api/v1/sessions", s.requireAuth(s.handleListSessions))
	s.mux.HandleFunc("POST /api/v1/sessions", s.requireAuth(s.handleCreateSession))
	s.mux.HandleFunc("GET /api/v1/sessions/{id}", s.requireAuth(s.handleGetSession))
	s.mux.HandleFunc("DELETE /api/v1/sessions/{id}", s.requireAuth(s.handleDeleteSession))

	// Credentials
	s.mux.HandleFunc("GET /api/v1/credentials", s.requireAuth(s.handleListCredentials))
	s.mux.HandleFunc("POST /api/v1/credentials", s.requireAuth(s.handleCreateCredential))
	s.mux.HandleFunc("DELETE /api/v1/credentials/{id}", s.requireAuth(s.handleDeleteCredential))
}

// --- Agent Configs ---

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	userID := service.UserIDFromContext(r.Context())
	agents, err := s.storage.ListAgentConfigsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("list agents failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(agents)
}

type createAgentRequest struct {
	Name        string         `json:"name"`
	ModelID     string         `json:"model_id"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	userID := service.UserIDFromContext(r.Context())
	cfg := &service.AgentConfig{
		ID:          generateID("agent"),
		UserID:      userID,
		Name:        req.Name,
		ModelID:     req.ModelID,
		Description: req.Description,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}
	if err := s.storage.SaveAgentConfig(r.Context(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("save agent failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := s.storage.GetAgentConfig(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get agent failed: %v", err), http.StatusNotFound)
		return
	}
	// Verify ownership.
	if cfg.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Verify ownership before delete.
	cfg, err := s.storage.GetAgentConfig(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get agent failed: %v", err), http.StatusNotFound)
		return
	}
	if cfg.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.storage.DeleteAgentConfig(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("delete agent failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Sessions ---

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	userID := service.UserIDFromContext(r.Context())
	sessions, err := s.storage.ListSessionsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("list sessions failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

type createSessionRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	userID := service.UserIDFromContext(r.Context())
	sess := &service.Session{
		ID:        generateID("sess"),
		UserID:    userID,
		Title:     req.Title,
		CreatedAt: time.Now(),
	}
	if err := s.storage.SaveSession(r.Context(), sess); err != nil {
		http.Error(w, fmt.Sprintf("save session failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get session failed: %v", err), http.StatusNotFound)
		return
	}
	if sess.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get session failed: %v", err), http.StatusNotFound)
		return
	}
	if sess.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.storage.DeleteSession(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("delete session failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Credentials ---

func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	userID := service.UserIDFromContext(r.Context())
	creds, err := s.storage.ListCredentialsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("list credentials failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(creds)
}

type createCredentialRequest struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Value    string `json:"value"`
}

func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var req createCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Provider == "" || req.Value == "" {
		http.Error(w, "provider and value are required", http.StatusBadRequest)
		return
	}

	userID := service.UserIDFromContext(r.Context())
	cred := &service.Credential{
		ID:        generateID("cred"),
		UserID:    userID,
		Provider:  req.Provider,
		Label:     req.Label,
		Encrypted: req.Value, // In production, encrypt this.
		CreatedAt: time.Now(),
	}
	if err := s.storage.SaveCredential(r.Context(), cred); err != nil {
		http.Error(w, fmt.Sprintf("save credential failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(cred)
}

func (s *Server) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cred, err := s.storage.GetCredential(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get credential failed: %v", err), http.StatusNotFound)
		return
	}
	if cred.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.storage.DeleteCredential(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("delete credential failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
