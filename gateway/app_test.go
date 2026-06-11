package gateway

import (
	"testing"

	"github.com/linkerlin/agentscope.go/observability"
	"github.com/linkerlin/agentscope.go/service"
)

func TestNewApp_RegisterAppRoutes(t *testing.T) {
	storage := service.NewMemoryStorage()
	btm := NewBackgroundTaskManager(NewAgentRegistry(), nil).WithStorage(storage)
	srv := NewApp(AppConfig{
		Agent:             &mockAgent{name: "test"},
		Storage:           storage,
		BackgroundTaskMgr: btm,
		WorkspaceManager:  NewWorkspaceManager(t.TempDir(), ""),
	})
	srv.RegisterAppRoutes(nil)

	if srv.storage == nil || srv.backgroundTaskMgr == nil || srv.workspaceMgr == nil {
		t.Fatal("expected app dependencies wired")
	}
}

func TestNewApp_WithTracing(t *testing.T) {
	storage := service.NewMemoryStorage()
	adapter := &observability.TracingMiddlewareAdapter{Tracer: observability.NoopTracer, Name: "test"}
	srv := NewApp(AppConfig{
		Agent:   &mockAgent{name: "test"},
		Storage: storage,
	})
	srv.RegisterAppRoutes(nil)
	// 演示 Phase 5 tracing adapter 可用于 middleware
	_ = adapter
}

func TestNewApp_BootstrapWithTracingMiddleware(t *testing.T) {
	storage := service.NewMemoryStorage()
	tracingMW := &observability.TracingMiddlewareAdapter{
		Tracer: observability.NoopTracer,
		Name:   "bootstrap-test",
	}
	srv := NewApp(AppConfig{
		Agent:   &mockAgent{name: "test"},
		Storage: storage,
	})
	srv.RegisterAppRoutes(nil)
	// In real usage, pass tracingMW to react.Builder().Middlewares(tracingMW)
	_ = tracingMW
	if srv.storage == nil {
		t.Fatal("expected storage wired in bootstrap with tracing")
	}
}
