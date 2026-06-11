package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/credential"
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
	s.mux.HandleFunc("PATCH /api/v1/agents/{id}", s.requireAuth(s.handleUpdateAgent))
	s.mux.HandleFunc("DELETE /api/v1/agents/{id}", s.requireAuth(s.handleDeleteAgent))

	// Sessions
	s.mux.HandleFunc("GET /api/v1/sessions", s.requireAuth(s.handleListSessions))
	s.mux.HandleFunc("POST /api/v1/sessions", s.requireAuth(s.handleCreateSession))
	s.mux.HandleFunc("GET /api/v1/sessions/{id}", s.requireAuth(s.handleGetSession))
	s.mux.HandleFunc("PATCH /api/v1/sessions/{id}", s.requireAuth(s.handleUpdateSession))
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/messages", s.requireAuth(s.handleListSessionMessages))
	s.mux.HandleFunc("DELETE /api/v1/sessions/{id}", s.requireAuth(s.handleDeleteSession))

	// Credentials
	s.mux.HandleFunc("GET /api/v1/credentials", s.requireAuth(s.handleListCredentials))
	s.mux.HandleFunc("GET /api/v1/credentials/schemas", s.requireAuth(s.handleListCredentialSchemas))
	s.mux.HandleFunc("POST /api/v1/credentials", s.requireAuth(s.handleCreateCredential))
	s.mux.HandleFunc("GET /api/v1/credentials/{id}", s.requireAuth(s.handleGetCredential))
	s.mux.HandleFunc("PATCH /api/v1/credentials/{id}", s.requireAuth(s.handleUpdateCredential))
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
	Name         string         `json:"name"`
	ModelID      string         `json:"model_id"`
	Description  string         `json:"description,omitempty"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type updateAgentRequest struct {
	Name         *string        `json:"name,omitempty"`
	ModelID      *string        `json:"model_id,omitempty"`
	Description  *string        `json:"description,omitempty"`
	SystemPrompt *string        `json:"system_prompt,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
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
		ID:           generateID("agent"),
		UserID:       userID,
		Name:         req.Name,
		ModelID:      req.ModelID,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Metadata:     req.Metadata,
		CreatedAt:    time.Now(),
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

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := s.storage.GetAgentConfig(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get agent failed: %v", err), http.StatusNotFound)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if cfg.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req updateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name != nil {
		cfg.Name = *req.Name
	}
	if req.ModelID != nil {
		cfg.ModelID = *req.ModelID
	}
	if req.Description != nil {
		cfg.Description = *req.Description
	}
	if req.SystemPrompt != nil {
		cfg.SystemPrompt = *req.SystemPrompt
	}
	if req.Metadata != nil {
		cfg.Metadata = req.Metadata
	}
	cfg.UpdatedAt = time.Now()
	if err := s.storage.SaveAgentConfig(r.Context(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("update agent failed: %v", err), http.StatusInternalServerError)
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
	userID := service.UserIDFromContext(r.Context())
	if cfg.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	ctx := r.Context()
	if s.backgroundTaskMgr != nil {
		schedules, _ := s.backgroundTaskMgr.ListSchedules(ctx, userID)
		for _, sch := range schedules {
			if sch.AgentID == id {
				_ = s.backgroundTaskMgr.DeleteSchedule(ctx, userID, sch.ID)
			}
		}
	}
	sessions, _ := s.storage.ListSessionsByUser(ctx, userID)
	for _, se := range sessions {
		if se.AgentID == id {
			_ = s.storage.DeleteSession(ctx, se.ID)
		}
	}

	if err := s.storage.DeleteAgentConfig(ctx, id); err != nil {
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
	if agentID := r.URL.Query().Get("agent_id"); agentID != "" {
		filtered := make([]*service.Session, 0, len(sessions))
		for _, se := range sessions {
			if se.AgentID == agentID {
				filtered = append(filtered, se)
			}
		}
		sessions = filtered
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

type createSessionRequest struct {
	Title   string `json:"title"`
	AgentID string `json:"agent_id,omitempty"`
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
		AgentID:   req.AgentID,
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

type updateSessionRequest struct {
	Title       *string `json:"title,omitempty"`
	AgentID     *string `json:"agent_id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
}

type listSessionMessagesResponse struct {
	Messages  []*service.StoredMessage `json:"messages"`
	Total     int                      `json:"total"`
	IsRunning bool                     `json:"is_running"`
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get session failed: %v", err), http.StatusNotFound)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if sess.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req updateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Title != nil {
		sess.Title = *req.Title
	}
	if req.AgentID != nil {
		sess.AgentID = *req.AgentID
	}
	if req.WorkspaceID != nil {
		sess.WorkspaceID = *req.WorkspaceID
	}
	sess.UpdatedAt = time.Now()
	if err := s.storage.SaveSession(r.Context(), sess); err != nil {
		http.Error(w, fmt.Sprintf("update session failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sess)
}

func (s *Server) handleListSessionMessages(w http.ResponseWriter, r *http.Request) {
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

	offset := parseQueryInt(r, "offset", 0)
	limit := parseQueryInt(r, "limit", 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	msgs, err := s.storage.ListMessagesBySession(r.Context(), id, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("list messages failed: %v", err), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []*service.StoredMessage{}
	}

	isRunning := false
	if s.sessionMgr != nil {
		isRunning = s.sessionMgr.IsActive(id)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listSessionMessagesResponse{
		Messages:  msgs,
		Total:     len(msgs),
		IsRunning: isRunning,
	})
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

func (s *Server) handleListCredentialSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := credential.DefaultFactory.ListSchemas()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schemas": schemas,
	})
}

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
	Provider string         `json:"provider"`
	Label    string         `json:"label"`
	Value    string         `json:"value"`
	Data     map[string]any `json:"data"` // preferred for typed credentials (supports /schemas)
}

type updateCredentialRequest struct {
	Label *string `json:"label,omitempty"`
	Value *string `json:"value,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var req createCredentialRequest

	ct := r.Header.Get("Content-Type")
	if ct == "application/x-www-form-urlencoded" || ct == "multipart/form-data" {
		// Support simple HTMX / form posts (used by the lightweight Go Studio)
		r.ParseForm()
		req.Label = r.FormValue("label")
		req.Provider = r.FormValue("provider")

		// Collect data[...] fields into req.Data
		req.Data = make(map[string]any)
		for k, vals := range r.Form {
			if len(k) > 5 && k[:5] == "data[" && k[len(k)-1] == ']' {
				key := k[5 : len(k)-1]
				if len(vals) > 0 {
					req.Data[key] = vals[0]
				}
			}
		}
		// Also allow a raw "data" json field for power users
		if raw := r.FormValue("data"); raw != "" {
			var m map[string]any
			if json.Unmarshal([]byte(raw), &m) == nil {
				for k, v := range m {
					req.Data[k] = v
				}
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	userID := service.UserIDFromContext(r.Context())

	var cred *service.Credential

	if len(req.Data) > 0 {
		// New typed path (recommended, enables dynamic forms via /schemas)
		c, err := credential.DefaultFactory.FromMap(req.Data)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid credential data: %v", err), http.StatusBadRequest)
			return
		}
		if req.Label != "" {
			// allow overriding name from top level for convenience
			// (typed creds carry their own name)
		}
		sc, err := credential.EncryptToService(c, s.cipher)
		if err != nil {
			http.Error(w, fmt.Sprintf("encrypt credential: %v", err), http.StatusInternalServerError)
			return
		}
		sc.UserID = userID
		if sc.Label == "" {
			sc.Label = req.Label
		}
		if sc.Provider == "" {
			sc.Provider = c.Provider()
		}
		cred = sc
	} else {
		// Legacy flat path (provider + value)
		if req.Provider == "" || req.Value == "" {
			http.Error(w, "provider and value are required (or use data for typed credentials)", http.StatusBadRequest)
			return
		}
		encrypted := req.Value
		if s.cipher != nil {
			enc, err := s.cipher.Encrypt(req.Value)
			if err != nil {
				http.Error(w, fmt.Sprintf("encryption failed: %v", err), http.StatusInternalServerError)
				return
			}
			encrypted = enc
		}
		cred = &service.Credential{
			ID:        generateID("cred"),
			UserID:    userID,
			Provider:  req.Provider,
			Label:     req.Label,
			Encrypted: encrypted,
			CreatedAt: time.Now(),
		}
	}

	if err := s.storage.SaveCredential(r.Context(), cred); err != nil {
		http.Error(w, fmt.Sprintf("save credential failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(cred)
}

func (s *Server) handleGetCredential(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cred)
}

func (s *Server) handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cred, err := s.storage.GetCredential(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("get credential failed: %v", err), http.StatusNotFound)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if cred.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req updateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Label != nil {
		cred.Label = *req.Label
	}
	if req.Value != nil {
		encrypted := *req.Value
		if s.cipher != nil {
			enc, err := s.cipher.Encrypt(*req.Value)
			if err != nil {
				http.Error(w, fmt.Sprintf("encryption failed: %v", err), http.StatusInternalServerError)
				return
			}
			encrypted = enc
		}
		cred.Encrypted = encrypted
	}
	cred.UpdatedAt = time.Now()
	if err := s.storage.SaveCredential(r.Context(), cred); err != nil {
		http.Error(w, fmt.Sprintf("update credential failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
