package runcontext

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

type stubModel struct{ name string }

func (s *stubModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return nil, nil
}

func (s *stubModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, nil
}

func (s *stubModel) ModelName() string { return s.name }

func TestModelContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := Model(ctx); got != nil {
		t.Fatalf("expected nil model, got %v", got)
	}
	m := &stubModel{name: "cheap"}
	ctx = WithModel(ctx, m)
	got := Model(ctx)
	if got == nil || got.ModelName() != "cheap" {
		t.Fatalf("expected cheap model, got %v", got)
	}
	if WithModel(ctx, nil) != ctx {
		t.Fatal("nil model must not replace the context")
	}
}
