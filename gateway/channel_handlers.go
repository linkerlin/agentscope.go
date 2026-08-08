package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/channel"
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

// RegisterChannelRoutes registers the channel management + webhook endpoints.
// No-op when no channel gateway is wired.
func (s *Server) RegisterChannelRoutes() {
	if s.channelRegistry == nil {
		return
	}
	s.mux.HandleFunc("GET /api/v1/channels", s.requireAuth(s.handleListChannels))
	s.mux.HandleFunc("POST /api/v1/channels/{id}/webhook", s.handleWebhookDelivery)
}

type channelInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	var out []channelInfo
	for _, c := range s.channelRegistry.List() {
		out = append(out, channelInfo{ID: c.ID(), Type: channelTypeOf(c)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func channelTypeOf(c channel.Channel) string {
	switch c.(type) {
	case *channel.WebhookChannel:
		return "webhook"
	default:
		return fmt.Sprintf("%T", c)
	}
}

// handleWebhookDelivery is the inbound HTTP endpoint for a webhook channel.
// The webhook channel is looked up by id and its ServeHTTP is invoked.
func (s *Server) handleWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c := s.channelRegistry.Get(id)
	if c == nil {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	wh, ok := c.(*channel.WebhookChannel)
	if !ok {
		http.Error(w, "channel is not a webhook", http.StatusBadRequest)
		return
	}
	wh.ServeHTTP(w, r)
}
