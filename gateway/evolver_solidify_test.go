package gateway

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/evolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidifyCapture embeds MockEvolver and captures the last SolidifyRequest.
type solidifyCapture struct {
	*evolver.MockEvolver
	mu  sync.Mutex
	req *evolver.SolidifyRequest
}

func (c *solidifyCapture) Solidify(ctx context.Context, req evolver.SolidifyRequest) (*evolver.SolidifyResult, error) {
	c.mu.Lock()
	r := req
	c.req = &r
	c.mu.Unlock()
	return c.MockEvolver.Solidify(ctx, req)
}

func (c *solidifyCapture) last() *evolver.SolidifyRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.req
}

func TestCPHTTP_AutoSolidifyOnGoalComplete(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	cap := &solidifyCapture{MockEvolver: evolver.NewMockEvolver()}
	srv := NewServer(nil).WithControlPlane(k).WithEvolver(cap).WithAutoSolidifyOnGoalComplete(true)
	srv.RegisterControlPlaneRoutes()

	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "close the loop"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	gid := decodeBody(t, rr)["id"].(string)

	// Transition to completed triggers the async solidify.
	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "completed"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	require.Eventually(t, func() bool { return cap.last() != nil }, 2*time.Second, 10*time.Millisecond)
	req := cap.last()
	assert.Equal(t, "controlplane", req.DecisionSource)
	assert.Equal(t, gid, req.PrimaryCause)
	assert.Equal(t, "close the loop", req.Intent)
	assert.Contains(t, req.Signals, "goal_completed")
}

func TestCPHTTP_NoSolidifyWhenDisabled(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	cap := &solidifyCapture{MockEvolver: evolver.NewMockEvolver()}
	srv := NewServer(nil).WithControlPlane(k).WithEvolver(cap) // autoSolidify stays off
	srv.RegisterControlPlaneRoutes()

	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	require.Equal(t, http.StatusCreated, rr.Code)
	gid := decodeBody(t, rr)["id"].(string)

	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "completed"})
	require.Equal(t, http.StatusOK, rr.Code)

	time.Sleep(50 * time.Millisecond)
	assert.Nil(t, cap.last())
}

func TestCPHTTP_NoDoubleSolidifyOnRepeatPatch(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	cap := &solidifyCapture{MockEvolver: evolver.NewMockEvolver()}
	srv := NewServer(nil).WithControlPlane(k).WithEvolver(cap).WithAutoSolidifyOnGoalComplete(true)
	srv.RegisterControlPlaneRoutes()

	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "once"})
	gid := decodeBody(t, rr)["id"].(string)

	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "completed"})
	require.Equal(t, http.StatusOK, rr.Code)
	require.Eventually(t, func() bool { return cap.last() != nil }, 2*time.Second, 10*time.Millisecond)

	// Patch again with the same (already completed) state: no new transition,
	// no second solidify.
	cap.mu.Lock()
	cap.req = nil
	cap.mu.Unlock()
	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "completed"})
	require.Equal(t, http.StatusOK, rr.Code)
	time.Sleep(50 * time.Millisecond)
	assert.Nil(t, cap.last())
}
