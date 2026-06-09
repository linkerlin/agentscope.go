package gateway

import (
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
)

// AppConfig wires the gateway server for multi-tenant AgentScope deployments.
// It mirrors PyV2 create_app() dependencies in a Go-idiomatic struct.
type AppConfig struct {
	Agent               agent.Agent
	Storage             service.Storage
	Authenticator       service.Authenticator
	JWTAuth             *service.JWTAuthenticator
	Cipher              *service.Cipher
	Registry            *AgentRegistry
	SessionManager      *SessionManager
	BackgroundTaskMgr   *BackgroundTaskManager
	WorkspaceManager    *WorkspaceManager
	ToolOffloadManager  *ToolOffloadManager
	ModelCardsDir       string
	EmbeddingModel      model.EmbeddingModel
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
	}
	if cfg.ToolOffloadManager != nil {
		srv.WithToolOffloadManager(cfg.ToolOffloadManager)
	}
	if cfg.WorkspaceManager != nil {
		srv.WithWorkspaceManager(cfg.WorkspaceManager)
	}
	if cfg.ModelCardsDir != "" {
		srv.WithModelCardsDir(cfg.ModelCardsDir)
	}
	if cfg.EmbeddingModel != nil {
		srv.WithEmbeddingModel(cfg.EmbeddingModel)
	}
	return srv
}

// RegisterAppRoutes registers all built-in HTTP routes (auth, CRUD, chat, schedule, workspace).
func (s *Server) RegisterAppRoutes(jwtAuth *service.JWTAuthenticator) {
	s.RegisterAuthRoutes(jwtAuth)
	s.RegisterServiceRoutes()
	s.RegisterWorkspaceRoutes()
	s.RegisterScheduleRoutes()
	s.RegisterBackgroundTaskRoutes()
	s.RegisterModelRoutes()
	s.RegisterEmbeddingRoutes()
	s.RegisterV2Routes()
}
