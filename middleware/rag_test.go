package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

type stubAgent struct{ name string }

func (s *stubAgent) AgentName() string { return s.name }

// stubSearcher returns deterministic hits based on the query keyword.
func stubSearcher(ctx context.Context, query string, topK int) ([]KnowledgeHit, error) {
	if strings.Contains(query, "boom") {
		return nil, errors.New("searcher exploded")
	}
	return []KnowledgeHit{
		{Text: "PTO is 15 days", Score: 0.9, Source: "onboarding.md"},
		{Text: "Rotate keys every 90 days", Score: 0.8, Source: "security.md"},
	}, nil
}

func newUserMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}

func TestRAGMiddleware_StaticInjectsHint(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "handbook").WithMode(RAGModeStatic)
	ctx := context.Background()
	called := false
	final := func(ctx context.Context) (*message.Msg, error) {
		// The OnReasoning should have appended a hint message.
		called = true
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}
	input := &ReplyInput{Messages: []*message.Msg{newUserMsg("How many PTO days?")}}
	msg, err := mw.OnReply(ctx, &stubAgent{"A"}, input, func(ctx context.Context) (*message.Msg, error) {
		// OnReply stores hits in context; simulate OnReasoning reading them.
		rin := &ReasoningInput{Messages: input.Messages}
		return mw.OnReasoning(ctx, &stubAgent{"A"}, rin, final)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("final handler not called")
	}
	if msg == nil {
		t.Fatal("nil reply")
	}
}

func TestRAGMiddleware_HintContainsHits(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "handbook").WithMode(RAGModeStatic)
	ctx := context.Background()
	var captured []*message.Msg
	final := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("answer").Build(), nil
	}
	mw.OnReply(ctx, &stubAgent{"A"}, &ReplyInput{Messages: []*message.Msg{newUserMsg("PTO?")}}, func(ctx context.Context) (*message.Msg, error) {
		rin := &ReasoningInput{Messages: []*message.Msg{newUserMsg("PTO?")}}
		mw.OnReasoning(ctx, &stubAgent{"A"}, rin, func(ctx context.Context) (*message.Msg, error) {
			captured = rin.Messages
			return final(ctx)
		})
		return final(ctx)
	})

	if len(captured) < 2 {
		t.Fatalf("expected hint message appended, got %d messages", len(captured))
	}
	hint := captured[len(captured)-1]
	var hb *message.HintBlock
	for _, b := range hint.Content {
		if h, ok := b.(*message.HintBlock); ok {
			hb = h
		}
	}
	if hb == nil {
		t.Fatal("last message should carry a HintBlock")
	}
	if !strings.Contains(hb.Text, "PTO is 15 days") {
		t.Fatalf("hint missing hit text: %q", hb.Text)
	}
	if !strings.Contains(hb.Text, "onboarding.md") {
		t.Fatalf("hint missing source citation: %q", hb.Text)
	}
}

func TestRAGMiddleware_AgentModeNoInjection(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "handbook").WithMode(RAGModeAgent)
	injected := false
	final := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}
	rin := &ReasoningInput{Messages: []*message.Msg{newUserMsg("PTO?")}}
	mw.OnReasoning(context.Background(), &stubAgent{"A"}, rin, func(ctx context.Context) (*message.Msg, error) {
		// If a hint was appended, the message count grew.
		if len(rin.Messages) > 1 {
			injected = true
		}
		return final(ctx)
	})
	if injected {
		t.Fatal("agent mode should not inject hints")
	}
}

func TestRAGMiddleware_EmptyQueryNoSearch(t *testing.T) {
	searched := false
	mw := NewRAGMiddleware(func(ctx context.Context, q string, k int) ([]KnowledgeHit, error) {
		searched = true
		return nil, nil
	}, "kb").WithMode(RAGModeStatic)
	mw.OnReply(context.Background(), &stubAgent{"A"}, &ReplyInput{Messages: []*message.Msg{newUserMsg("")}}, func(ctx context.Context) (*message.Msg, error) {
		return nil, nil
	})
	if searched {
		t.Fatal("should not search on empty query")
	}
}

func TestRAGMiddleware_SearchErrorSwallowed(t *testing.T) {
	// A failing searcher should not break the reply; hints are simply absent.
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeStatic)
	proceeded := false
	mw.OnReply(context.Background(), &stubAgent{"A"}, &ReplyInput{Messages: []*message.Msg{newUserMsg("boom")}}, func(ctx context.Context) (*message.Msg, error) {
		proceeded = true
		return nil, nil
	})
	if !proceeded {
		t.Fatal("search error should not abort the reply")
	}
}

func TestRAGMiddleware_MinScoreFilter(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeStatic).WithMinScore(0.85)
	ctx := context.Background()
	mw.OnReply(ctx, &stubAgent{"A"}, &ReplyInput{Messages: []*message.Msg{newUserMsg("days")}}, func(ctx context.Context) (*message.Msg, error) {
		state, _ := ctx.Value(ragCtxKey{}).(*ragState)
		if state == nil {
			t.Fatal("no rag state")
		}
		for _, h := range state.hits {
			if h.Score < 0.85 {
				t.Fatalf("hit below min-score leaked: %.2f", h.Score)
			}
		}
		return nil, nil
	})
}

func TestRAGMiddleware_ToolsStaticEmpty(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeStatic)
	if tools := mw.Tools(); len(tools) != 0 {
		t.Fatalf("static mode should expose no tools, got %d", len(tools))
	}
}

func TestRAGMiddleware_ToolsAgentMode(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeAgent)
	tools := mw.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name() != "search_knowledge" {
		t.Fatalf("tool name = %q", tools[0].Name())
	}
	resp, err := tools[0].Execute(context.Background(), map[string]any{"query": "PTO"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "PTO is 15 days") {
		t.Fatalf("tool result = %q", resp.GetTextContent())
	}
}

func TestRAGMiddleware_OnSystemPrompt(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeAgent)
	out, err := mw.OnSystemPrompt(context.Background(), &stubAgent{"A"}, "base")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "search_knowledge") {
		t.Fatalf("agent mode prompt should mention tool: %q", out)
	}
	// Static mode should not modify the prompt.
	mw2 := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeStatic)
	out2, _ := mw2.OnSystemPrompt(context.Background(), &stubAgent{"A"}, "base")
	if out2 != "base" {
		t.Fatal("static mode should not change system prompt")
	}
}

func TestRAGSearchTool_EmptyQuery(t *testing.T) {
	mw := NewRAGMiddleware(stubSearcher, "kb").WithMode(RAGModeAgent)
	tl := mw.Tools()[0]
	if _, err := tl.Execute(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error for empty query")
	}
}

// ensure the tool satisfies tool.Tool
var _ tool.Tool = (*ragSearchTool)(nil)
