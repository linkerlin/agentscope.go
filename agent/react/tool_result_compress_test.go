package react

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type tokenCountModel struct {
	mockChatModel
	limit int //nolint:unused // test config field, reserved
}

func (m *tokenCountModel) CountTokens(messages []*message.Msg, tools []model.ToolSpec) (int, error) {
	total := 0
	for _, msg := range messages {
		total += len(msg.GetTextContent()) / 4
	}
	return total, nil
}

func TestSplitToolResultForCompression_BelowLimit(t *testing.T) {
	block := message.NewToolResultBlock("t1", []message.ContentBlock{
		message.NewTextBlock("short"),
	}, false)
	reserved, offload, err := SplitToolResultForCompression(&tokenCountModel{}, block, 100)
	if err != nil || offload != nil {
		t.Fatalf("expected no split, err=%v offload=%v", err, offload)
	}
	if blocksTextSummary(reserved.Content) != "short" {
		t.Fatalf("unexpected reserved text: %q", blocksTextSummary(reserved.Content))
	}
}

func TestSplitToolResultForCompression_TruncatesLargeText(t *testing.T) {
	long := strings.Repeat("x", 800)
	block := message.NewToolResultBlock("t1", []message.ContentBlock{
		message.NewTextBlock(long),
	}, false)
	reserved, offload, err := SplitToolResultForCompression(&tokenCountModel{}, block, 50)
	if err != nil {
		t.Fatal(err)
	}
	if offload == nil {
		t.Fatal("expected offload block")
	}
	if len(blocksTextSummary(reserved.Content)) >= len(long) {
		t.Fatal("expected reserved portion to be shorter")
	}
}

func TestCompressToolResultBlocks_AddsReminder(t *testing.T) {
	long := strings.Repeat("y", 800)
	a, err := Builder().
		Name("t").
		Model(&tokenCountModel{}).
		ContextConfig(agent.DefaultContextConfig()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	a.contextConfig.ToolResultLimit = 50
	out := a.compressToolResultBlocks(context.Background(), "t1", []message.ContentBlock{
		message.NewTextBlock(long),
	}, false)
	if !strings.Contains(blocksTextSummary(out), "<<<TRUNCATED>>>") {
		t.Fatalf("expected truncation reminder, got %q", blocksTextSummary(out))
	}
}

// TestSplitToolResultForCompression_DoesNotMutateSource guards the deep-copy
// requirement: a shallow block copy made every truncation pass write its own
// prefix back into the tool result stored in memory.
func TestSplitToolResultForCompression_DoesNotMutateSource(t *testing.T) {
	long := strings.Repeat("z", 800)
	block := message.NewToolResultBlock("t1", []message.ContentBlock{
		message.NewTextBlock(long),
	}, false)
	before := blocksTextSummary(block.Content)

	if _, _, err := SplitToolResultForCompression(&tokenCountModel{}, block, 50); err != nil {
		t.Fatal(err)
	}
	if after := blocksTextSummary(block.Content); after != before {
		t.Fatalf("source tool result was mutated: before %d chars, after %d chars", len(before), len(after))
	}
}

func TestNewToolResultBlock_PreservesBlockFields(t *testing.T) {
	src := &message.ToolResultBlock{
		ID:        "id1",
		Name:      "search",
		ToolUseID: "tu1",
		Content:   []message.ContentBlock{message.NewTextBlock("x")},
		IsError:   true,
		State:     "success",
	}
	got := newToolResultBlock(src, []message.ContentBlock{message.NewTextBlock("y")})
	if got.ID != src.ID || got.Name != src.Name || got.ToolUseID != src.ToolUseID {
		t.Fatalf("identity fields lost: %+v", got)
	}
	if !got.IsError || got.State != src.State {
		t.Fatalf("state fields lost: %+v", got)
	}
}

func TestCloneContentBlocks_DeepCopies(t *testing.T) {
	src := message.NewTextBlock("original")
	cloned := cloneContentBlocks([]message.ContentBlock{src})
	text, ok := cloned[0].(*message.TextBlock)
	if !ok {
		t.Fatal("expected TextBlock")
	}
	if text == src {
		t.Fatal("clone shares the same pointer as the source block")
	}
	text.Text = "mutated"
	if src.Text != "original" {
		t.Fatalf("source block mutated through clone: %q", src.Text)
	}
}
