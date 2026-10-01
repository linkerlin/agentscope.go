// gateway/dingtalk_bridge.go realises the gateway half of the DingTalk
// closed loop (18.3):
//
//   - enterprise-app credentials come from the 18.4 binding store, never
//     from plaintext process configuration: ResolveDingtalkCredential reads
//     the referenced service.Credential, refuses anything not AUTHORIZED,
//     and decrypts through the configured cipher;
//   - card callbacks are signature-verified (the platform's HMAC pair —
//     that is the "认证" for a machine-to-platform callback) and the HITL
//     decision is injected through the 23.2 idempotent resume state
//     machine — duplicate callbacks and cross-replica consumers are safe
//     by construction.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel/dingtalk"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/service"
)

// dingtalkCredentialData is the secret-reference payload inside an 18.4
// binding credential (delivered at authorize time, encrypted at rest): the
// enterprise-app key/secret pair plus the optional outgoing-robot callback
// secret. References only — the plaintext exists transiently in memory.
type dingtalkCredentialData struct {
	AppKey      string `json:"app_key"`
	AppSecret   string `json:"app_secret"`
	RobotSecret string `json:"robot_secret,omitempty"`
}

// ResolveDingtalkCredential materialises a DingTalk channel config from an
// 18.4 credential. Only AUTHORIZED (or legacy blank-status) credentials
// resolve; PENDING bindings are refused — an unfinished binding must never
// yield working platform credentials. The returned secret lives only for the
// caller's constructor call.
func ResolveDingtalkCredential(ctx context.Context, storage service.Storage, cipher *service.Cipher, credentialID string) (appKey, appSecret, robotSecret string, err error) {
	cred, err := storage.GetCredential(ctx, credentialID)
	if err != nil {
		return "", "", "", fmt.Errorf("dingtalk: credential %s: %w", credentialID, err)
	}
	if st := cred.NormalizedStatus(); st != service.CredentialAuthorized {
		return "", "", "", fmt.Errorf("dingtalk: credential %s is %s, not usable (18.4 binding incomplete)", credentialID, st)
	}
	raw := cred.Encrypted
	if cipher != nil {
		plain, derr := cipher.Decrypt(raw)
		if derr != nil {
			return "", "", "", fmt.Errorf("dingtalk: decrypt credential %s: %w", credentialID, derr)
		}
		raw = plain
	}
	var data dingtalkCredentialData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", "", "", fmt.Errorf("dingtalk: credential %s payload: %w", credentialID, err)
	}
	if data.AppKey == "" || data.AppSecret == "" {
		return "", "", "", fmt.Errorf("dingtalk: credential %s missing app_key/app_secret", credentialID)
	}
	return data.AppKey, data.AppSecret, data.RobotSecret, nil
}

// NewDingtalkChannelFromCredential builds the channel from an 18.4 binding.
func NewDingtalkChannelFromCredential(ctx context.Context, id, credentialID string, storage service.Storage, cipher *service.Cipher) (*dingtalk.Channel, error) {
	appKey, appSecret, robotSecret, err := ResolveDingtalkCredential(ctx, storage, cipher, credentialID)
	if err != nil {
		return nil, err
	}
	ch := dingtalk.New(id, appKey, appSecret)
	if robotSecret != "" {
		ch = ch.WithRobotSecret(robotSecret)
	}
	return ch, nil
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
func (s *Server) handleDingtalkCardCallback(w http.ResponseWriter, r *http.Request) {
	chID := r.PathValue("id")
	raw := s.channelRegistry.Get(chID)
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
	se, err := s.storage.GetSession(ctx, action.SessionID)
	if err != nil || se == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	resolve := s.dingtalkAgentResolver
	if resolve == nil {
		resolve = s.buildSessionAgentFromStorage
	}
	ag, err := resolve(ctx, se.AgentID, se.ID)
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
	if s.sessionState != nil {
		err = s.sessionState.Resume(ctx, action.SessionID, v2, ev)
	} else {
		err = v2.InjectEvent(ctx, ev)
	}
	switch {
	case err == nil,
		errors.Is(err, sessionapi.ErrResumeAlreadyExecuting), // idempotent duplicate
		errors.Is(err, sessionapi.ErrResumeNotDelivered):     // persisted; wakeup notifies the holder
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, ErrStorageNotAvailable):
		http.Error(w, "session state not available", http.StatusServiceUnavailable)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
