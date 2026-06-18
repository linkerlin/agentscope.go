package middleware_test

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/tool"
)

func userMsg(text string) *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
}

// hintTextInReasoning runs a single-reply simulation (reply -> reasoning ->
// model call) with the given middleware and returns the concatenated text of
// any HintBlock messages appended to the reasoning input, plus whether a
// memory hint was injected.
func runLTMReply(t *testing.T, mw *middleware.LongTermMemoryMiddleware, query string) (hintText string) {
	t.Helper()
	chain := middleware.Classify([]middleware.Middleware{mw})
	agent := stubAgent{name: "mem-agent"}

	replyHandler := middleware.ChainReply(chain, agent, &middleware.ReplyInput{
		Messages: []*message.Msg{userMsg(query)},
	}, func(ctx context.Context) (*message.Msg, error) {
		rInput := &middleware.ReasoningInput{
			Iteration: 0,
			Messages:  []*message.Msg{userMsg(query)},
			ChatOpts:  nil,
		}
		rHandler := middleware.ChainReasoning(chain, agent, rInput, func(ctx context.Context) (*message.Msg, error) {
			return newAssistantMsg("ok"), nil
		})
		if _, err := rHandler(ctx); err != nil {
			t.Fatalf("reasoning: %v", err)
		}
		// Capture any hint block appended to the reasoning input.
		for _, m := range rInput.Messages {
			for _, b := range m.Content {
				if hb, ok := b.(*message.HintBlock); ok {
					hintText += hb.Text
				}
			}
		}
		return newAssistantMsg("I will help."), nil
	})
	if _, err := replyHandler(context.Background()); err != nil {
		t.Fatalf("reply: %v", err)
	}
	return hintText
}

func TestInMemoryLongTermMemory_SearchAdd(t *testing.T) {
	m := middleware.NewInMemoryLongTermMemory()
	if err := m.Add(context.Background(), []string{"likes espresso", "lives in Tokyo", "owns a cat"}, middleware.AddOptions{UserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Search(context.Background(), "espresso", middleware.SearchOptions{TopK: 5, UserID: "u1"})
	if len(got) != 1 || !strings.Contains(got[0].Text, "espresso") {
		t.Fatalf("expected espresso match, got %+v", got)
	}
	// Empty query returns up to TopK.
	all, _ := m.Search(context.Background(), "", middleware.SearchOptions{TopK: 10, UserID: "u1"})
	if len(all) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(all))
	}
	// User isolation.
	if other, _ := m.Search(context.Background(), "espresso", middleware.SearchOptions{UserID: "other"}); len(other) != 0 {
		t.Fatalf("expected user isolation, got %+v", other)
	}
}

func TestLongTermMemoryMiddleware_StaticControlInjectsAndWritesBack(t *testing.T) {
	backend := middleware.NewInMemoryLongTermMemory()
	_ = backend.Add(context.Background(), []string{"the user prefers concise answers"}, middleware.AddOptions{UserID: "alice"})

	mw := middleware.NewLongTermMemoryMiddleware(backend, "alice").
		WithMode(middleware.MemoryModeStaticControl)

	// Query "concise" is a substring of the stored memory, so the in-memory
	// backend retrieves it and the middleware injects it as a hint.
	hint := runLTMReply(t, mw, "concise")
	if !strings.Contains(hint, "concise answers") {
		t.Fatalf("expected injected memory hint to contain the memory, got: %q", hint)
	}
	// Write-back: the user query + assistant exchange should now be stored.
	snap := backend.Snapshot("alice")
	foundExchange := false
	for _, mem := range snap {
		if strings.Contains(mem.Text, "I will help") {
			foundExchange = true
		}
	}
	if !foundExchange {
		t.Fatalf("expected write-back to persist the exchange; snapshot=%+v", snap)
	}
}

func TestLongTermMemoryMiddleware_AgentControlNoInjection(t *testing.T) {
	backend := middleware.NewInMemoryLongTermMemory()
	_ = backend.Add(context.Background(), []string{"secret fact"}, middleware.AddOptions{UserID: "bob"})

	mw := middleware.NewLongTermMemoryMiddleware(backend, "bob").
		WithMode(middleware.MemoryModeAgentControl)

	hint := runLTMReply(t, mw, "secret")
	if hint != "" {
		t.Fatalf("agent_control must not inject memories, got: %q", hint)
	}
	// No automatic write-back in agent_control.
	snap := backend.Snapshot("bob")
	for _, mem := range snap {
		if strings.Contains(mem.Text, "secret") && len(snap) > 1 {
			t.Fatalf("agent_control must not write back; snapshot=%+v", snap)
		}
	}
}

func TestLongTermMemoryMiddleware_ToolsProvidedAndSystemPrompt(t *testing.T) {
	backend := middleware.NewInMemoryLongTermMemory()
	mwBoth := middleware.NewLongTermMemoryMiddleware(backend, "u").WithMode(middleware.MemoryModeBoth)
	tools := mwBoth.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools in both mode, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tt := range tools {
		names[tt.Name()] = true
	}
	if !names["search_memory"] || !names["add_memory"] {
		t.Fatalf("expected search_memory + add_memory, got %v", names)
	}

	// static_control provides no tools.
	mwStatic := middleware.NewLongTermMemoryMiddleware(backend, "u").WithMode(middleware.MemoryModeStaticControl)
	if got := mwStatic.Tools(); len(got) != 0 {
		t.Fatalf("static_control should expose no tools, got %d", len(got))
	}

	// System prompt advertises tools in both/agent_control, not static.
	out, err := mwBoth.OnSystemPrompt(context.Background(), stubAgent{name: "a"}, "base")
	if err != nil || !strings.Contains(out, "search_memory") {
		t.Fatalf("system prompt should advertise tools in both mode: %q err=%v", out, err)
	}
	out2, _ := mwStatic.OnSystemPrompt(context.Background(), stubAgent{name: "a"}, "base")
	if out2 != "base" {
		t.Fatalf("static_control system prompt should be unchanged: %q", out2)
	}
}

func TestMemoryTools_Execute(t *testing.T) {
	backend := middleware.NewInMemoryLongTermMemory()
	mw := middleware.NewLongTermMemoryMiddleware(backend, "carol").WithMode(middleware.MemoryModeBoth)

	// add_memory tool
	addTool := middleware.NewMemoryAddTool(backend, mw)
	resp, err := addTool.Execute(context.Background(), map[string]any{"text": "carol works remotely"})
	if err != nil {
		t.Fatal(err)
	}
	if len(backend.Snapshot("carol")) != 1 {
		t.Fatalf("add_memory did not persist; snapshot=%+v", backend.Snapshot("carol"))
	}
	_ = resp

	// search_memory tool finds it
	searchTool := middleware.NewMemorySearchTool(backend, mw)
	resp2, err := searchTool.Execute(context.Background(), map[string]any{"query": "remotely"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsText(resp2, "carol works remotely") {
		t.Fatalf("search_memory did not find the memory: %+v", resp2)
	}
	// empty query is short-circuited by the tool (LLM should supply a query)
	resp3, _ := searchTool.Execute(context.Background(), map[string]any{"query": ""})
	if !containsText(resp3, "empty query") {
		t.Fatalf("search_memory empty query should be guarded: %+v", resp3)
	}
	// spec sanity
	if searchTool.Name() != "search_memory" || addTool.Name() != "add_memory" {
		t.Fatal("tool names mismatch")
	}
	if searchTool.Spec().Name != "search_memory" {
		t.Fatal("spec name mismatch")
	}
}

func TestFuncLongTermMemory(t *testing.T) {
	searchCalled := false
	addCalled := false
	m := middleware.NewFuncLongTermMemory(
		func(ctx context.Context, q string, o middleware.SearchOptions) ([]middleware.Memory, error) {
			searchCalled = true
			return []middleware.Memory{{Text: "fn-memory"}}, nil
		},
		func(ctx context.Context, texts []string, o middleware.AddOptions) error {
			addCalled = true
			return nil
		},
	)
	ms, _ := m.Search(context.Background(), "x", middleware.SearchOptions{})
	if !searchCalled || len(ms) != 1 {
		t.Fatalf("func search failed: called=%v ms=%+v", searchCalled, ms)
	}
	_ = m.Add(context.Background(), []string{"y"}, middleware.AddOptions{})
	if !addCalled {
		t.Fatal("func add not called")
	}
	// nil func adapter is a safe no-op.
	nilAdapter := &middleware.FuncLongTermMemory{}
	if ms, _ := nilAdapter.Search(context.Background(), "x", middleware.SearchOptions{}); ms != nil {
		t.Fatal("nil SearchFn should return nil")
	}
	if err := nilAdapter.Add(context.Background(), nil, middleware.AddOptions{}); err != nil {
		t.Fatal("nil AddFn should return nil error")
	}
}

// containsText reports whether a tool.Response carries the substring in any
// text block.
func containsText(resp *tool.Response, sub string) bool {
	if resp == nil {
		return false
	}
	for _, b := range resp.Content {
		if tb, ok := b.(*message.TextBlock); ok && strings.Contains(tb.Text, sub) {
			return true
		}
	}
	return false
}
