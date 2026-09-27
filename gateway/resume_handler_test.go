package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resumeRecordingAgent is a V2 mock that records InjectEvent calls so tests
// can assert the in-memory resume path actually fired.
type resumeRecordingAgent struct {
	mockV2Agent
	injected []event.AgentEvent
}

func (m *resumeRecordingAgent) InjectEvent(_ context.Context, ev event.AgentEvent) error {
	m.injected = append(m.injected, ev)
	return nil
}

var (
	_ agent.V2Agent = (*resumeRecordingAgent)(nil)
	_               = message.NewMsg
)

func postResume(t *testing.T, srv *Server, userID, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"reply_id":   "r1",
		"confirm_id": "c1",
		"decisions":  []map[string]string{{"tool_call_id": "tc1", "decision": "allow"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v2/resume", bytes.NewReader(body))
	if userID != "" {
		req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, userID))
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// TestV2Resume_InMemoryInjectWithoutStorage covers the nil-storage branch of
// the HTTP resume path (mirrors the WS branch TestGateway_ChatWSV2_SuspendResume
// guards): no snapshot persistence, resume goes straight to InjectEvent.
func TestV2Resume_InMemoryInjectWithoutStorage(t *testing.T) {
	mock := &resumeRecordingAgent{}
	srv := NewServer(mock)
	srv.RegisterV2Routes()

	w := postResume(t, srv, "", "sess-r1")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "resumed", resp["status"])
	require.Len(t, mock.injected, 1, "resume must inject the confirm result in memory")
	confirm, ok := mock.injected[0].(*event.UserConfirmResultEvent)
	require.True(t, ok, "injected event must be a UserConfirmResultEvent")
	assert.Equal(t, "c1", confirm.ConfirmID)
}

// TestV2Resume_TenantIsolation covers the checkSessionAccess branch the
// in-memory tests cannot see: with storage, another user's session answers
// 404 (existence not leaked) while the owner resumes normally.
func TestV2Resume_TenantIsolation(t *testing.T) {
	st := service.NewMemoryStorage()
	require.NoError(t, st.SaveSession(context.Background(), &service.Session{
		ID: "s-own", UserID: "u1", AgentID: "a1",
	}))

	mock := &resumeRecordingAgent{}
	srv := NewServer(mock)
	srv.WithStorage(st)
	srv.RegisterV2Routes()

	// Another user's session: 404, no injection.
	w := postResume(t, srv, "u2", "s-own")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Empty(t, mock.injected, "denied resume must not reach the agent")

	// The owner resumes through the storage-backed path.
	w = postResume(t, srv, "u1", "s-own")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, mock.injected, 1)
}
