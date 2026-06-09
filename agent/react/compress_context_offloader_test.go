package react

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/workspace"
)

func TestCompressContext_OffloadsCompactedMessages(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	long := strings.Repeat("z", 400)
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent(long).Build())
	_ = mem.Add(message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build())
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent("recent").Build())

	off := workspace.NewMemoryOffloader()
	a, err := Builder().
		Name("t").
		Model(&summaryMockModel{mockChatModel: mockChatModel{name: "mock"}}).
		Memory(mem).
		SysPrompt("sys").
		ContextSize(100).
		ContextConfig(agent.ContextConfig{TriggerRatio: 0.8, ReserveRatio: 0.1}).
		Offloader(off).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	err = a.CompressContext(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil)
	if err != nil {
		t.Fatal(err)
	}
	summary := a.getCompressedSummary()
	if !strings.Contains(summary, "offloaded to") {
		t.Fatalf("expected offload reminder in summary, got %q", summary)
	}
}
