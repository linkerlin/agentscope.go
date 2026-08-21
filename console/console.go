// Package console provides an interactive bubbletea TUI for trying out and
// debugging agents, aligned with Python agentscope.console: multi-turn chat,
// live rendering of streamed V2 events, human-in-the-loop tool confirmation
// (y/n/a) and Ctrl+C mid-turn interruption.
package console

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// Verbosity controls which stream events are rendered.
type Verbosity int

const (
	// Quiet renders only reply text and errors.
	Quiet Verbosity = iota
	// Default additionally renders thinking, tool calls/results, tokens and HITL notices.
	Default
	// Debug additionally renders lifecycle markers and raw event types.
	Debug
)

// Options configures the console.
type Options struct {
	// UserName labels user input lines (default "user").
	UserName string
	// Verbosity selects event rendering detail (default Default when zero).
	Verbosity Verbosity
	// MaxToolResultLines caps rendered tool-result lines (default 20, negative = unlimited).
	MaxToolResultLines int
}

// Launch runs an interactive TUI chat with ag until the user quits
// ("exit"/"quit", Ctrl+D on empty input, or Ctrl+C while idle).
// Ctrl+C during a running reply interrupts the agent mid-turn.
func Launch(ctx context.Context, ag agent.V2Agent, opts Options) error {
	if opts.UserName == "" {
		opts.UserName = "user"
	}
	if opts.MaxToolResultLines == 0 {
		opts.MaxToolResultLines = 20
	}
	p := tea.NewProgram(newModel(ctx, ag, opts), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

type phase int

const (
	phaseIdle phase = iota
	phaseRunning
	phaseConfirming
)

// eventMsg carries one agent event from the reply stream into the TUI loop.
type eventMsg struct{ ev event.AgentEvent }

// streamClosedMsg signals that the reply stream channel closed.
type streamClosedMsg struct{}

func waitForEvent(ch <-chan event.AgentEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return eventMsg{ev}
	}
}

// interruptor is implemented by agents supporting mid-turn interruption
// (e.g. react.ReActAgent via agent.Base).
type interruptor interface{ Interrupt() }

// Model is the bubbletea model driving the console.
type Model struct {
	ctx  context.Context
	ag   agent.V2Agent
	opts Options

	phase phase
	quit  bool

	lines    []string        // committed conversation lines
	partial  strings.Builder // streaming reply text of the current text block
	thinkBuf strings.Builder
	hintBuf  strings.Builder
	toolBuf  strings.Builder
	toolName string

	evCh <-chan event.AgentEvent

	pending    *event.RequireUserConfirmEvent
	confirmIdx int
	decisions  []event.ConfirmDecision

	input textarea.Model
	vp    viewport.Model
	spin  spinner.Model

	width, height int
}

func newModel(ctx context.Context, ag agent.V2Agent, opts Options) *Model {
	ta := textarea.New()
	ta.Prompt = "› "
	ta.MaxHeight = 3
	ta.Focus()
	return &Model{
		ctx:   ctx,
		ag:    ag,
		opts:  opts,
		input: ta,
		vp:    viewport.New(80, 20),
		spin:  spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return textarea.Blink }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.refreshView()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if m.phase == phaseRunning {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case eventMsg:
		return m.handleEvent(msg.ev)

	case streamClosedMsg:
		if m.phase == phaseRunning {
			m.flushPartial()
			m.phase = phaseIdle
			m.refreshView()
		}
		return m, nil
	}

	if m.phase == phaseIdle {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refreshView()
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.quit {
		return "bye\n"
	}
	if m.width == 0 {
		return "starting…"
	}
	header := styleHeader(fmt.Sprintf("%s · console · verbosity %s", m.ag.Name(), verbosityName(m.opts.Verbosity)))

	var status string
	switch m.phase {
	case phaseRunning:
		status = m.spin.View() + " thinking…"
	case phaseConfirming:
		status = "waiting for confirmation (y/n/a)"
	default:
		status = "enter send · alt+enter newline · exit/quit or ctrl+d leave · ctrl+c interrupt"
	}

	var b strings.Builder
	b.WriteString(header + "\n")
	b.WriteString(m.vp.View() + "\n")
	b.WriteString(styleDim(status) + "\n")
	if m.phase != phaseConfirming {
		b.WriteString(m.input.View())
	}
	return b.String()
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseIdle:
		switch {
		case msg.Type == tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		case msg.Type == tea.KeyCtrlD && m.input.Value() == "":
			m.quit = true
			return m, tea.Quit
		case msg.Type == tea.KeyEnter && !msg.Alt:
			text := strings.TrimSpace(m.input.Value())
			m.input.Reset()
			if text == "" {
				return m, nil
			}
			if text == "exit" || text == "quit" {
				m.quit = true
				return m, tea.Quit
			}
			return m, m.startReply(text)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refreshView()
		return m, cmd

	case phaseRunning:
		if msg.Type == tea.KeyCtrlC {
			if i, ok := m.ag.(interruptor); ok {
				m.appendLine(styleConfirm("interrupting…"))
				i.Interrupt()
			} else {
				m.appendLine(styleError("this agent does not support interruption"))
			}
			m.refreshView()
		}
		return m, nil

	case phaseConfirming:
		return m.handleConfirmKey(msg)
	}
	return m, nil
}

func (m *Model) startReply(text string) tea.Cmd {
	m.appendLine(styleUser(m.opts.UserName, text))
	msg := message.NewMsg().Role(message.RoleUser).TextContent(text).Build()
	ch, err := m.ag.ReplyStream(m.ctx, msg)
	if err != nil {
		m.appendLine(styleError(err.Error()))
		m.refreshView()
		return nil
	}
	m.evCh = ch
	m.phase = phaseRunning
	m.refreshView()
	return tea.Batch(waitForEvent(ch), m.spin.Tick)
}

func (m *Model) handleEvent(ev event.AgentEvent) (tea.Model, tea.Cmd) {
	if m.phase == phaseIdle {
		return m, nil // stale event from a previous turn
	}

	if c, ok := ev.(*event.RequireUserConfirmEvent); ok {
		m.flushPartial()
		m.pending = c
		m.confirmIdx = 0
		m.decisions = nil
		m.phase = phaseConfirming
		m.renderConfirmRequest(c)
		m.refreshView()
		return m, nil
	}

	m.renderEvent(ev)

	switch ev.(type) {
	case *event.ReplyEndEvent, *event.ErrorEvent, *event.InterruptEvent, *event.ExceedMaxItersEvent:
		m.flushPartial()
		m.phase = phaseIdle
		m.refreshView()
		return m, nil
	}

	m.refreshView()
	return m, waitForEvent(m.evCh)
}

// renderEvent appends rendered lines for ev according to verbosity and
// accumulates streaming block buffers. Phase transitions are handled by the
// caller, not here.
func (m *Model) renderEvent(ev event.AgentEvent) {
	v := m.opts.Verbosity
	switch e := ev.(type) {
	case *event.TextBlockDeltaEvent:
		m.partial.WriteString(e.Delta)
	case *event.TextBlockEndEvent:
		m.flushPartial()
	case *event.ThinkingBlockDeltaEvent:
		if v >= Default {
			m.thinkBuf.WriteString(e.Delta)
		}
	case *event.ThinkingBlockEndEvent:
		if v >= Default && m.thinkBuf.Len() > 0 {
			m.appendLine(styleThinking(m.thinkBuf.String()))
			m.thinkBuf.Reset()
		}
	case *event.HintBlockDeltaEvent:
		if v >= Default {
			m.hintBuf.WriteString(e.Delta)
		}
	case *event.HintBlockEndEvent:
		if v >= Default && m.hintBuf.Len() > 0 {
			m.appendLine(styleHint(m.hintBuf.String()))
			m.hintBuf.Reset()
		}
	case *event.DataBlockStartEvent:
		if v >= Default {
			m.appendLine(styleToolLine(fmt.Sprintf("· data block (%s)", e.MediaType)))
		}
	case *event.ToolCallStartEvent:
		if v >= Default {
			m.appendLine(styleToolLine("· tool call: " + e.ToolName))
		}
	case *event.ToolResultStartEvent:
		m.toolName = e.ToolName
		m.toolBuf.Reset()
	case *event.ToolResultTextDeltaEvent:
		m.toolBuf.WriteString(e.Delta)
	case *event.ToolResultEndEvent:
		if v >= Default && m.toolBuf.Len() > 0 {
			m.appendLine(styleToolResult(m.toolName, truncateLines(m.toolBuf.String(), m.opts.MaxToolResultLines)))
		}
		m.toolBuf.Reset()
	case *event.ModelCallEndEvent:
		if v >= Default {
			m.appendLine(styleDim(fmt.Sprintf("· model %s · %d→%d tokens", e.ModelName, e.InputTokens, e.OutputTokens)))
		}
	case *event.RequireExternalExecutionEvent:
		m.appendLine(styleError("external execution required (not supported in console)"))
	case *event.ExceedMaxItersEvent:
		m.appendLine(styleError(fmt.Sprintf("reached max iterations (%d) without a final answer", e.MaxIters)))
	case *event.ErrorEvent:
		m.appendLine(styleError("error: " + e.Err))
	case *event.InterruptEvent:
		m.appendLine(styleConfirm("interrupted (" + e.Source + ")"))
	case *event.ReplyStartEvent:
		if v >= Debug {
			m.appendLine(styleDim("── reply start (" + e.AgentName + ") ──"))
		}
	case *event.ReplyEndEvent:
		if v >= Debug {
			m.appendLine(styleDim("── reply end ──"))
		}
	case *event.ModelCallStartEvent:
		if v >= Debug {
			m.appendLine(styleDim("── model call: " + e.ModelName + " ──"))
		}
	default:
		if v >= Debug {
			m.appendLine(styleDim("· event: " + ev.EventType()))
		}
	}
}

func (m *Model) renderConfirmRequest(c *event.RequireUserConfirmEvent) {
	m.appendLine(styleConfirm("human confirmation required:"))
	for _, tc := range c.ToolCalls {
		m.appendLine(styleConfirm(fmt.Sprintf("  - %s %s", tc.Name, compactInput(tc.Input))))
	}
	m.appendLine(m.confirmPrompt())
}

func (m *Model) confirmPrompt() string {
	call := m.pending.ToolCalls[m.confirmIdx]
	return styleConfirm(fmt.Sprintf("allow '%s'? [y]es / [N]o / [a]lways", call.Name))
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		// Deny all remaining calls and resume the agent.
		for i := m.confirmIdx; i < len(m.pending.ToolCalls); i++ {
			m.decisions = append(m.decisions, event.ConfirmDecision{
				ToolCallID: m.pending.ToolCalls[i].ID,
				Decision:   "deny",
			})
		}
		return m, m.injectDecisions()
	}

	var decision string
	switch {
	case keyIs(msg, "y"):
		decision = "allow"
	case keyIs(msg, "a"):
		decision = "always_allow"
	case keyIs(msg, "n"), msg.Type == tea.KeyEnter:
		decision = "deny"
	default:
		return m, nil
	}
	call := m.pending.ToolCalls[m.confirmIdx]
	m.decisions = append(m.decisions, event.ConfirmDecision{ToolCallID: call.ID, Decision: decision})
	m.confirmIdx++
	if m.confirmIdx < len(m.pending.ToolCalls) {
		m.appendLine(m.confirmPrompt())
		m.refreshView()
		return m, nil
	}
	return m, m.injectDecisions()
}

func keyIs(msg tea.KeyMsg, s string) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && strings.EqualFold(string(msg.Runes[0]), s)
}

func (m *Model) injectDecisions() tea.Cmd {
	ev := event.NewUserConfirmResult(m.pending.ReplyID(), m.pending.ConfirmID, m.decisions)
	if err := m.ag.InjectEvent(m.ctx, ev); err != nil {
		m.appendLine(styleError("resume failed: " + err.Error()))
		m.pending = nil
		m.phase = phaseIdle
		m.refreshView()
		return nil
	}
	m.pending = nil
	m.phase = phaseRunning
	m.refreshView()
	return waitForEvent(m.evCh)
}

func (m *Model) flushPartial() {
	if m.partial.Len() == 0 {
		return
	}
	m.appendLine(styleAgent(m.partial.String()))
	m.partial.Reset()
}

func (m *Model) appendLine(s string) { m.lines = append(m.lines, s) }

func (m *Model) layout() {
	const inputHeight = 3
	avail := m.height - 1 - 1 - inputHeight - 2 // header, status, input, margins
	if avail < 3 {
		avail = 3
	}
	m.vp.Width = m.width
	m.vp.Height = avail
	m.input.SetWidth(m.width - 2)
}

func (m *Model) refreshView() {
	content := strings.Join(m.lines, "\n")
	if m.partial.Len() > 0 {
		if content != "" {
			content += "\n"
		}
		content += styleAgent(m.partial.String())
	}
	m.vp.SetContent(content)
	m.vp.GotoBottom()
}

func compactInput(in map[string]any) string {
	if len(in) == 0 {
		return "{}"
	}
	b, err := json.Marshal(in)
	if err != nil {
		return fmt.Sprint(in)
	}
	s := string(b)
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}
