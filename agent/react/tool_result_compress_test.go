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
	limit int
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
