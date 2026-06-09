package gateway

import (
	"encoding/json"
	"net/http"
)

type listBackgroundTasksResponse struct {
	Tasks []*OffloadedToolTask `json:"tasks"`
	Total int                  `json:"total"`
}

func (s *Server) handleListBackgroundTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mgr := s.toolOffloadMgr()
	if mgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	sessionID := r.PathValue("session_id")
	tasks := mgr.ListTasksBySession(sessionID)
	if tasks == nil {
		tasks = []*OffloadedToolTask{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listBackgroundTasksResponse{Tasks: tasks, Total: len(tasks)})
}

func (s *Server) handleCancelBackgroundTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mgr := s.toolOffloadMgr()
	if mgr == nil {
		http.Error(w, "background task manager not configured", http.StatusServiceUnavailable)
		return
	}
	sessionID := r.PathValue("session_id")
	taskID := r.PathValue("task_id")
	tasks := mgr.ListTasksBySession(sessionID)
	found := false
	for _, t := range tasks {
		if t.ID == taskID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "background task not found", http.StatusNotFound)
		return
	}
	if !mgr.Cancel(taskID) {
		http.Error(w, "background task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterBackgroundTaskRoutes adds PyV2-aligned /background-tasks endpoints.
func (s *Server) RegisterBackgroundTaskRoutes() {
	s.mux.HandleFunc("/background-tasks/{session_id}", s.requireAuth(s.handleListBackgroundTasks))
	s.mux.HandleFunc("/background-tasks/{session_id}/{task_id}", s.requireAuth(s.handleCancelBackgroundTask))
}
