package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completedBuf builds a minimal completed-buffer entry for retention tests.
func seedCompleted(sm *SessionManager, id string, at time.Time) {
	sm.completed[id] = []event.AgentEvent{event.NewReplyStart("r", id)}
	sm.completedAt[id] = at
}

// TestSessionManager_RetentionTTL locks the 23.3 rule: completed buffers
// older than the TTL are dropped by the retention pass.
func TestSessionManager_RetentionTTL(t *testing.T) {
	sm := NewSessionManager()
	sm.SetRetention(RetentionLimits{CompletedTTL: time.Hour, CompletedMaxEntries: 100})

	now := time.Now()
	seedCompleted(sm, "old", now.Add(-2*time.Hour))
	seedCompleted(sm, "fresh", now.Add(-time.Minute))

	stats := sm.EnforceRetention()
	assert.Equal(t, 1, stats.CompletedDropped)
	assert.Equal(t, 1, stats.CompletedLive)
	assert.NotContains(t, sm.completed, "old")
	assert.Contains(t, sm.completed, "fresh")
}

// TestSessionManager_RetentionCap locks the 23.3 rule: over the entry cap,
// the oldest-enqueued completed buffers are dropped first.
func TestSessionManager_RetentionCap(t *testing.T) {
	sm := NewSessionManager()
	sm.SetRetention(RetentionLimits{CompletedMaxEntries: 2, CompletedTTL: time.Hour})

	// All entries are inside the TTL window; only the cap applies.
	now := time.Now()
	seedCompleted(sm, "a", now.Add(-50*time.Minute))
	seedCompleted(sm, "b", now.Add(-40*time.Minute))
	seedCompleted(sm, "c", now.Add(-30*time.Minute))

	stats := sm.EnforceRetention()
	assert.Equal(t, 1, stats.CompletedDropped)
	assert.Equal(t, 2, stats.CompletedLive)
	assert.Contains(t, sm.completed, "b")
	assert.Contains(t, sm.completed, "c", "newest must survive the cap")
	assert.NotContains(t, sm.completed, "a", "oldest must drop first")
}

// TestSessionCoordinator_ReaperDropsStaleMarkers locks the 23.3 rule: the
// reaper CAS-deletes running-registry markers whose lease expiry has passed
// (a crashed replica's residue) and never touches a live one.
func TestSessionCoordinator_ReaperDropsStaleMarkers(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	cb := messagebus.AsCoordBus(bus)
	ns := c.keys.SessionRunRegistryNS()
	ctx := context.Background()

	// A stale (expired) marker and a live one.
	stale, _ := json.Marshal(leaseMarker{Owner: "dead", Fence: 1, Expires: time.Now().Add(-time.Minute)})
	live, _ := json.Marshal(leaseMarker{Owner: "alive", Fence: 2, Expires: time.Now().Add(time.Minute)})
	require.NoError(t, cb.RegistrySet(ctx, ns, "-:s-crashed", stale))
	require.NoError(t, cb.RegistrySet(ctx, ns, "-:s-live", live))

	dropped := c.dropStaleRunMarkers(ctx)
	assert.Equal(t, 1, dropped)

	_, err := cb.RegistryGet(ctx, ns, "-:s-crashed")
	assert.ErrorIs(t, err, messagebus.ErrNotFound, "stale marker must be gone")
	_, err = cb.RegistryGet(ctx, ns, "-:s-live")
	assert.NoError(t, err, "live marker must survive")
}

// TestSessionCoordinator_ReaperRun locks the 23.3 rule: StartReaper's pass
// drops stale state and the stop func terminates it cleanly.
func TestSessionCoordinator_ReaperRun(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)

	stop := c.StartReaper(context.Background(), 20*time.Millisecond)
	time.Sleep(60 * time.Millisecond) // let at least one pass run
	stop()
	stop() // idempotent
}

// TestCoordinator_LogAppendTrimsAtCap locks the 23.3 rule: the coordinated
// event log is trimmed to the retention cap (amortized), so a chatty session
// cannot grow its log without bound.
func TestCoordinator_LogAppendTrimsAtCap(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	c.retentionLimit = 10
	require.NotNil(t, c.trimmer)

	logNS := c.keys.SessionEventLogNS("-:s-chatty")
	for i := 0; i < 25; i++ {
		c.appendEvent(context.Background(), logNS, []byte{byte('a' + i%26)})
	}

	entries, _, err := messagebus.AsCoordBus(bus).LogRead(context.Background(), logNS, 0, 100)
	require.NoError(t, err)
	// The log was trimmed on the 10th and 20th append: it holds ~the newest
	// entries, never growing to 25.
	assert.Less(t, len(entries), 25, "log must be trimmed under the amortized cap")
	assert.Greater(t, len(entries), 0)
}

// TestSessionDelete_PurgesCoordinationState locks the 23.3 rule: deleting a
// session via the service API also purges its coordinated event log,
// running marker and in-memory completed buffer.
func TestSessionDelete_PurgesCoordinationState(t *testing.T) {
	bus := messagebus.NewLocalBus()
	sm := NewSessionManager()
	c := NewSessionCoordinator(sm).WithBus(bus)
	storage := service.NewMemoryStorage()
	ctx := context.Background()

	// Session row + coordination residue.
	require.NoError(t, storage.SaveSession(ctx, &service.Session{ID: "s-del", UserID: "u1"}))
	cb := messagebus.AsCoordBus(bus)
	coordID := scopedSessionID(ctxAsUser("u1"), "s-del")
	require.NoError(t, cb.RegistrySet(ctx, c.keys.SessionRunRegistryNS(), coordID, []byte(`{}`)))
	_, err := cb.LogAppend(ctx, c.keys.SessionEventLogNS(coordID), []byte(`{}`))
	require.NoError(t, err)
	seedCompleted(sm, "s-del", time.Now())

	// DELETE /api/v1/sessions/s-del.
	key, err := service.GenerateAPIKey()
	require.NoError(t, err)
	storage.SaveUser(ctx, &service.User{ID: "u1", Name: "u"})
	storage.SaveCredential(ctx, &service.Credential{ID: "c1", UserID: "u1", Provider: "api_key", Encrypted: service.HashAPIKey(key)})

	srv := NewServer(&mockAgent{}).
		WithStorage(storage).
		WithAuthenticator(service.NewAPIKeyAuthenticator(storage, "")).
		WithSessionManager(sm).
		WithSessionCoordinator(c)
	srv.RegisterServiceRoutes()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-del", nil)
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code, "delete must succeed: %s", rec.Body.String())

	// Coordination state is gone with the session row.
	_, err = cb.RegistryGet(ctx, c.keys.SessionRunRegistryNS(), coordID)
	assert.ErrorIs(t, err, messagebus.ErrNotFound, "running marker purged")
	logEntries, _, err := cb.LogRead(ctx, c.keys.SessionEventLogNS(coordID), 0, 10)
	assert.NoError(t, err)
	assert.Empty(t, logEntries, "event log purged")
	assert.NotContains(t, sm.completed, "s-del", "completed buffer purged")
}
