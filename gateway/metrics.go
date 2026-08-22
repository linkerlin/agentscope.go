package gateway

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/linkerlin/agentscope.go/observability"
)

// WithMetricsRegistry serves GET /metrics (Prometheus exposition) from the
// given registry. When a control plane is attached, its governance counters
// (should-run decisions, spends, gates, writebacks, rewards) are registered
// automatically. Chain before RegisterV2Routes/Start so routes order freely.
func (s *Server) WithMetricsRegistry(reg *prometheus.Registry) *Server {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	if s.controlPlane != nil {
		reg.MustRegister(observability.NewControlPlaneCollector(s.controlPlane))
	}
	s.mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return s
}
