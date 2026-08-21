package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// maxReadFileBytes caps read_file responses to protect against huge artifacts.
const maxReadFileBytes = 5 << 20 // 5 MiB

type fileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type workspaceStatus struct {
	Dir        string   `json:"dir"`
	IsGitRepo  bool     `json:"is_git_repo"`
	GitBranch  string   `json:"git_branch,omitempty"`
	GitChanges []string `json:"git_changes,omitempty"`
}

// workspaceSafeJoin resolves rel (slash-separated, relative) against root,
// rejecting absolute paths and traversal outside root.
func workspaceSafeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "\\") || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pAbs, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	if pAbs != rootAbs && !strings.HasPrefix(pAbs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return pAbs, nil
}

// workspaceFromQuery resolves the session workspace from agent_id/session_id
// query params, writing the HTTP error and returning false on failure.
func (s *Server) workspaceFromQuery(w http.ResponseWriter, r *http.Request) (*SessionWorkspace, bool) {
	if s.workspaceMgr == nil || s.storage == nil {
		http.Error(w, "workspace manager not configured", http.StatusServiceUnavailable)
		return nil, false
	}
	userID, agentID, sessionID, ok := s.workspaceQuery(r)
	if !ok {
		http.Error(w, "agent_id and session_id are required", http.StatusBadRequest)
		return nil, false
	}
	sw, err := s.workspaceMgr.GetOrCreate(r.Context(), s.storage, userID, agentID, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil, false
	}
	return sw, true
}

// handleWorkspaceListDir lists a directory inside the session workspace.
// GET /workspace/list_dir?agent_id=&session_id=&path=
func (s *Server) handleWorkspaceListDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sw, ok := s.workspaceFromQuery(w, r)
	if !ok {
		return
	}
	dir, err := workspaceSafeJoin(sw.Dir(), r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	out := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		var size int64
		if fi, err := e.Info(); err == nil {
			size = fi.Size()
		}
		out = append(out, fileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleWorkspaceReadFile reads a file artifact from the session workspace.
// GET /workspace/read_file?agent_id=&session_id=&path=
func (s *Server) handleWorkspaceReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sw, ok := s.workspaceFromQuery(w, r)
	if !ok {
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	p, err := workspaceSafeJoin(sw.Dir(), rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	st, err := os.Stat(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if st.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}
	if st.Size() > maxReadFileBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	_, _ = w.Write(data)
}

// handleWorkspaceStatus reports the session working directory and git state.
// GET /workspace/status?agent_id=&session_id=
func (s *Server) handleWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sw, ok := s.workspaceFromQuery(w, r)
	if !ok {
		return
	}
	branch, changes, isRepo := gitStatus(r.Context(), sw.Dir())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(workspaceStatus{
		Dir:        sw.Dir(),
		IsGitRepo:  isRepo,
		GitBranch:  branch,
		GitChanges: changes,
	})
}

// gitStatus returns branch and porcelain changes for dir, degrading gracefully
// when git is missing or dir is not a repository.
func gitStatus(ctx context.Context, dir string) (branch string, changes []string, isRepo bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, false
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", nil, false
	}
	branch = strings.TrimSpace(string(out))
	if out, err = exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if line = strings.TrimRight(line, "\r"); line != "" {
				changes = append(changes, line)
			}
		}
	}
	return branch, changes, true
}
