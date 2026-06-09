package gateway

import (
	"testing"

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
