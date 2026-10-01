// gateway/schedule_compat.go keeps the root's public surface stable while
// the schedule cluster lives in gateway/scheduleapi (16.2, third
// registration-functionized cluster after kbapi/sessionapi):
//
//   - BackgroundTaskManager is an embedding wrapper over
//     scheduleapi.BackgroundTaskManager. Embedding promotes every method
//     (zero breakage vs the pre-extraction root type) and lets the root add
//     ToolOffload(), whose lazy singleton belongs to the gateway assembly
//     (the offload manager is root-owned).
//   - RegisterScheduleRoutes delegates to scheduleapi.Handlers with the
//     same URLs and auth wrapping.
package gateway

import (
	"github.com/linkerlin/agentscope.go/gateway/scheduleapi"
	"github.com/linkerlin/agentscope.go/service"
)

// BackgroundTaskManager wraps the scheduleapi engine; promoted methods
// keep the pre-16.2 surface intact.
type BackgroundTaskManager struct {
	*scheduleapi.BackgroundTaskManager
}

// NewBackgroundTaskManager creates a manager and starts the internal cron
// scheduler. Call Stop() on shutdown.
func NewBackgroundTaskManager(registry *AgentRegistry, sessions *SessionManager) *BackgroundTaskManager {
	return &BackgroundTaskManager{
		BackgroundTaskManager: scheduleapi.NewBackgroundTaskManager(registry, sessions),
	}
}

// WithStorage enables schedule persistence and session linkage (chainable
// on the wrapper so existing call sites keep working).
func (btm *BackgroundTaskManager) WithStorage(st service.Storage) *BackgroundTaskManager {
	btm.BackgroundTaskManager = btm.BackgroundTaskManager.WithStorage(st)
	return btm
}

// WithSessionRun routes cron-triggered turns through the coordinator entry
// (18.5; chainable on the wrapper).
func (btm *BackgroundTaskManager) WithSessionRun(fn sessionRunFunc) *BackgroundTaskManager {
	btm.BackgroundTaskManager = btm.BackgroundTaskManager.WithSessionRun(scheduleapi.RunFunc(fn))
	return btm
}

// toolOffload holds the lazy offload singleton (root-owned assembly; the
// schedule engine no longer carries it, 16.2).
var toolOffloadOnce struct {
	mgr *ToolOffloadManager
}

// ToolOffload returns the tool offload manager (lazy init, process-wide
// single instance — same semantics as the pre-16.2 per-BTM singleton for
// every deployment that creates one BTM per process).
func (btm *BackgroundTaskManager) ToolOffload() *ToolOffloadManager {
	if toolOffloadOnce.mgr == nil {
		toolOffloadOnce.mgr = NewToolOffloadManager()
	}
	return toolOffloadOnce.mgr
}

// scheduleHandlers lazily builds the scheduleapi handlers over the server's
// wiring. Wire the background task manager and storage BEFORE the first
// request — the same ordering requirement sessionapi has.
func (s *Server) scheduleHandlers() *scheduleapi.Handlers {
	s.scheduleAPIBuild.Do(func() {
		var btm *scheduleapi.BackgroundTaskManager
		if s.backgroundTaskMgr != nil {
			btm = s.backgroundTaskMgr.BackgroundTaskManager // typed-nil guard (16.2)
		}
		s.scheduleAPIHandlers = scheduleapi.NewHandlers(scheduleapi.Deps{
			BTM:          btm,
			Storage:      s.storage,
			MaxBodyBytes: maxBodyBytes,
		})
	})
	return s.scheduleAPIHandlers
}

// RegisterScheduleRoutes adds schedule CRUD endpoints aligned with PyV2
// /schedule. URLs, auth wrapping and behaviours are unchanged from the
// pre-extraction root routes (16.2).
func (s *Server) RegisterScheduleRoutes() {
	s.scheduleHandlers().Register(s.mux, s.requireAuth)
}
