package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/runcontext"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
)

const defaultToolOffloadTimeout = 15 * time.Second

// ToolOffloadManager tracks long-running tool executions offloaded from the
// synchronous ReAct loop and stores completed results for later injection.
type ToolOffloadManager struct {
	mu             sync.Mutex
	pendingResults map[string][]string // sessionID -> hint texts
	tasks          map[string]*offloadedToolTask
	timeout        time.Duration
}

type offloadedToolTask struct {
	ID        string
	SessionID string
	ToolName  string
	Started   time.Time
	Done      bool
}

// NewToolOffloadManager creates a manager with the default offload timeout.
func NewToolOffloadManager() *ToolOffloadManager {
	return &ToolOffloadManager{
		pendingResults: make(map[string][]string),
		tasks:          make(map[string]*offloadedToolTask),
		timeout:        defaultToolOffloadTimeout,
	}
}

// WithTimeout overrides the synchronous wait before offloading.
func (m *ToolOffloadManager) WithTimeout(d time.Duration) *ToolOffloadManager {
	if d > 0 {
		m.timeout = d
	}
	return m
}

// PushResult stores a completed background tool hint for a session.
func (m *ToolOffloadManager) PushResult(sessionID, hint string) {
	if sessionID == "" || hint == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingResults[sessionID] = append(m.pendingResults[sessionID], hint)
}

// PopResults returns and clears pending hints for a session.
func (m *ToolOffloadManager) PopResults(sessionID string) []string {
	if sessionID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.pendingResults[sessionID]
	delete(m.pendingResults, sessionID)
	return out
}

// ListTasks returns snapshots of offloaded tool tasks.
func (m *ToolOffloadManager) ListTasks() []*offloadedToolTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*offloadedToolTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		cpy := *t
		out = append(out, &cpy)
	}
	return out
}

// Cancel marks an offloaded task as cancelled (best-effort; underlying work may continue).
func (m *ToolOffloadManager) Cancel(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[taskID]
	if !ok || t.Done {
		return false
	}
	t.Done = true
	return true
}

func (m *ToolOffloadManager) registerTask(sessionID, toolName string) string {
	id := uuid.New().String()
	m.mu.Lock()
	m.tasks[id] = &offloadedToolTask{
		ID:        id,
		SessionID: sessionID,
		ToolName:  toolName,
		Started:   time.Now(),
	}
	m.mu.Unlock()
	return id
}

func (m *ToolOffloadManager) finishTask(taskID string) {
	m.mu.Lock()
	if t := m.tasks[taskID]; t != nil {
		t.Done = true
	}
	m.mu.Unlock()
}

// ToolOffloadMiddleware offloads tool calls that exceed TimeoutSecs.
type ToolOffloadMiddleware struct {
	manager   *ToolOffloadManager
	sessionID string
	timeout   time.Duration
}

// NewToolOffloadMiddleware creates middleware bound to a session.
// When sessionID is empty, the session is read from ContextWithSessionID at execution time.
func NewToolOffloadMiddleware(m *ToolOffloadManager, sessionID string) *ToolOffloadMiddleware {
	if m == nil {
		m = NewToolOffloadManager()
	}
	return &ToolOffloadMiddleware{
		manager:   m,
		sessionID: sessionID,
		timeout:   m.timeout,
	}
}

func (m *ToolOffloadMiddleware) resolveSessionID(ctx context.Context) string {
	if m.sessionID != "" {
		return m.sessionID
	}
	return runcontext.SessionID(ctx)
}

func (m *ToolOffloadMiddleware) Wrap(next toolkit.Handler) toolkit.Handler {
	return func(ctx context.Context, req *toolkit.Request) (*toolkit.Response, error) {
		if req.Stage != toolkit.StageExecuteTool {
			return next(ctx, req)
		}

		ch := make(chan execResult, 1)
		go func() {
			resp, err := next(ctx, req)
			ch <- execResult{resp: resp, err: err}
		}()

		waitCtx, cancel := context.WithTimeout(ctx, m.timeout)
		defer cancel()

		select {
		case res := <-ch:
			return res.resp, res.err
		case <-waitCtx.Done():
			sessID := m.resolveSessionID(ctx)
			taskID := m.manager.registerTask(sessID, req.ToolName)
			go m.collectBackgroundResult(sessID, taskID, req.ToolName, ch)
			placeholder := fmt.Sprintf(
				"<system-reminder>Tool '%s' is taking longer than expected and has been moved to the background (task_id=%s). "+
					"The result will be injected into the context automatically when finished.</system-reminder>",
				req.ToolName, taskID,
			)
			return &toolkit.Response{
				Single: tool.NewTextResponse(placeholder),
			}, nil
		}
	}
}

func (m *ToolOffloadMiddleware) collectBackgroundResult(sessionID, taskID, toolName string, ch <-chan execResult) {
	defer m.manager.finishTask(taskID)
	res := <-ch
	var hint string
	if res.err != nil {
		hint = fmt.Sprintf(
			"<system-notification>\nBackground task for tool '%s' failed.\nError: %v\n</system-notification>",
			toolName, res.err,
		)
	} else if res.resp != nil && res.resp.Single != nil {
		hint = fmt.Sprintf(
			"<system-notification>\nBackground task for tool '%s' has completed.\nResult:\n%s\n</system-notification>",
			toolName, res.resp.Single.GetTextContent(),
		)
	} else {
		hint = fmt.Sprintf(
			"<system-notification>\nBackground task for tool '%s' has completed with no result.\n</system-notification>",
			toolName,
		)
	}
	m.manager.PushResult(sessionID, hint)
}

type execResult struct {
	resp *toolkit.Response
	err  error
}
