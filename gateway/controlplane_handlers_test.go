package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCPTestServer builds a Server with an in-memory control plane and no
// authenticator (so requireAuth is a no-op and routes are open for testing).
func newCPTestServer(t *testing.T) (*Server, *controlplane.Kernel) {
	t.Helper()
	k := controlplane.NewKernel(nil, nil, nil)
	srv := NewServer(nil).WithControlPlane(k)
	srv.RegisterControlPlaneRoutes()
	return srv, k
}

func doJSON(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if rr.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	}
	return out
}

func TestCPHTTP_CreateGoalAndShouldRun(t *testing.T) {
	srv, _ := newCPTestServer(t)

	// Create a goal.
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals",
		map[string]any{"objective": "ship feature", "scope": []string{"safe"}})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	g := decodeBody(t, rr)
	gid := g["id"].(string)
	assert.Equal(t, "ship feature", g["objective"])
	assert.Equal(t, string(controlplane.GoalActive), g["state"])

	// GET the goal back.
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	// should-run -> eligible.
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/should-run?agent=a1", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	dec := decodeBody(t, rr)
	assert.Equal(t, true, dec["should_run"])
	assert.Equal(t, string(controlplane.ComputeEligible), dec["state"])
}

func TestCPHTTP_GateBlocksThenResolve(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)

	// Open a gate.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/gates", map[string]any{
		"question": "approve?",
		"scope":    map[string]any{"kind": "production", "scope_key": "prod"},
	})
	require.Equal(t, http.StatusCreated, rr.Code)

	// should-run now blocked.
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/should-run?agent=a1", nil)
	dec := decodeBody(t, rr)
	assert.Equal(t, false, dec["should_run"])
	assert.Equal(t, string(controlplane.ComputeOperatorGate), dec["state"])
	gateID := dec["gate_id"].(string)

	// List pending gates shows it.
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/gates", nil)
	require.Contains(t, decodeBody(t, rr), "pending_gates")

	// Resolve -> eligible again.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/gates/"+gateID+"/resolve",
		map[string]any{"decision": "approve", "by": "alice"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/should-run?agent=a1", nil)
	dec = decodeBody(t, rr)
	assert.Equal(t, true, dec["should_run"])
}

func TestCPHTTP_WritebackAndSpend(t *testing.T) {
	srv, k := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)
	// Create a todo (and make it current).
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/todos",
		map[string]any{"description": "do work", "current_todo": true})
	require.Equal(t, http.StatusCreated, rr.Code)
	tid := decodeBody(t, rr)["id"].(string)

	// Writeback with evidence.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/writeback", map[string]any{
		"todo_id": tid, "turn_id": "turn-1",
		"outcome":  map[string]any{"status": "progress"},
		"evidence": []any{map[string]any{"id": "e1", "kind": "test_pass", "summary": "green"}},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	delivery := decodeBody(t, rr)["delivery_id"]
	assert.NotEmpty(t, delivery)

	// Spend without execute (dry-run) does not commit.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/spend",
		map[string]any{"turn_id": "turn-1", "execute": false})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, false, decodeBody(t, rr)["executed"])

	// Spend execute.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/spend",
		map[string]any{"turn_id": "turn-1", "execute": true})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, float64(1), decodeBody(t, rr)["spent_in_window"])

	// Spend again -> error (already spent).
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/spend",
		map[string]any{"turn_id": "turn-1", "execute": true})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "already spent")

	// Direct kernel assertion: ledger recorded it.
	led := k.Ledger()
	require.NotNil(t, led)
	evs, _, err := led.Read(context.Background(), gid, 0, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(evs), 2) // writeback + spend
}

func TestCPHTTP_LeaseAcquireRelease(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/todos", map[string]any{"description": "x"})
	tid := decodeBody(t, rr)["id"].(string)

	// Acquire.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/leases",
		map[string]any{"todo_id": tid, "owner": "wA", "ttl_seconds": 60})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Second owner -> conflict.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/leases",
		map[string]any{"todo_id": tid, "owner": "wB"})
	require.Equal(t, http.StatusConflict, rr.Code)

	// Release by owner.
	rr = doJSON(t, srv, "DELETE", "/api/v1/controlplane/goals/"+gid+"/leases/"+tid+"?owner=wA", nil)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestCPHTTP_ReviewPacket(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)

	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/review", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	pkt := decodeBody(t, rr)
	assert.Equal(t, gid, pkt["goal"].(map[string]any)["id"])
}

func TestCPHTTP_MissingGoal404(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "GET", "/api/v1/controlplane/goals/ghost/should-run?agent=a1", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCPHTTP_CapabilitiesCatalog(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "GET", "/api/v1/controlplane/capabilities", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	caps := decodeBody(t, rr)["capabilities"].([]any)
	require.NotEmpty(t, caps, "builtin capabilities present")
	// Find issue-fix and assert its lane + gated review/merge.
	var issueFix map[string]any
	for _, c := range caps {
		cm := c.(map[string]any)
		if cm["id"] == "issue-fix" {
			issueFix = cm
		}
	}
	require.NotNil(t, issueFix, "issue-fix flagship capability present")
	lane := issueFix["lane"].([]any)
	assert.Len(t, lane, 5)
}

func TestCPHTTP_IllegalGoalTransition(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)
	// completed -> active is illegal.
	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "completed"})
	require.Equal(t, http.StatusOK, rr.Code)
	rr = doJSON(t, srv, "PATCH", "/api/v1/controlplane/goals/"+gid, map[string]any{"state": "active"})
	require.Equal(t, http.StatusConflict, rr.Code)
}

func TestCPHTTP_KanbanAndSupersede(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/todos", map[string]any{"description": "v1", "current_todo": true})
	tid := decodeBody(t, rr)["id"].(string)

	// Supersede t1 -> successor.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/todos/"+tid+"/supersede",
		map[string]any{"description": "v2 better approach", "agent": "a1", "reason": "flaky"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	succ := decodeBody(t, rr)
	succID := succ["id"].(string)
	assert.Equal(t, tid, succ["supersedes"])

	// Kanban projection groups by column and records lineage.
	rr = doJSON(t, srv, "GET", "/api/v1/controlplane/goals/"+gid+"/kanban", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	board := decodeBody(t, rr)
	assert.Equal(t, gid, board["goal_id"])
	lineage := board["lineage"].([]any)
	require.Len(t, lineage, 1)
	edge := lineage[0].(map[string]any)
	assert.Equal(t, tid, edge["from"])
	assert.Equal(t, succID, edge["to"])

	// The deferred column carries the superseded card with lifecycle=superseded.
	cols := board["columns"].(map[string]any)
	deferred := cols["deferred"].([]any)
	require.Len(t, deferred, 1)
	card := deferred[0].(map[string]any)
	assert.Equal(t, "superseded", card["lifecycle"])
	assert.Equal(t, succID, card["superseded_by"])
}

// --- #5 round-3: HTTP hardening ---

func TestCPHTTP_CreateGoalIgnoresRequestedState(t *testing.T) {
	srv, _ := newCPTestServer(t)
	// Request a terminal state at creation -> must be forced to active.
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals",
		map[string]any{"objective": "g", "state": "completed"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	g := decodeBody(t, rr)
	assert.Equal(t, "active", g["state"], "create must force active; state moves only via PATCH")
}

func TestCPHTTP_LeaseRejectsPhantomTodo(t *testing.T) {
	srv, _ := newCPTestServer(t)
	rr := doJSON(t, srv, "POST", "/api/v1/controlplane/goals", map[string]any{"objective": "g"})
	gid := decodeBody(t, rr)["id"].(string)
	// No todo created -> leasing a phantom must 404.
	rr = doJSON(t, srv, "POST", "/api/v1/controlplane/goals/"+gid+"/leases",
		map[string]any{"todo_id": "ghost", "owner": "w"})
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}
