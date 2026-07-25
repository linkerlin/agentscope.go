package middleware

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// reflectChatStub is a minimal model.ChatModel that returns a canned reply.
type reflectChatStub struct {
	reply string
	err   error
	calls int32
}

func (s *reflectChatStub) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.err != nil {
		return nil, s.err
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(s.reply).Build(), nil
}

func (s *reflectChatStub) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (s *reflectChatStub) ModelName() string { return "reflect-stub" }

// captureWriter collects facts for inspection.
type captureWriter struct {
	mu    sync.Mutex
	facts []string
}

func (w *captureWriter) WriteFact(ctx context.Context, fact string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.facts = append(w.facts, fact)
	return nil
}

type agentStub string

func (a agentStub) AgentName() string { return string(a) }

// TestParseFactsJSON covers the strict-JSON parser against common LLM quirks.
func TestParseFactsJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"clean", `{"facts":["a","b"]}`, []string{"a", "b"}},
		{"empty", `{"facts":[]}`, []string{}},
		{"markdown fence", "```json\n{\"facts\":[\"x\"]}\n```", []string{"x"}},
		{"trailing prose", `Here you go: {"facts":["y"]} hope this helps!`, []string{"y"}},
		{"empty raw", "", nil},
		{"garbage", "no json here", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseFactsJSON(c.raw)
			if err != nil && len(c.want) > 0 {
				t.Fatalf("parseFactsJSON(%q): %v", c.raw, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("len mismatch: got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("idx %d: got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestBuildReflectionPrompt verifies the dialogue content makes it into the prompt.
func TestBuildReflectionPrompt(t *testing.T) {
	p := buildReflectionPrompt("I love Python", "Glad to hear that!")
	if !strings.Contains(p, "I love Python") || !strings.Contains(p, "Glad to hear that!") {
		t.Fatalf("prompt missing dialogue content: %s", p)
	}
	if !strings.Contains(p, "STRICT JSON") {
		t.Errorf("prompt missing strict-JSON directive")
	}
}

// TestReflectionMiddleware_AcquireRelease exercises the semaphore bound.
func TestReflectionMiddleware_AcquireRelease(t *testing.T) {
	m := &ReflectionMiddleware{Workers: 1}
	m.initSem()
	if cap(m.sem) != 1 {
		t.Fatalf("expected sem cap 1, got %d", cap(m.sem))
	}
	if !m.acquire() {
		t.Fatal("first acquire should succeed")
	}
	if m.acquire() {
		t.Fatal("second acquire should fail (capacity=1)")
	}
	m.release()
	if !m.acquire() {
		t.Fatal("acquire after release should succeed")
	}
	m.release()
}

// TestReflectionMiddleware_NilSafe verifies OnReply passthrough when disabled.
func TestReflectionMiddleware_NilSafe(t *testing.T) {
	m := &ReflectionMiddleware{} // no Model/Writer
	called := false
	next := func(ctx context.Context) (*message.Msg, error) {
		called = true
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}
	input := &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}}
	out, err := m.OnReply(context.Background(), agentStub("a"), input, next)
	if err != nil {
		t.Fatalf("OnReply: %v", err)
	}
	if !called {
		t.Error("next not called when middleware disabled")
	}
	if out == nil || out.GetTextContent() != "ok" {
		t.Errorf("expected passthrough reply, got %v", out)
	}
}

// TestReflectionMiddleware_HappyPath wires stubbed model + capture writer and
// verifies facts flow through end-to-end.
func TestReflectionMiddleware_HappyPath(t *testing.T) {
	modelStub := &reflectChatStub{reply: `{"facts":["User prefers Go", "User is in Tokyo"]}`}
	w := &captureWriter{}
	m := &ReflectionMiddleware{
		Model:    modelStub,
		Writer:   w,
		MaxFacts: 5,
		Workers:  1,
	}
	next := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("reply").Build(), nil
	}
	input := &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}}
	if _, err := m.OnReply(context.Background(), agentStub("a"), input, next); err != nil {
		t.Fatalf("OnReply: %v", err)
	}
	// Reflection runs in a goroutine — wait for facts to arrive.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		n := len(w.facts)
		w.mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.facts) != 2 {
		t.Fatalf("expected 2 facts captured, got %d: %v", len(w.facts), w.facts)
	}
}

// TestReflectionMiddleware_DropsWhenWriterErr verifies that a Writer error
// does not crash the goroutine (other facts still attempted).
func TestReflectionMiddleware_DropsWhenWriterErr(t *testing.T) {
	modelStub := &reflectChatStub{reply: `{"facts":["a","b"]}`}
	w := FactWriterFunc(func(ctx context.Context, fact string) error {
		return errors.New("simulated write failure")
	})
	m := &ReflectionMiddleware{
		Model:   modelStub,
		Writer:  w,
		Workers: 1,
	}
	next := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("r").Build(), nil
	}
	input := &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	}}
	if _, err := m.OnReply(context.Background(), agentStub("a"), input, next); err != nil {
		t.Fatalf("OnReply should not surface writer errors: %v", err)
	}
	// Give the goroutine a moment to ensure no panic.
	time.Sleep(150 * time.Millisecond)
}

// TestReflectionMiddleware_BusyDrop verifies that when all workers are busy,
// the reflection is silently dropped (never blocks OnReply).
func TestReflectionMiddleware_BusyDrop(t *testing.T) {
	modelStub := &reflectChatStub{reply: `{"facts":["x"]}`}
	w := &captureWriter{}
	m := &ReflectionMiddleware{
		Model:   modelStub,
		Writer:  w,
		Workers: 1,
	}
	// Pre-acquire the single worker slot.
	m.initSem()
	if !m.acquire() {
		t.Fatal("pre-acquire should succeed")
	}
	defer m.release()

	next := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("r").Build(), nil
	}
	input := &ReplyInput{Messages: []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}}
	start := time.Now()
	if _, err := m.OnReply(context.Background(), agentStub("a"), input, next); err != nil {
		t.Fatalf("OnReply: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("OnReply blocked for %v when it should drop and return immediately", elapsed)
	}
	time.Sleep(100 * time.Millisecond)
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.facts) != 0 {
		t.Fatalf("expected 0 facts (worker busy), got %d", len(w.facts))
	}
}
