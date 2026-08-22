package observability

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/linkerlin/agentscope.go/controlplane"
)

func TestControlPlaneCollector(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	ctx := context.Background()

	// Drive a few counters: one gate open/resolve cycle and a reward record.
	_ = k.OpenGate(ctx, controlplane.UserGate{GateID: "gt1", GoalID: "g1", Question: "ok?"})
	_ = k.ResolveGate(ctx, "g1", "gt1", controlplane.GateOutcome{Decision: controlplane.DecisionApprove, By: "op"})
	_ = k.RecordReward(ctx, "g1", controlplane.RewardRecord{Class: controlplane.AuthorityHardPolicy, Content: "no direct push to main"})

	coll := NewControlPlaneCollector(k)

	if n := testutil.CollectAndCount(coll); n != len(cpMetrics) {
		t.Fatalf("expected %d metrics, got %d", len(cpMetrics), n)
	}
	lint, err := testutil.CollectAndLint(coll)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	for _, problem := range lint {
		t.Fatalf("lint: %s", problem.Text)
	}

	// Spot-check values via a real registry scrape.
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			values[mf.GetName()] = m.GetCounter().GetValue()
		}
	}
	for metric, want := range map[string]float64{
		"agentscope_controlplane_gates_opened_total":     1,
		"agentscope_controlplane_gates_resolved_total":   1,
		"agentscope_controlplane_rewards_recorded_total": 1,
	} {
		if values[metric] != want {
			t.Fatalf("%s = %v, want %v", metric, values[metric], want)
		}
	}
}

func TestControlPlaneCollectorNilKernel(t *testing.T) {
	coll := NewControlPlaneCollector(nil)
	if n := testutil.CollectAndCount(coll); n != 0 {
		t.Fatalf("nil kernel should expose no metrics, got %d", n)
	}
}
