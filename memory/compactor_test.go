package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type mockChatModel struct {
	reply string
	err   error
}

func (m *mockChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	text := m.reply
	if text == "" {
		text = sampleCompactMarkdown()
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(text).Build(), nil
}

func (m *mockChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (m *mockChatModel) ModelName() string { return "mock" }

func sampleCompactMarkdown() string {
	return `## Goal
finish task

## Constraints
- c1

## Progress
done

## Key Decisions
- d1

## Next Steps
- n1

## Critical Context
- ctx1
`
}

func TestCompactorCompact(t *testing.T) {
	ctx := context.Background()
	cm := &mockChatModel{}
	c := NewCompactor(cm)
	u := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	sum, err := c.Compact(ctx, []*message.Msg{u}, CompactOptions{Language: "zh", PreviousSummary: "prev"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Raw == "" || sum.Goal != "finish task" {
		t.Fatalf("goal=%q raw empty=%v", sum.Goal, sum.Raw == "")
	}
	if len(sum.Constraints) != 1 || sum.Constraints[0] != "- c1" {
		t.Fatal(sum.Constraints)
	}
}

func TestCompactorCompactChatError(t *testing.T) {
	cm := &mockChatModel{err: errors.New("boom")}
	c := NewCompactor(cm)
	_, err := c.Compact(context.Background(), []*message.Msg{}, CompactOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompactorNil(t *testing.T) {
	var c *Compactor
	_, err := c.Compact(context.Background(), nil, CompactOptions{})
	if !errors.Is(err, ErrCompactorNoModel) {
		t.Fatal(err)
	}
	c = NewCompactor(nil)
	_, err = c.Compact(context.Background(), nil, CompactOptions{})
	if !errors.Is(err, ErrCompactorNoModel) {
		t.Fatal(err)
	}
}

func TestParseCompactSummary(t *testing.T) {
	raw := sampleCompactMarkdown()
	s := parseCompactSummary(raw)
	if s.Goal != "finish task" || s.Progress != "done" {
		t.Fatal(s.Goal, s.Progress)
	}
	if len(s.KeyDecisions) != 1 || !strings.Contains(s.KeyDecisions[0], "d1") {
		t.Fatal(s.KeyDecisions)
	}
}

func TestSplitLinesEmpty(t *testing.T) {
	if splitLines("") != nil {
		t.Fatal("expected nil")
	}
	if len(splitLines("a\n\nb")) != 2 {
		t.Fatal(splitLines("a\n\nb"))
	}
}
