// examples/memory_benchmark/main.go
//
// Demo: Run memory system benchmarks and print results.
//
// This demo shows how to instantiate a VectorMemory and run the built-in
// benchmark suite. A mock embedder is used so no API key is required.
//
// How to run:
//   cd examples/memory_benchmark && go run main.go

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// stubEmbedder is a mock embedder for demo purposes.
type stubEmbedder struct{}

func (s *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text)), 0.2, 0.3, 0.4}, nil
}
func (s *stubEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, t := range texts {
		v, _ := s.Embed(ctx, t)
		out = append(out, v)
	}
	return out, nil
}

func main() {
	ctx := context.Background()

	// 1. Create a mock embedder and vector memory.
	embedder := &stubEmbedder{}
	cfg := memory.DefaultReMeFileConfig()
	cfg.WorkingDir = ".cache/bench_demo"
	vm, err := memory.NewReMeVectorMemory(cfg, memory.NewSimpleTokenCounter(), nil, embedder)
	if err != nil {
		fmt.Println("memory init error:", err)
		return
	}
	defer vm.Close()

	// 2. Seed some synthetic conversation messages.
	for i := 0; i < 20; i++ {
		msg := message.NewMsg().Role(message.RoleUser).TextContent(fmt.Sprintf("message number %d about agentscope", i)).Build()
		_ = vm.Add(msg)
	}

	// 3. Run the LoCoMo long-conversation benchmark.
	locomo := &memory.LoCoMoBenchmark{
		TestConversations: [][]*message.Msg{
			{
				message.NewMsg().Role(message.RoleUser).TextContent("What is Agentscope?").Build(),
				message.NewMsg().Role(message.RoleAssistant).TextContent("Agentscope is a multi-agent platform.").Build(),
			},
		},
		TestQueries:       []string{"multi-agent platform"},
		ExpectedRetrieval: []string{"Agentscope is a multi-agent platform."},
	}

	result, err := locomo.Run(ctx, vm)
	if err != nil {
		fmt.Println("benchmark error:", err)
		return
	}

	// 4. Print results.
	fmt.Printf("Benchmark: %s\n", result.Name)
	fmt.Printf("  OverallScore:   %.3f\n", result.OverallScore)
	fmt.Printf("  MemoryAccuracy: %.3f\n", result.MemoryAccuracy)
	fmt.Printf("  QAAccuracy:     %.3f\n", result.QAAccuracy)
	fmt.Printf("  TotalTime:      %s\n", result.TotalTime.Round(time.Millisecond))
	fmt.Printf("  MemoryCount:    %d\n", result.MemoryCount)
}
