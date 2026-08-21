package gateway

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/skill"
)

type addSkillRequest struct {
	SkillPath string `json:"skill_path"`
}

type selectSkillsRequest struct {
	Names []string `json:"names"`
}

func (s *Server) workspaceQuery(r *http.Request) (userID, agentID, sessionID string, ok bool) {
	userID = service.UserIDFromContext(r.Context())
	agentID = r.URL.Query().Get("agent_id")
	sessionID = r.URL.Query().Get("session_id")
	return userID, agentID, sessionID, agentID != "" && sessionID != ""
}

func (s *Server) handleWorkspaceMCP(w http.ResponseWriter, r *http.Request) {
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		mcps, err := s.workspaceMgr.ListMCPs(r.Context(), s.storage, userID, agentID, sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if mcps == nil {
			mcps = []MCPStatus{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mcps)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reg, err := parseMCPAddRequest(body, s.workspaceMgr.defaultMCPGatewayURL, s.workspaceMgr.defaultMCPGatewayToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.workspaceMgr.AddMCP(r.Context(), s.storage, userID, agentID, sessionID, reg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkspaceMCPDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return
	}
	name := r.PathValue("mcp_name")
	if err := s.workspaceMgr.RemoveMCP(r.Context(), s.storage, userID, agentID, sessionID, name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWorkspaceSkill(w http.ResponseWriter, r *http.Request) {
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		skills, err := s.workspaceMgr.ListSkills(r.Context(), s.storage, userID, agentID, sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if skills == nil {
			skills = []*skill.AgentSkill{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(skills)
	case http.MethodPost:
		var req addSkillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.workspaceMgr.AddSkill(r.Context(), s.storage, userID, agentID, sessionID, req.SkillPath); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkspaceSkillDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return
	}
	name := r.PathValue("skill_name")
	if err := s.workspaceMgr.RemoveSkill(r.Context(), s.storage, userID, agentID, sessionID, name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleWorkspaceAgentSkills lists the agent-level skill library.
// GET /workspace/agent_skills?agent_id=
func (s *Server) handleWorkspaceAgentSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}
	skills := s.workspaceMgr.ListAgentSkills(userID, agentID)
	if skills == nil {
		skills = []*skill.AgentSkill{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(skills)
}

// handleWorkspaceSkillSelect replaces the session's active skills with a
// named subset of the agent-level skill library.
// POST /workspace/skill/select?agent_id=&session_id=
func (s *Server) handleWorkspaceSkillSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return
	}
	var req selectSkillsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.workspaceMgr.SelectSkills(r.Context(), s.storage, userID, agentID, sessionID, req.Names); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RegisterWorkspaceRoutes adds PyV2-aligned /workspace endpoints.
func (s *Server) RegisterWorkspaceRoutes() {
	if s.workspaceMgr == nil {
		return
	}
	s.mux.HandleFunc("/workspace/mcp", s.requireAuth(s.handleWorkspaceMCP))
	s.mux.HandleFunc("/workspace/mcp/{mcp_name}", s.requireAuth(s.handleWorkspaceMCPDelete))
	s.mux.HandleFunc("/workspace/skill", s.requireAuth(s.handleWorkspaceSkill))
	s.mux.HandleFunc("/workspace/skill/{skill_name}", s.requireAuth(s.handleWorkspaceSkillDelete))
	s.mux.HandleFunc("/workspace/skill/select", s.requireAuth(s.handleWorkspaceSkillSelect))
	s.mux.HandleFunc("/workspace/agent_skills", s.requireAuth(s.handleWorkspaceAgentSkills))
	s.mux.HandleFunc("/workspace/list_dir", s.requireAuth(s.handleWorkspaceListDir))
	s.mux.HandleFunc("/workspace/read_file", s.requireAuth(s.handleWorkspaceReadFile))
	s.mux.HandleFunc("/workspace/status", s.requireAuth(s.handleWorkspaceStatus))
}

// WithWorkspaceManager attaches a workspace manager for session workspace HTTP APIs.
func (s *Server) WithWorkspaceManager(m *WorkspaceManager) *Server {
	s.workspaceMgr = m
	return s
}
