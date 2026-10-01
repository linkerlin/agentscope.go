package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/gateway/channelapi"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
)

// WithChannelGateway wires the channel subsystem into the server: a registry,
// a gateway (router + runner), and the dispatcher that starts every channel.
// nil-safe: without a gateway, channel routes are not registered.
func (s *Server) WithChannelGateway(reg *channel.Registry, gw *channel.Gateway) *Server {
	s.channelRegistry = reg
	s.channelGateway = gw
	if reg != nil && gw != nil {
		s.channelDispatcher = channel.NewDispatcher(gw, reg)
	}
	return s
}

// StartChannels launches every registered channel listener (idempotent).
// Called automatically from Start() when a channel gateway is wired.
func (s *Server) StartChannels() {
	if s.channelDispatcher != nil && !s.channelsStarted {
		s.channelsStarted = true
		go func() { _ = s.channelDispatcher.StartAll(context.Background()) }()
	}
}

// RegisterChannelRoutes registers the channel management + webhook endpoints
// through gateway/channelapi (16.2). No-op when no channel gateway is wired.
func (s *Server) RegisterChannelRoutes() {
	if s.channelRegistry == nil {
		return
	}
	s.channelHandlers().Register(s.mux, s.requireAuth)
}

// channelHandlers lazily builds the channelapi handler set. Both the agent
// resolver and the session-state view are read per request (the resolver and
// the state manager may be wired after RegisterChannelRoutes — the pre-16.2
// inline handler read them at request time, and tests rely on that).
func (s *Server) channelHandlers() *channelapi.Handlers {
	s.channelAPIBuild.Do(func() {
		s.channelAPIHandlers = channelapi.NewHandlers(channelapi.Deps{
			Registry: s.channelRegistry,
			Storage:  s.storage,
			State: func() sessionapi.SessionState {
				if s.sessionState == nil {
					return nil // typed-nil guard: nil *SessionStateManager must not enter the interface
				}
				return s.sessionState
			},
			ResolveAgent: func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
				resolve := s.dingtalkAgentResolver
				if resolve == nil {
					resolve = s.buildSessionAgentFromStorage
				}
				return resolve(ctx, agentID, sessionID)
			},
		})
	})
	return s.channelAPIHandlers
}
