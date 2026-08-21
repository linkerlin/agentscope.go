package console

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// fakeAgent is a scripted V2Agent for TUI state-machine tests.
type fakeAgent struct {
	name     string
	events   []event.AgentEvent
	stream   chan event.AgentEvent
	injected []event.AgentEvent
}

func (f *fakeAgent) Name() string { return f.name }

func (f *fakeAgent) Call(context.Context, *message.Msg) (*message.Msg, error) { return nil, nil }

func (f *fakeAgent) CallStream(context.Context, *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func (f *fakeAgent) Reply(context.Context, *message.Msg) (*message.Msg, error) { return nil, nil }

func (f *fakeAgent) ReplyStream(context.Context, *message.Msg) (<-chan event.AgentEvent, error) {
	f.stream = make(chan event.AgentEvent, len(f.events))
	for _, e := range f.events {
		f.stream <- e
	}
	return f.stream, nil
}

func (f *fakeAgent) LoadState(*agent.AgentState) error     { return nil }
func (f *fakeAgent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (f *fakeAgent) InjectEvent(_ context.Context, ev event.AgentEvent) error {
	f.injected = append(f.injected, ev)
	return nil
}

func newTestModel(fa *fakeAgent) *Model {
	m := newModel(context.Background(), fa, Options{UserName: "user", Verbosity: Default, MaxToolResultLines: 20})
	m.width, m.height = 80, 24
	m.layout()
	return m
}

func joined(m *Model) string { return strings.Join(m.lines, "\n") }

func TestConsole_TextStreamFlow(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{
		event.NewReplyStart("r1", "bot"),
		event.NewTextBlockStart("r1", 0),
		event.NewTextBlockDelta("r1", 0, "hello "),
		event.NewTextBlockDelta("r1", 0, "world"),
		event.NewTextBlockEnd("r1", 0),
		event.NewReplyEnd("r1", "bot"),
	}}
	m := newTestModel(fa)
	m.startReply("hi")
	if m.phase != phaseRunning {
		t.Fatalf("phase after startReply = %v, want running", m.phase)
	}
	for m.phase == phaseRunning {
		m.handleEvent(<-fa.stream)
	}
	out := joined(m)
	if !strings.Contains(out, "user> hi") || !strings.Contains(out, "hello world") {
		t.Fatalf("missing user/agent lines:\n%s", out)
	}
}

func TestConsole_ToolResultTruncation(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{
		event.NewToolResultStart("r1", 0, "t1", "search"),
		event.NewToolResultTextDelta("r1", 0, "t1", strings.Repeat("line\n", 30)),
		event.NewToolResultEnd("r1", 0, "t1"),
		event.NewReplyEnd("r1", "bot"),
	}}
	m := newTestModel(fa)
	m.startReply("go")
	for m.phase == phaseRunning {
		m.handleEvent(<-fa.stream)
	}
	out := joined(m)
	if !strings.Contains(out, "result (search)") || !strings.Contains(out, "(10 more lines)") {
		t.Fatalf("expected truncated tool result:\n%s", out)
	}
}

func TestConsole_ConfirmAllowFlow(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{
		event.NewRequireUserConfirm("r1", "c1", []event.ToolCallSummary{
			{ID: "t1", Name: "calculator", Input: map[string]any{"a": 1.0}},
		}),
	}}
	m := newTestModel(fa)
	m.startReply("run tool")
	m.handleEvent(<-fa.stream)
	if m.phase != phaseConfirming {
		t.Fatalf("phase = %v, want confirming", m.phase)
	}
	if !strings.Contains(joined(m), "allow 'calculator'?") {
		t.Fatalf("missing confirm prompt:\n%s", joined(m))
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.phase != phaseRunning {
		t.Fatalf("phase after allow = %v, want running", m.phase)
	}
	if len(fa.injected) != 1 {
		t.Fatalf("injected %d events, want 1", len(fa.injected))
	}
	res, ok := fa.injected[0].(*event.UserConfirmResultEvent)
	if !ok {
		t.Fatalf("injected %T, want UserConfirmResultEvent", fa.injected[0])
	}
	if res.ConfirmID != "c1" || len(res.Decisions) != 1 || res.Decisions[0].Decision != "allow" {
		t.Fatalf("unexpected decisions: %+v", res)
	}
}

func TestConsole_ConfirmCtrlCDeniesAll(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{
		event.NewRequireUserConfirm("r1", "c1", []event.ToolCallSummary{
			{ID: "t1", Name: "a"}, {ID: "t2", Name: "b"},
		}),
	}}
	m := newTestModel(fa)
	m.startReply("go")
	m.handleEvent(<-fa.stream)

	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if len(fa.injected) != 1 {
		t.Fatalf("injected %d events, want 1", len(fa.injected))
	}
	res := fa.injected[0].(*event.UserConfirmResultEvent)
	if len(res.Decisions) != 2 {
		t.Fatalf("decisions = %+v, want 2 denies", res.Decisions)
	}
	for _, d := range res.Decisions {
		if d.Decision != "deny" {
			t.Fatalf("decision %+v, want deny", d)
		}
	}
}

func TestConsole_MultiCallPerCallPrompts(t *testing.T) {
	fa := &fakeAgent{name: "bot", events: []event.AgentEvent{
		event.NewRequireUserConfirm("r1", "c1", []event.ToolCallSummary{
			{ID: "t1", Name: "a"}, {ID: "t2", Name: "b"},
		}),
	}}
	m := newTestModel(fa)
	m.startReply("go")
	m.handleEvent(<-fa.stream)

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.phase != phaseConfirming || len(fa.injected) != 0 {
		t.Fatalf("first answer should only advance the prompt (phase=%v injected=%d)", m.phase, len(fa.injected))
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseRunning || len(fa.injected) != 1 {
		t.Fatalf("second answer should inject (phase=%v injected=%d)", m.phase, len(fa.injected))
	}
	res := fa.injected[0].(*event.UserConfirmResultEvent)
	if res.Decisions[0].Decision != "allow" || res.Decisions[1].Decision != "deny" {
		t.Fatalf("decisions = %+v", res.Decisions)
	}
}

func TestConsole_StaleEventsIgnored(t *testing.T) {
	fa := &fakeAgent{name: "bot"}
	m := newTestModel(fa)
	m.handleEvent(event.NewTextBlockDelta("r1", 0, "late"))
	if len(m.lines) != 0 || m.partial.Len() != 0 {
		t.Fatalf("stale event must be ignored: lines=%v partial=%q", m.lines, m.partial.String())
	}
}

func TestConsole_IdleCtrlCQuits(t *testing.T) {
	m := newTestModel(&fakeAgent{name: "bot"})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.quit {
		t.Fatal("ctrl+c while idle should quit")
	}
}

func TestTruncateLines(t *testing.T) {
	if got := truncateLines("a\nb", 5); got != "a\nb" {
		t.Fatalf("no-op case: %q", got)
	}
	got := truncateLines("1\n2\n3\n4", 2)
	if !strings.HasPrefix(got, "1\n2") || !strings.Contains(got, "(2 more lines)") {
		t.Fatalf("truncate case: %q", got)
	}
	if got := truncateLines("x", -1); got != "x" {
		t.Fatalf("unlimited case: %q", got)
	}
}
