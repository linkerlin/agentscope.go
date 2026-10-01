// Package channelapi is the 16.2 fifth registration-functionized cluster:
// the channel management + webhook HTTP face. Dependencies are narrowed to
// *channel.Registry, service.Storage, the sessionapi.SessionState resume
// lifecycle and a ResolveAgent closure so the cluster does not import the
// gateway root; the root keeps the Server assembly (WithChannelGateway /
// StartChannels / RegisterChannelRoutes) plus the ChannelRunner adapter
// (whose public mutable fields are root-manager glue, 16.2).
package channelapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/channel/dingtalk"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/service"
)

// Deps carries the cluster's narrowed dependencies.
type Deps struct {
	// Registry lists and resolves configured channels.
	Registry *channel.Registry
	// Storage resolves callback sessions (nil = every callback 404s, matching
	// the pre-16.2 nil-storage behaviour).
	Storage service.Storage
	// State yields the optional suspend/resume lifecycle per request
	// (sessionapi.SessionState, satisfied structurally by the root's
	// *SessionStateManager). Returning nil falls back to a direct in-memory
	// InjectEvent — a getter, not a snapshot, preserves the pre-16.2
	// behaviour where the root read its field at request time.
	State func() sessionapi.SessionState
	// ResolveAgent builds the agent for a HITL card callback's session
	// (root's Server.buildSessionAgentFromStorage or an override).
	ResolveAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error)
}

// Handlers serves the channel HTTP face.
type Handlers struct{ d Deps }

// NewHandlers builds the handler set.
func NewHandlers(d Deps) *Handlers { return &Handlers{d: d} }

// Register wires the three routes onto mux. wrap is applied to the
// management list route only (tenant auth); webhook and card callbacks are
// machine-to-platform traffic authenticated inside the handler (18.3).
func (h *Handlers) Register(mux *http.ServeMux, wrap func(http.HandlerFunc) http.HandlerFunc) {
	list := http.HandlerFunc(h.handleList)
	if wrap != nil {
		list = wrap(list)
	}
	mux.Handle("GET /api/v1/channels", list)
	mux.HandleFunc("POST /api/v1/channels/{id}/webhook", h.handleWebhookDelivery)
	// DingTalk AI-card callbacks (18.3): authenticated by the platform's
	// HMAC signature pair inside the handler, not by a tenant JWT — a
	// machine-to-platform callback cannot carry one. The HITL decision is
	// injected through the 23.2 idempotent resume state machine.
	mux.HandleFunc("POST /api/v1/channels/{id}/dingtalk/card-callback", h.handleDingtalkCardCallback)
}

type channelInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	var out []channelInfo
	for _, c := range h.d.Registry.List() {
		out = append(out, channelInfo{ID: c.ID(), Type: channelTypeOf(c)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func channelTypeOf(c channel.Channel) string {
	switch c.(type) {
	case *channel.WebhookChannel:
		return "webhook"
	default:
		// Self-describing adapters (18.3 dingtalk et al.) report their own
		// kind so this package need not import every adapter.
		if self, ok := c.(interface{ ChannelKind() string }); ok {
			return self.ChannelKind()
		}
		return fmt.Sprintf("%T", c)
	}
}

// handleWebhookDelivery is the inbound HTTP endpoint for HTTP-pull channels
// (webhook, 18.3 dingtalk outgoing callback / card callback). The channel is
// looked up by id and, when it exposes an HTTP handler, the request is
// delegated.
func (h *Handlers) handleWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c := h.d.Registry.Get(id)
	if c == nil {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	httpHandler, ok := c.(http.Handler)
	if !ok {
		http.Error(w, "channel has no HTTP receiver", http.StatusBadRequest)
		return
	}
	httpHandler.ServeHTTP(w, r)
}

// handleDingtalkCardCallback receives one AI-card callback for channel
// {id}: verify the robot signature, decode the HITL action, and inject the
// decision through the idempotent resume state machine. Responses:
//
//   - non-HITL action values → 200 ACK (custom buttons are legitimate and
//     simply not ours to process);
//   - HITL decision delivered → 200;
//   - duplicate callback (command already executing) → 200: idempotent, the
//     tool runs at most once (23.2);
//   - decision persisted but no live waiter here → 200 as well: the command
//     stays persisted and the 18.5 wakeup already notifies the worker tier;
//   - unknown session → 404 (no existence leak).
func (h *Handlers) handleDingtalkCardCallback(w http.ResponseWriter, r *http.Request) {
	chID := r.PathValue("id")
	raw := h.d.Registry.Get(chID)
	dc, ok := raw.(*dingtalk.Channel)
	if !ok {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	if !dc.VerifyRequest(r) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	cb, err := dingtalk.ParseCardCallback(r)
	if err != nil {
		http.Error(w, "bad callback: "+err.Error(), http.StatusBadRequest)
		return
	}
	action, err := dingtalk.ParseHitlAction(cb)
	if err != nil {
		if errors.Is(err, dingtalk.ErrNotHITLAction) {
			w.WriteHeader(http.StatusOK) // not ours: ACK and drop
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The callback session must exist (404 on unknown — no existence leak).
	ctx := r.Context()
	se, err := h.d.Storage.GetSession(ctx, action.SessionID)
	if err != nil || se == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	ag, err := h.d.ResolveAgent(ctx, se.AgentID, se.ID)
	if err != nil || ag == nil {
		http.Error(w, "agent unavailable", http.StatusServiceUnavailable)
		return
	}
	v2, ok := ag.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support resume", http.StatusNotImplemented)
		return
	}

	ev := event.NewUserConfirmResult(action.ReplyID, action.ConfirmID, action.Decisions)
	// 23.2 idempotent resume: duplicates refused, undeliverable commands
	// stay persisted for the holder replica (18.5 worker wakeup armed).
	if st := h.d.State(); st != nil {
		err = st.Resume(ctx, action.SessionID, v2, ev)
	} else {
		err = v2.InjectEvent(ctx, ev)
	}
	switch {
	case err == nil,
		errors.Is(err, sessionapi.ErrResumeAlreadyExecuting), // idempotent duplicate
		errors.Is(err, sessionapi.ErrResumeNotDelivered):     // persisted; wakeup notifies the holder
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, sessionapi.ErrStorageNotAvailable):
		http.Error(w, "session state not available", http.StatusServiceUnavailable)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeJSON mirrors the root's helper (kept local so the cluster stays
// root-free).
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
