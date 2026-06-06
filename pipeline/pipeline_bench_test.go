package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// BenchmarkPipeline_Sequential measures the latency of a sequential pipeline
// with 3 fast steps.
func BenchmarkPipeline_Sequential(b *testing.B) {
	p := New("seq",
		&mockParallelAgent{name: "a", latency: 1 * time.Millisecond, response: "A"},
		&mockParallelAgent{name: "b", latency: 1 * time.Millisecond, response: "B"},
		&mockParallelAgent{name: "c", latency: 1 * time.Millisecond, response: "C"},
	)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Call(context.Background(), msg)
	}
}

// BenchmarkPipeline_Parallel measures the latency of a parallel pipeline
// with 3 concurrent steps.
func BenchmarkPipeline_Parallel(b *testing.B) {
	p := NewParallel("par",
		&mockParallelAgent{name: "a", latency: 5 * time.Millisecond, response: "A"},
		&mockParallelAgent{name: "b", latency: 5 * time.Millisecond, response: "B"},
		&mockParallelAgent{name: "c", latency: 5 * time.Millisecond, response: "C"},
	)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Call(context.Background(), msg)
	}
}
