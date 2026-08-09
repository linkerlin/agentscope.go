package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/service"
)

// WithControlPlane attaches a control-plane Kernel. When set, the control-plane
// HTTP routes (goals/todos/gates/should-run/writeback/spend/leases/review) are
// enabled via RegisterControlPlaneRoutes. nil = disabled (default), and the
// gateway behaves exactly as before — the control plane is opt-in.
func (s *Server) WithControlPlane(k *controlplane.Kernel) *Server {
	s.controlPlane = k
	return s
}

// RegisterControlPlaneRoutes registers the long-running-agent governance API.
// No-op if no Kernel is attached (control plane disabled).
func (s *Server) RegisterControlPlaneRoutes() {
	k := s.controlPlane
	if k == nil {
		return
	}
	mux := s.mux
	mux.HandleFunc("GET /api/v1/controlplane/goals", s.requireAuth(s.handleListCPGoals))
	mux.HandleFunc("GET /api/v1/controlplane/capabilities", s.requireAuth(s.handleListCPCapabilities))
	mux.HandleFunc("POST /api/v1/controlplane/goals", s.requireAuth(s.handleCreateCPGoal))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}", s.requireAuth(s.handleGetCPGoal))
	mux.HandleFunc("PATCH /api/v1/controlplane/goals/{id}", s.requireAuth(s.handleUpdateCPGoal))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}/todos", s.requireAuth(s.handleListCPTodos))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/todos", s.requireAuth(s.handleCreateCPTodo))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}/gates", s.requireAuth(s.handleListCPGates))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/gates", s.requireAuth(s.handleOpenCPGate))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/gates/{gid}/resolve", s.requireAuth(s.handleResolveCPGate))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}/should-run", s.requireAuth(s.handleCPShouldRun))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/authorize", s.requireAuth(s.handleCPAuthorize))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/writeback", s.requireAuth(s.handleCPWriteback))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/spend", s.requireAuth(s.handleCPSpend))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/leases", s.requireAuth(s.handleCPAcquireLease))
	mux.HandleFunc("DELETE /api/v1/controlplane/goals/{id}/leases/{tid}", s.requireAuth(s.handleCPReleaseLease))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}/review", s.requireAuth(s.handleCPReview))
	mux.HandleFunc("GET /api/v1/controlplane/goals/{id}/kanban", s.requireAuth(s.handleCPKanban))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/todos/{tid}/supersede", s.requireAuth(s.handleCPSupersedeTodo))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/rewards", s.requireAuth(s.handleCPRecordReward))
	mux.HandleFunc("POST /api/v1/controlplane/goals/{id}/rewards/{rid}/revoke", s.requireAuth(s.handleCPRevokeReward))
	mux.HandleFunc("POST /api/v1/controlplane/maintenance", s.requireAuth(s.handleCPMaintenance))
}

func (s *Server) cp() (*controlplane.Kernel, bool) {
	if k := s.controlPlane; k != nil {
		return k, true
	}
	return nil, false
}

// cpGoalOrFail writes a 404 and returns false when the goal is absent.
func cpGoalOrFail(w http.ResponseWriter, r *http.Request, k *controlplane.Kernel, id string) (*controlplane.Goal, bool) {
	g, err := k.GoalStore().Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "goal not found"})
		return nil, false
	}
	return g, true
}

// cpAuthGoal loads a goal and enforces tenant ownership (#5): an authenticated
// user may only touch goals they own. When no authenticator is configured
// (UserIDFromContext returns ""), the check is skipped (test/dev mode).
func cpAuthGoal(w http.ResponseWriter, r *http.Request, k *controlplane.Kernel, id string) (*controlplane.Goal, bool) {
	g, ok := cpGoalOrFail(w, r, k, id)
	if !ok {
		return nil, false
	}
	uid := service.UserIDFromContext(r.Context())
	if uid != "" && g.OwnerUserID != uid {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden: goal owned by another tenant"})
		return nil, false
	}
	return g, true
}

// --- Goals ---

func (s *Server) handleListCPGoals(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gs, err := k.GoalStore().List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	// Tenant filter (#5): an authenticated user sees only their own goals.
	uid := service.UserIDFromContext(r.Context())
	if uid != "" {
		filtered := gs[:0]
		for _, g := range gs {
			if g.OwnerUserID == uid {
				filtered = append(filtered, g)
			}
		}
		gs = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"goals": gs})
}

// handleListCPCapabilities returns the capability catalog (built-in + extension
// lanes like issue-fix), so an operator can see what the control plane can do
// and which lanes are gated.
func (s *Server) handleListCPCapabilities(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"capabilities": k.CapabilityRegistry().List()})
}

type createCPGoalReq struct {
	Objective string                 `json:"objective"`
	Scope     []string               `json:"scope,omitempty"`
	State     controlplane.GoalState `json:"state,omitempty"`
	Quota     *controlplane.Quota    `json:"quota,omitempty"`
}

func (s *Server) handleCreateCPGoal(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	var req createCPGoalReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Objective == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "objective required"})
		return
	}
	g := &controlplane.Goal{
		ID: uuid.NewString(), Objective: req.Objective, Scope: req.Scope,
		OwnerUserID: service.UserIDFromContext(r.Context()),
		State:       controlplane.GoalActive, // #5: goals always start active; state moves only via PATCH (legal transitions)
		Quota:       controlplane.DefaultQuota(),
	}
	if req.Quota != nil {
		g.Quota = *req.Quota
	}
	if err := k.GoalStore().Upsert(r.Context(), g); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleGetCPGoal(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	g, ok := cpAuthGoal(w, r, k, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, g)
}

type updateCPGoalReq struct {
	State         *controlplane.GoalState `json:"state,omitempty"`
	CurrentTodoID *string                 `json:"current_todo_id,omitempty"`
	Quota         *controlplane.Quota     `json:"quota,omitempty"`
}

func (s *Server) handleUpdateCPGoal(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	g, ok := cpAuthGoal(w, r, k, r.PathValue("id"))
	if !ok {
		return
	}
	var req updateCPGoalReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.State != nil && *req.State != g.State {
		if !controlplane.LegalGoalTransition(g.State, *req.State) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "illegal goal transition", "from": g.State, "to": *req.State})
			return
		}
		g.State = *req.State
	}
	if req.CurrentTodoID != nil {
		g.CurrentTodoID = *req.CurrentTodoID
	}
	if req.Quota != nil {
		g.Quota = *req.Quota
	}
	if err := k.GoalStore().Upsert(r.Context(), g); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// --- Todos ---

func (s *Server) handleListCPTodos(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	if _, ok := cpAuthGoal(w, r, k, r.PathValue("id")); !ok {
		return
	}
	ts, err := k.TodoStore().List(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	// Public/private boundary (#2 round-6): redact evidence source refs here
	// too — ReviewPacket/Kanban already redact, so the raw /todos endpoint was
	// leaking what every other projection scrubbed.
	for _, t := range ts {
		t.Evidence = controlplane.RedactEvidenceSlice(t.Evidence)
	}
	writeJSON(w, http.StatusOK, map[string]any{"todos": ts})
}

type createCPTodoReq struct {
	Description string                 `json:"description"`
	TaskClass   controlplane.TaskClass `json:"task_class"`
	CurrentTodo bool                   `json:"current_todo,omitempty"`
}

func (s *Server) handleCreateCPTodo(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	g, ok := cpAuthGoal(w, r, k, gid)
	if !ok {
		return
	}
	var req createCPTodoReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	tc := req.TaskClass
	if tc == "" {
		tc = controlplane.TaskAdvancement
	}
	t := &controlplane.Todo{
		ID: uuid.NewString(), GoalID: gid, OwnerUserID: g.OwnerUserID, Description: req.Description,
		TaskClass: tc, State: controlplane.TodoOpen,
	}
	if err := k.TodoStore().Upsert(r.Context(), t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.CurrentTodo {
		g, _ := k.GoalStore().Get(r.Context(), gid)
		if g != nil {
			g.CurrentTodoID = t.ID
			_ = k.GoalStore().Upsert(r.Context(), g)
		}
	}
	writeJSON(w, http.StatusCreated, t)
}

// --- Gates ---

func (s *Server) handleListCPGates(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	if _, ok := cpAuthGoal(w, r, k, r.PathValue("id")); !ok {
		return
	}
	gates, err := k.GateStore().ListUnresolved(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending_gates": gates})
}

func (s *Server) handleOpenCPGate(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var g controlplane.UserGate
	if err := decodeJSON(r, &g); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	g.GoalID = gid
	if g.GateID == "" {
		g.GateID = uuid.NewString()
	}
	if err := k.OpenGate(r.Context(), g); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

type resolveCPGateReq struct {
	Decision controlplane.DecisionOutcome `json:"decision"`
	By       string                       `json:"by,omitempty"`
	Note     string                       `json:"note,omitempty"`
}

func (s *Server) handleResolveCPGate(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid, gidOk := r.PathValue("id"), r.PathValue("gid")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var req resolveCPGateReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	// The resolver identity is the authenticated user (#5), not a body field —
	// a request cannot forge who answered the gate.
	uid := service.UserIDFromContext(r.Context())
	if uid != "" {
		req.By = uid
	}
	outcome := controlplane.GateOutcome{Decision: req.Decision, By: req.By, Note: req.Note}
	if err := k.ResolveGate(r.Context(), gid, gidOk, outcome); err != nil {
		if errors.Is(err, controlplane.ErrUnauthorizedResolver) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"gate_id": gidOk, "resolved": true})
}

// --- Operators ---

func (s *Server) handleCPShouldRun(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	id := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, id); !ok {
		return
	}
	agentID := r.URL.Query().Get("agent")
	dec, err := k.ShouldRun(r.Context(), id, agentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, dec)
}

// handleCPAuthorize is the ticket-minting variant of should-run (#2 round-4).
// A runtime that intends to spend against a turn calls this with ?turn=... to
// obtain a Decision.TurnToken, then passes that token to POST .../spend. This
// closes the HTTP enforced-flow loop: without it, enforceTicket made /spend
// require a token that no HTTP endpoint could produce.
func (s *Server) handleCPAuthorize(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	id := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, id); !ok {
		return
	}
	turnID := r.URL.Query().Get("turn")
	if turnID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "turn query param required"})
		return
	}
	agentID := r.URL.Query().Get("agent")
	dec, err := k.ShouldRunTurn(r.Context(), id, agentID, turnID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, dec)
}

func (s *Server) handleCPWriteback(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var wb controlplane.ValidatedWriteback
	if err := decodeJSON(r, &wb); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	wb.GoalID = gid
	// The caller identity is the authenticated user, not a body field (#1 round-4):
	// this is what the claim-ownership check validates against, so it must not be
	// forgeable. (Mirror of handleResolveCPGate setting By = UserIDFromContext.)
	if uid := service.UserIDFromContext(r.Context()); uid != "" {
		wb.AgentID = uid
	}
	deliveryID, err := k.Writeback(r.Context(), wb)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"delivery_id": deliveryID})
}

type cpSpendReq struct {
	TurnID    string `json:"turn_id"`
	TurnToken string `json:"turn_token,omitempty"` // from a prior /should-run response (#3)
	Execute   bool   `json:"execute,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (s *Server) handleCPSpend(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var req cpSpendReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	total, err := k.SpendSlot(r.Context(), gid, req.TurnID, controlplane.SpendOpts{
		Execute: req.Execute, Reason: req.Reason, Token: req.TurnToken,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spent_in_window": total, "executed": req.Execute})
}

type cpLeaseReq struct {
	TodoID string `json:"todo_id"`
	Owner  string `json:"owner"`
	TTLSec int    `json:"ttl_seconds,omitempty"`
}

func (s *Server) handleCPAcquireLease(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var req cpLeaseReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	// #5: refuse to lease a phantom or terminal todo — the lease must target a
	// real, actionable todo.
	todo, err := k.TodoStore().Get(r.Context(), gid, req.TodoID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "todo not found"})
		return
	}
	if todo.State.IsTerminal() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "todo is terminal, cannot lease"})
		return
	}
	var ttl time.Duration
	if req.TTLSec > 0 {
		ttl = time.Duration(req.TTLSec) * time.Second
	}
	l, err := k.AcquireLease(r.Context(), gid, req.TodoID, req.Owner, ttl)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleCPReleaseLease(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid, tid := r.PathValue("id"), r.PathValue("tid")
	owner := r.URL.Query().Get("owner")
	if err := k.ReleaseLease(r.Context(), gid, tid, owner); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCPReview(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	pkt, err := k.ReviewPacket(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pkt)
}

// --- Kanban projection + row lineage ---
func (s *Server) handleCPKanban(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	board, err := k.Kanban(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, board)
}

type cpSupersedeReq struct {
	Description string `json:"description"`
	Agent       string `json:"agent,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func (s *Server) handleCPSupersedeTodo(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid, tid := r.PathValue("id"), r.PathValue("tid")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var req cpSupersedeReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "description required"})
		return
	}
	succ, err := k.SupersedeTodo(r.Context(), gid, tid, req.Agent, req.Description, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, succ)
}

// handleCPRecordReward records a classified reward policy (e.g. a hard_policy
// veto or a soft_preference advisory) on the goal. This is the operator entry
// point for the reward-memory feature — previously only the Go API could add
// policies, and nothing could ever revoke them.
func (s *Server) handleCPRecordReward(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid := r.PathValue("id")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	var rec controlplane.RewardRecord
	if err := decodeJSON(r, &rec); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if rec.Class == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "class required (e.g. hard_policy)"})
		return
	}
	rec.Scope.Granularity = controlplane.GranularityGoal
	rec.Scope.ScopeKey = gid
	if err := k.RecordReward(r.Context(), gid, rec); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	// Return the record with its assigned ID so the operator can revoke it.
	recs, _ := k.RewardStore().List(r.Context(), gid)
	writeJSON(w, http.StatusCreated, map[string]any{"recorded": true, "active_policies": recs})
}

// handleCPRevokeReward deactivates a policy by record ID — the undo for a
// misconfigured hard_policy veto.
func (s *Server) handleCPRevokeReward(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	gid, rid := r.PathValue("id"), r.PathValue("rid")
	if _, ok := cpAuthGoal(w, r, k, gid); !ok {
		return
	}
	if err := k.RevokeReward(r.Context(), gid, rid); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": true, "record_id": rid})
}

// handleCPMaintenance triggers storage housekeeping (#2 round-5): reaps
// consumed tickets, spent deliveries, and inactive rewards older than
// older_days, and compacts each goal's ledger to its last keep_last_n events.
// Before this endpoint existed, Reap/Compact had no caller — growth was
// bounded in capability but never in operation. Wrap in an admin-role check
// for stricter deployments.
func (s *Server) handleCPMaintenance(w http.ResponseWriter, r *http.Request) {
	k, ok := s.cp()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errCPDisabled())
		return
	}
	var req struct {
		OlderDays int `json:"older_days,omitempty"`
		KeepLastN int `json:"keep_last_n,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	olderThan := time.Duration(req.OlderDays) * 24 * time.Hour
	if olderThan <= 0 {
		olderThan = 24 * time.Hour
	}
	keepLastN := req.KeepLastN
	if keepLastN <= 0 {
		keepLastN = 200
	}
	if err := k.ReapAll(r.Context(), olderThan, keepLastN); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reaped": true, "older_than": olderThan.String(), "keep_last_n": keepLastN})
}

// decodeJSON is the gateway's shared JSON request decoder. It caps the request
// body at 1 MiB (#5 round-3) to prevent unbounded-memory DoS on the control
// plane endpoints.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

// errCPDisabled is the standard response when the control plane is not wired.
func errCPDisabled() map[string]any {
	return map[string]any{"error": "control plane not enabled"}
}
