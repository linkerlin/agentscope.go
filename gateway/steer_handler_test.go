package gateway

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// steerMockAgent is a V2Agent that parks until its run context is cancelled
// and records Steer/Interrupt calls.
type steerMockAgent struct {
	mu          sync.Mutex
	steered     []string
	interrupted bool
}

func (a *steerMockAgent) Name() string { return "steer-mock" }

func (a *steerMockAgent) Call(context.Context, *message.Msg) (*message.Msg, error) { return nil, nil }

func (a *steerMockAgent) CallStream(context.Context, *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func (a *steerMockAgent) Reply(context.Context, *message.Msg) (*message.Msg, error) { return nil, nil }

func (a *steerMockAgent) ReplyStream(ctx context.Context, _ *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	go func() {
		defer close(ch)
		<-ctx.Done()
	}()
	return ch, nil
}

func (a *steerMockAgent) LoadState(*agent.AgentState) error     { return nil }
func (a *steerMockAgent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (a *steerMockAgent) InjectEvent(context.Context, event.AgentEvent) error {
	return nil
}

func (a *steerMockAgent) Steer(text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steered = append(a.steered, text)
	return nil
}

func (a *steerMockAgent) Interrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.interrupted = true
}

func newSteerTestEnv(t *testing.T) (*Server, *SessionManager, *steerMockAgent) {
	t.Helper()
	ag := &steerMockAgent{}
	sm := NewSessionManager()
	srv := NewServer(ag).WithSessionManager(sm)
	srv.RegisterV2Routes()

	_, err := sm.Run(context.Background(), "sess1", ag,
		message.NewMsg().Role(message.RoleUser).TextContent("go").Build())
	require.NoError(t, err)
	require.Eventually(t, func() bool { return sm.IsActive("sess1") }, time.Second, 5*time.Millisecond)
	t.Cleanup(func() { sm.Terminate("sess1") })
	return srv, sm, ag
}

func TestV2Steer_InjectsIntoActiveRun(t *testing.T) {
	srv, _, ag := newSteerTestEnv(t)

	rr := doJSON(t, srv, "POST", "/v2/sessions/sess1/steer", map[string]any{"text": "turn left"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	ag.mu.Lock()
	defer ag.mu.Unlock()
	require.Len(t, ag.steered, 1)
	assert.Equal(t, "turn left", ag.steered[0])
}

func TestV2Steer_RejectsEmptyAndUnknown(t *testing.T) {
	srv, _, _ := newSteerTestEnv(t)

	rr := doJSON(t, srv, "POST", "/v2/sessions/sess1/steer", map[string]any{"text": ""})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSON(t, srv, "POST", "/v2/sessions/nope/steer", map[string]any{"text": "hi"})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestV2Interrupt_TerminatesActiveRun(t *testing.T) {
	srv, sm, ag := newSteerTestEnv(t)

	rr := doJSON(t, srv, "POST", "/v2/sessions/sess1/interrupt", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	ag.mu.Lock()
	assert.True(t, ag.interrupted)
	ag.mu.Unlock()
	require.Eventually(t, func() bool { return !sm.IsActive("sess1") }, time.Second, 5*time.Millisecond)

	// Second interrupt finds no active run.
	rr = doJSON(t, srv, "POST", "/v2/sessions/sess1/interrupt", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
