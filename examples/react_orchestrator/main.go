// examples/react_orchestrator/main.go
//
// Demo: ReAct step recorder and orchestrator with memory injection.
//
// This demo shows how to create a ReactStepRecorder, a ReactOrchestrator,
// and inject retrieved memories into a reasoning step. A mock vector store
// is used so no real embedding service is required.
//
// How to run:
//   cd examples/react_orchestrator && go run main.go

package main

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// mockVectorStore is a minimal VectorStore for demo purposes.
type mockVectorStore struct{}

func (m *mockVectorStore) Insert(ctx context.Context, nodes []*memory.MemoryNode) error { return nil }
func (m *mockVectorStore) Search(ctx context.Context, query string, opts memory.RetrieveOptions) ([]*memory.MemoryNode, error) {
	return []*memory.MemoryNode{
		memory.NewMemoryNode(memory.MemoryTypePersonal, "user-1", "User prefers concise answers."),
	}, nil
}
func (m *mockVectorStore) Get(ctx context.Context, memoryID string) (*memory.MemoryNode, error) {
	return nil, nil
}
func (m *mockVectorStore) Update(ctx context.Context, node *memory.MemoryNode) error { return nil }
func (m *mockVectorStore) Delete(ctx context.Context, memoryID string) error         { return nil }
func (m *mockVectorStore) DeleteAll(ctx context.Context) error                       { return nil }

func main() {
	ctx := context.Background()

	// 1. Create a step recorder with in-memory storage (demonstrates the API).
	_ = memory.NewReactStepRecorder(nil)

	// 2. Create a ReAct orchestrator with memory injection enabled.
	store := &mockVectorStore{}
	config := memory.DefaultReactOrchestratorConfig()
	config.EnableMemoryInjection = true
	config.MaxInjectedMemories = 3
	orch := memory.NewReactOrchestrator(nil, store, config)

	// 3. Simulate a reasoning step and inject related memories.
	query := "How do I use Agentscope?"
	history := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent(query).Build(),
	}

	nodes, injectMsg, err := orch.InjectMemory(ctx, query, history, "user-1", "onboarding")
	if err != nil {
		fmt.Println("inject memory error:", err)
		return
	}
	fmt.Printf("injected %d memory nodes\n", len(nodes))
	if injectMsg != nil {
		fmt.Printf("injection message length=%d\n", len(injectMsg.GetTextContent()))
	}

	// 4. Record a reasoning step.
	step, err := orch.RecordReActStep(ctx, 1, memory.StepReasoning, history, nodes)
	if err != nil {
		fmt.Println("record step error:", err)
		return
	}
	fmt.Printf("recorded step id=%s type=%s iteration=%d\n", step.ID, step.Type, step.Iteration)

	// 5. Retrieve step history.
	steps, err := orch.GetStepHistory(ctx)
	if err != nil {
		fmt.Println("step history error:", err)
		return
	}
	fmt.Printf("total steps recorded: %d\n", len(steps))
}
