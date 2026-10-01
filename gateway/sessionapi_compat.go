package gateway

import (
	"context"
	"net/http"

	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/messagebus"
)

// The session HTTP face (streamable HTTP, WebSocket, steer/interrupt,
// resume, status) lives in gateway/sessionapi — the second
// registration-functionized cluster after gateway/kbapi (16.2). This file
// keeps the root's public surface stable: the Server lazily builds one
// Handlers instance over its wiring, mounts routes through it, and
// re-exports the AGUI types tests and clients reference.

// sessionAPIOnce/BroadcastToRoom/SessionCount note: handlers are built on
// first use (first request or first V2 route registration). Wire storage,
// session manager and coordinator BEFORE serving traffic — the same ordering
// requirement the authenticator already has.
func (s *Server) sessionAPI() *sessionapi.Handlers {
	s.sessionAPIBuild.Do(func() {
		var state sessionapi.SessionState
		if s.sessionState != nil {
			state = s.sessionState
		}
		var coord sessionapi.Coordinator
		if s.sessionCoord != nil {
			coord = s.sessionCoord
		}
		// Guard the typed-nil trap: assigning a nil *SessionManager straight
		// into the interface would make `Sessions != nil` true while the
		// receiver is nil.
		var sessions sessionapi.Sessions
		if s.sessionMgr != nil {
			sessions = s.sessionMgr
		}
		s.sessionAPIHandlers = sessionapi.NewHandlers(sessionapi.Deps{
			Sessions:     sessions,
			Coordinator:  coord,
			State:        state,
			Storage:      s.storage,
			Agent:        s.agent,
			ResolveAgent: s.resolveAgentForRequest,
			EnrichCtx:    s.enrichContextWithWorkspaceTools,
			OffloadHints: func(sessionID, text string) string {
				return injectOffloadHints(s, sessionID, text)
			},
			// Resume commands persisted without a local waiter notify the
			// worker tier through the wakeup stream (18.5): the dispatcher
			// that wins the session lease consumes the command.
			OnResumeNotified: func(sessionID string) {
				if tb := messagebus.AsTeamBus(s.messageBus); tb != nil {
					_ = tb.EnqueueWakeup(context.Background(), sessionID)
				}
			},
			// Write endpoints share the gateway body cap (18.11).
			MaxBodyBytes: maxBodyBytes,
		})
	})
	return s.sessionAPIHandlers
}

// --- Public-surface re-exports (aliases) ---

// HeaderAgentSessionID carries the session id between client and server.
const HeaderAgentSessionID = sessionapi.HeaderAgentSessionID

// AGUI types re-exported from sessionapi for existing clients and tests.
type (
	AGUIConvertOptions   = sessionapi.AGUIConvertOptions
	AGUIConverter        = sessionapi.AGUIConverter
	DefaultAGUIConverter = sessionapi.DefaultAGUIConverter
)

// NewDefaultAGUIConverter creates an AG-UI converter.
var NewDefaultAGUIConverter = sessionapi.NewDefaultAGUIConverter

// EncodeStreamEvent serializes an AgentEvent for SSE/WS transport.
var EncodeStreamEvent = sessionapi.EncodeStreamEvent

// BroadcastToRoom sends a JSON message to all sessions in a room.
func (s *Server) BroadcastToRoom(room string, v interface{}) {
	s.sessionAPI().BroadcastToRoom(room, v)
}

// SessionCount returns the number of active WebSocket sessions.
func (s *Server) SessionCount() int {
	return s.sessionAPI().SessionCount()
}

// legacyWSHandler adapts the lazily-built session handlers onto the
// construction-time /chat/ws route.
func (s *Server) legacyWSHandler(w http.ResponseWriter, r *http.Request) {
	s.sessionAPI().HandleChatWS(w, r)
}
