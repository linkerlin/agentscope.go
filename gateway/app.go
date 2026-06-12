package gateway

import (
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
)

// AppConfig wires the gateway server for multi-tenant AgentScope deployments.
// It mirrors PyV2 create_app() dependencies in a Go-idiomatic struct.
//
// Recommended usage for new code:
//   srv := gateway.NewApp(gateway.AppConfig{
//       Storage: storage,
//       // ... other fields
//   })
//   srv.RegisterAppRoutes(jwt)
//   http.ListenAndServe(":8080", srv)
//
// The high-level NewApp + RegisterAppRoutes + existing managers give a close
// experience to Python's create_app + lifespan automatic wiring.
//
// See examples/full_service and examples/production for complete production bootstrap.

type AppConfig struct {
	Agent              agent.Agent
	Storage            service.Storage
	Authenticator      service.Authenticator
	JWTAuth            *service.JWTAuthenticator
	Cipher             *service.Cipher
	Registry           *AgentRegistry
	SessionManager     *SessionManager
	BackgroundTaskMgr  *BackgroundTaskManager
	WorkspaceManager   *WorkspaceManager
	ToolOffloadManager *ToolOffloadManager
	ModelCardsDir      string
	EmbeddingModel     model.EmbeddingModel
	AudioModel         model.AudioModel

	// --- Auto-assembly options (more "create_app" like experience) ---
	WorkspaceBaseDir      string               // if set and WorkspaceManager==nil, auto-create Local WorkspaceManager
	AutoStandardTools     bool                 // if true, auto-inject StandardTools (file+task+schedule+web+json) for session agents
	StandardToolsOptions  StandardToolsOptions // customization for auto tools (WorkspaceDir etc. will be overridden by base dir if set)
	DefaultPermissionMode permission.Mode      // default permission mode for auto-wired agents (defaults to Explore)
	AutoToolOffload       bool                 // ensure ToolOffload is wired into BTM and session deps

	// Embedding cache dir: if set and EmbeddingModel provided, auto-wrap with embedding.WithFileCache
	EmbeddingCacheDir string

	// Evolver integration (Phase 6 GEP alignment): set true or provide external MCP URL
	// to allow session agents to discover/call evolver tools (run/reflect/solidify, genes, capsules,
	// remember/recall, meetings, ATP tasks) via the existing MCP gateway.
	// See evolver/ package, examples/evolver, and docs for wiring + MockEvolver for tests.
	EvolverEnabled bool
}

// NewApp builds a configured gateway Server from AppConfig.
func NewApp(cfg AppConfig) *Server {
	srv := NewServer(cfg.Agent)
	if cfg.Storage != nil {
		srv.WithStorage(cfg.Storage)
	}
	if cfg.Authenticator != nil {
		srv.WithAuthenticator(cfg.Authenticator)
	}
	if cfg.Cipher != nil {
		srv.WithCipher(cfg.Cipher)
	}
	if cfg.Registry != nil {
		srv.WithRegistry(cfg.Registry)
	}
	if cfg.SessionManager != nil {
		srv.WithSessionManager(cfg.SessionManager)
	} else if cfg.Storage != nil {
		srv.WithSessionManager(NewSessionManager().WithStorage(cfg.Storage))
	}
	if cfg.BackgroundTaskMgr != nil {
		srv.WithBackgroundTaskManager(cfg.BackgroundTaskMgr)
	} else if cfg.Storage != nil {
		// Auto-create BackgroundTaskManager for schedule persistence + cron execution
		// (this gives schedule restore on startup, matching Python lifespan behavior).
		reg := cfg.Registry
		if reg == nil {
			reg = NewAgentRegistry()
		}
		sessionMgr := srv.sessionMgr
		if sessionMgr == nil {
			sessionMgr = NewSessionManager().WithStorage(cfg.Storage)
		}
		btm := NewBackgroundTaskManager(reg, sessionMgr).WithStorage(cfg.Storage)
		srv.WithBackgroundTaskManager(btm)
	}
	if cfg.ToolOffloadManager != nil {
		srv.WithToolOffloadManager(cfg.ToolOffloadManager)
	}
	if cfg.WorkspaceManager != nil {
		srv.WithWorkspaceManager(cfg.WorkspaceManager)
	} else if cfg.WorkspaceBaseDir != "" {
		// Auto-create workspace manager for session-scoped sandboxes (local by default).
		wm := NewWorkspaceManager(cfg.WorkspaceBaseDir, "")
		srv.WithWorkspaceManager(wm)
	}

	if cfg.ToolOffloadManager != nil {
		srv.WithToolOffloadManager(cfg.ToolOffloadManager)
	} else if cfg.AutoToolOffload || cfg.AutoStandardTools {
		// Ensure tool offload is available when auto tools or explicit flag is set.
		if btm := srv.backgroundTaskMgr; btm != nil {
			_ = btm.ToolOffload()
		}
	}

	if cfg.ModelCardsDir != "" {
		srv.WithModelCardsDir(cfg.ModelCardsDir)
	}
	if cfg.EmbeddingModel != nil {
		emb := cfg.EmbeddingModel
		if cfg.EmbeddingCacheDir != "" {
			emb = embedding.WithFileCache(emb, cfg.EmbeddingCacheDir)
		}
		srv.WithEmbeddingModel(emb)
	}
	if cfg.AudioModel != nil {
		srv.WithAudioModel(cfg.AudioModel)
	}

	// Auto-assemble default SessionAgentDeps for dynamic per-session agents
	// (this wires StandardTools, permission, tool-offload etc. automatically).
	if cfg.AutoStandardTools || cfg.DefaultPermissionMode != "" || cfg.AutoToolOffload {
		deps := SessionAgentDeps{
			PermissionMode: cfg.DefaultPermissionMode,
		}
		toolOpts := cfg.StandardToolsOptions
		if cfg.WorkspaceBaseDir != "" {
			toolOpts.WorkspaceDir = cfg.WorkspaceBaseDir
		}
		if cfg.AutoStandardTools {
			// Only pull non-conflicting "extra" tools via StandardTools (glob/grep + web/json).
			// Core file ops + task/schedule are provided by sessionWorkspaceTools using the deps fields below.
			// This avoids duplicate tool name registration errors.
			toolOpts.IncludeWeb = true
			toolOpts.IncludeJSON = true
			// Do not set IncludeTask/IncludeSchedule here; they are handled via deps below.
			allExtra := StandardTools(toolOpts)
			filtered := make([]tool.Tool, 0, len(allExtra))
			for _, t := range allExtra {
				n := t.Name()
				if n == "glob" || n == "grep" || n == "web_fetch" || n == "json_parse" || n == "json_query" {
					filtered = append(filtered, t)
				}
			}
			deps.ExtraTools = filtered

			// Auto-create simple in-memory TaskStore if not provided (when AutoStandardTools).
			if deps.TaskStore == nil {
				deps.TaskStore = state.NewTaskStore()
			}
			// Auto wire ScheduleMgr from BTM if available.
			if deps.ScheduleMgr == nil && srv.backgroundTaskMgr != nil {
				deps.ScheduleMgr = srv.ScheduleToolManager()
			}
		}
		if cfg.AutoToolOffload {
			if btm := srv.backgroundTaskMgr; btm != nil {
				deps.ToolOffload = btm.ToolOffload()
			}
		}
		srv.withDefaultSessionDeps(deps)
	}

	return srv
}

// Note: For even higher-level "create_app" style experience (auto default tools,
// schedule restore on startup, workspace manager lifecycle etc.), see Phase 2
// work and the updated production example. The current NewApp + RegisterAppRoutes
// + AppConfig already provides a very close equivalent to the Python pattern.

// RegisterAppRoutes registers all built-in HTTP routes (auth, CRUD, chat, schedule, workspace).
func (s *Server) RegisterAppRoutes(jwtAuth *service.JWTAuthenticator) {
	s.RegisterAuthRoutes(jwtAuth)
	s.RegisterServiceRoutes()
	s.RegisterWorkspaceRoutes()
	s.RegisterScheduleRoutes()
	s.RegisterBackgroundTaskRoutes()
	s.RegisterModelRoutes()
	s.RegisterEmbeddingRoutes()
	s.RegisterAudioRoutes()
	s.RegisterV2Routes()
}
