package react

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type summaryMockModel struct {
	mockChatModel
}

func (m *summaryMockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.lastMessages = messages
	raw := `{"task_overview":"do task","current_state":"in progress","important_discoveries":"none","next_steps":"continue","context_to_preserve":"prefs"}`
	return message.NewMsg().Role(message.RoleAssistant).TextContent(raw).Build(), nil
}

func TestCompressContext_SkipsBelowThreshold(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent("short").Build())

	a, err := Builder().
		Name("t").
		Model(&summaryMockModel{mockChatModel: mockChatModel{name: "mock"}}).
		Memory(mem).
		ContextSize(10000).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	err = a.CompressContext(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.getCompressedSummary() != "" {
		t.Fatal("expected no summary below threshold")
	}
	all, _ := mem.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected memory unchanged, got %d msgs", len(all))
	}
}

func TestCompressContext_TriggersAndReplacesMemory(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	long := strings.Repeat("x", 400)
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent(long).Build())
	_ = mem.Add(message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build())
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent("recent").Build())

	a, err := Builder().
		Name("t").
		Model(&summaryMockModel{mockChatModel: mockChatModel{name: "mock"}}).
		Memory(mem).
		SysPrompt("sys").
		ContextSize(100).
		ContextConfig(agent.ContextConfig{TriggerRatio: 0.8, ReserveRatio: 0.1}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	err = a.CompressContext(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.getCompressedSummary() == "" {
		t.Fatal("expected compressed summary")
	}
	all, _ := mem.GetAll()
	if len(all) == 3 {
		t.Fatalf("expected memory to shrink after compression, still have %d msgs", len(all))
	}
}

func TestCompressContext_SkipsReMeMemory(t *testing.T) {
	mockMem := newMockReMeMemory()
	a, err := Builder().
		Name("t").
		Model(&summaryMockModel{mockChatModel: mockChatModel{name: "mock"}}).
		Memory(mockMem).
		ContextSize(10).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = mockMem.Add(message.NewMsg().Role(message.RoleUser).TextContent(strings.Repeat("a", 400)).Build())
	err = a.CompressContext(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.getCompressedSummary() != "" {
		t.Fatal("ReMe memory should use PreReasoningPrepare instead")
	}
}
