package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/linkerlin/agentscope.go/controlplane"
)

// ControlPlaneCollector exposes the control-plane Kernel's runtime counters
// (ShouldRun decisions, spend, gates, writebacks, rewards) as Prometheus
// counters. Attach it to a registry:
//
//	reg.MustRegister(observability.NewControlPlaneCollector(kernel))
//	http.Handle("/metrics", promhttp.HandlerFor(reg, nil))
//
// The collector pulls a MetricsSnapshot on every scrape — no background
// goroutine, no cardinality beyond the fixed metric set.
type ControlPlaneCollector struct {
	kernel *controlplane.Kernel
}

// NewControlPlaneCollector builds a collector over the given Kernel.
func NewControlPlaneCollector(k *controlplane.Kernel) *ControlPlaneCollector {
	return &ControlPlaneCollector{kernel: k}
}

// cpMetrics maps descriptor name -> help text for the exported counters.
var cpMetrics = map[string]string{
	"should_run_eligible": "ShouldRun turns deemed eligible",
	"should_run_blocked":  "ShouldRun turns blocked (quota, gates, policy)",
	"spend_executed":      "Validated spends executed",
	"gates_opened":        "User gates opened",
	"gates_resolved":      "User gates resolved",
	"writebacks":          "Validated writebacks recorded",
	"supersedes":          "Todos superseded",
	"rewards_recorded":    "Reward records added",
	"rewards_revoked":     "Reward records revoked",
}

// Describe implements prometheus.Collector.
func (c *ControlPlaneCollector) Describe(ch chan<- *prometheus.Desc) {
	for name, help := range cpMetrics {
		ch <- prometheus.NewDesc("agentscope_controlplane_"+name+"_total", help, nil, nil)
	}
}

// Collect implements prometheus.Collector.
func (c *ControlPlaneCollector) Collect(ch chan<- prometheus.Metric) {
	if c.kernel == nil {
		return
	}
	snap := c.kernel.Metrics()
	for name, value := range map[string]int64{
		"should_run_eligible": snap.ShouldRunEligible,
		"should_run_blocked":  snap.ShouldRunBlocked,
		"spend_executed":      snap.SpendExecuted,
		"gates_opened":        snap.GatesOpened,
		"gates_resolved":      snap.GatesResolved,
		"writebacks":          snap.Writebacks,
		"supersedes":          snap.Supersedes,
		"rewards_recorded":    snap.RewardsRecorded,
		"rewards_revoked":     snap.RewardsRevoked,
	} {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("agentscope_controlplane_"+name+"_total", cpMetrics[name], nil, nil),
			prometheus.CounterValue, float64(value),
		)
	}
}

var _ prometheus.Collector = (*ControlPlaneCollector)(nil)
