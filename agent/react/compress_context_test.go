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

// --- Incremental Compression Tests ---

func TestMergeCompressionSummaries(t *testing.T) {
	prev := "<system-info>Here is a summary of your previous work\n" +
		"# Task Overview\nBuild a web app\n\n" +
		"# Current State\nBackend done\n\n" +
		"# Important Discoveries\nPostgres works\n\n" +
		"# Next Steps\nAdd frontend\n\n" +
		"# Context to Preserve\nUse TypeScript\n" +
		"</system-info>"

	delta := "<system-info>Here is a summary of your previous work\n" +
		"# Task Overview\nBuild a web app with API\n\n" +
		"# Current State\nBackend and frontend done\n\n" +
		"# Important Discoveries\nReact 19 is stable\n\n" +
		"# Next Steps\nDeploy to prod\n\n" +
		"# Context to Preserve\nUse pnpm\n" +
		"</system-info>"

	merged := mergeCompressionSummaries(prev, delta)

	if !strings.Contains(merged, "Postgres works") {
		t.Error("merge should preserve old ImportantDiscoveries")
	}
	if !strings.Contains(merged, "React 19 is stable") {
		t.Error("merge should include new ImportantDiscoveries")
	}
	if !strings.Contains(merged, "Use TypeScript") {
		t.Error("merge should preserve old ContextToPreserve")
	}
	if !strings.Contains(merged, "Use pnpm") {
		t.Error("merge should include new ContextToPreserve")
	}
	if !strings.Contains(merged, "Backend and frontend done") {
		t.Error("merge should use latest CurrentState")
	}
	if !strings.Contains(merged, "Deploy to prod") {
		t.Error("merge should use latest NextSteps")
	}
}

func TestExtractSection(t *testing.T) {
	text := "# Task Overview\nDo something\n\n# Current State\nWorking on it"
	if got := extractSection(text, "# Task Overview"); got != "Do something" {
		t.Fatalf("expected 'Do something', got '%s'", got)
	}
	if got := extractSection(text, "# Current State"); got != "Working on it" {
		t.Fatalf("expected 'Working on it', got '%s'", got)
	}
	if got := extractSection(text, "# Missing"); got != "" {
		t.Fatalf("expected empty for missing section, got '%s'", got)
	}
}

func TestMergeTexts(t *testing.T) {
	if got := mergeTexts("", "b"); got != "b" {
		t.Fatalf("expected 'b', got '%s'", got)
	}
	if got := mergeTexts("a", ""); got != "a" {
		t.Fatalf("expected 'a', got '%s'", got)
	}
	if got := mergeTexts("abc", "abc"); got != "abc" {
		t.Fatalf("dedup should avoid duplication, got '%s'", got)
	}
	if got := mergeTexts("aaa", "bbb"); got != "aaa\nbbb" {
		t.Fatalf("expected 'aaa\\nbbb', got '%s'", got)
	}
}

func TestPreTruncateToolResults(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	longContent := strings.Repeat("z", 2000)
	msg := message.NewMsg().Role(message.RoleTool).
		Content(message.NewToolResultBlock("call-1", []message.ContentBlock{
			message.NewTextBlock(longContent),
		}, false)).
		Build()

	result := preTruncateToolResults(m, []*message.Msg{msg}, 100)
	if len(result) != 1 {
		t.Fatal("expected 1 message")
	}
	for _, b := range result[0].Content {
		if tr, ok := b.(*message.ToolResultBlock); ok {
			text := blocksTextSummary(tr.Content)
			if len(text) >= len(longContent) {
				t.Error("expected tool result to be truncated")
			}
		}
	}
}

func TestTruncateSummaryText(t *testing.T) {
	long := strings.Repeat("a", 200)
	result := truncateSummaryText(long, 100)
	if len(result) > 110 {
		t.Fatalf("expected <= 110 chars, got %d", len(result))
	}
	if !strings.Contains(result, "TRUNCATED") {
		t.Error("expected truncation marker")
	}

	short := "short"
	if got := truncateSummaryText(short, 100); got != short {
		t.Error("short text should not be truncated")
	}
}

func TestCompressionWatermark(t *testing.T) {
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

	wmBefore := a.getCompressionWatermark()
	if wmBefore != 0 {
		t.Fatalf("expected watermark 0, got %d", wmBefore)
	}

	err = a.CompressContext(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), nil)
	if err != nil {
		t.Fatal(err)
	}

	wmAfter := a.getCompressionWatermark()
	if wmAfter <= 0 {
		t.Fatalf("expected watermark > 0 after compression, got %d", wmAfter)
	}
}
