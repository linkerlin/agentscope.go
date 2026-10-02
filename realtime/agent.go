// realtime/agent.go — RealtimeAgent (19.3): the orchestrator that drives a
// realtime voice Model (19.1) over the playout/VAD pipeline (19.2) with
// chunked tool execution. Acceptance clauses:
//
//   - barge-in keeps only the HEARD prefix: the interrupt cut happens at
//     the playout-confirmed position (client cards) or the server's
//     Interrupted event (server cards) — never "drop everything";
//   - reconnect loses no backlog and replays nothing twice: per-response
//     sequence de-dup plus a completed-response set keep the stream exact
//     across re-dials;
//   - a tool confirmation can time out and the conversation RESUMES
//     (the session never dead-ends waiting for a human);
//   - no bespoke multi-replica protocol: the agent owns no cross-replica
//     state; turn-level coordination stays with the session coordinator
//     (18.5) wrapping the caller's Run.
package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ToolHandler executes one realtime tool call. Returning is the resume
// point: block until the outcome is known (a confirmation, a result), and
// the agent enforces AgentConfig.ToolConfirmTimeout so a stalled handler
// cannot dead-end the session.
type ToolHandler interface {
	Handle(ctx context.Context, call ToolCall) (output []byte, err error)
}

// ToolHandlerFunc adapts a function to ToolHandler.
type ToolHandlerFunc func(ctx context.Context, call ToolCall) ([]byte, error)

// Handle implements ToolHandler.
func (f ToolHandlerFunc) Handle(ctx context.Context, call ToolCall) ([]byte, error) {
	return f(ctx, call)
}

// AgentConfig tunes RealtimeAgent.
type AgentConfig struct {
	// Offer is the client's audio-format preference order.
	Offer NegotiateOffer
	// OnBargeIn fires after the interrupt cut (observability/UI).
	OnBargeIn func(pos PlayoutPosition)
	// ToolConfirmTimeout bounds one tool call including any human
	// confirmation wait; 0 defaults to 30s. On timeout the agent resumes
	// with a ToolResult carrying the timeout error.
	ToolConfirmTimeout time.Duration
	// VAD is optional input speech detection; when set, a SpeechStart edge
	// while the assistant speaks triggers the barge-in (text-only sessions
	// rely on UserTranscribed instead).
	VAD *VAD
	// OnTurnMetrics fires when one response turn completes (ResponseDone)
	// with the collected TurnMetrics (19.4). Optional observability hook.
	OnTurnMetrics func(TurnMetrics)
}

// RealtimeAgent orchestrates one voice conversation. Build per session;
// AudioOut is the play stream (raw confirmed-ordered chunks — feed them to
// your device and call Advance/Position on the shared Playout).
type RealtimeAgent struct {
	model Model
	cfg   AgentConfig

	mu      sync.Mutex
	session Session
	playout *Playout
	format  AudioFormat
	tools   ToolHandler
	// seenSeq de-dupes audio within one response (reconnect backlog).
	seenSeq map[string]int
	// doneResponses marks responses already finalised (their late deltas
	// are backlog noise).
	doneResponses map[string]bool
	// metrics collects one record per completed turn (Metrics snapshot).
	metrics []TurnMetrics
	closed  bool
}

// NewRealtimeAgent builds an agent over a realtime model.
func NewRealtimeAgent(m Model, cfg AgentConfig) *RealtimeAgent {
	if cfg.ToolConfirmTimeout <= 0 {
		cfg.ToolConfirmTimeout = 30 * time.Second
	}
	return &RealtimeAgent{
		model:         m,
		cfg:           cfg,
		playout:       &Playout{},
		seenSeq:       map[string]int{},
		doneResponses: map[string]bool{},
	}
}

// Playout exposes the shared confirmed-position clock (the caller's audio
// device advances it; barge-in truncation reads it).
func (a *RealtimeAgent) Playout() *Playout { return a.playout }

// SendAudio forwards one input audio chunk to the live session (the console
// tier, 19.7, is the reference consumer; no-op error before Connect).
func (a *RealtimeAgent) SendAudio(ctx context.Context, chunk []byte) error {
	a.mu.Lock()
	sess := a.session
	a.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("realtime: send audio: not connected")
	}
	return sess.SendAudio(ctx, chunk)
}

// SendText injects a text input turn on the live session.
func (a *RealtimeAgent) SendText(ctx context.Context, text string) error {
	a.mu.Lock()
	sess := a.session
	a.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("realtime: send text: not connected")
	}
	return sess.SendText(ctx, text)
}

// BargeIn triggers a manual interrupt cut (the voice "stop talking" action).
// Equivalent to the UserTranscribed path: every truncation mode stops
// generation; client cards additionally cut the local playout at the
// confirmed (heard) position.
func (a *RealtimeAgent) BargeIn() {
	a.mu.Lock()
	sess := a.session
	a.mu.Unlock()
	if sess == nil {
		return
	}
	a.bargeIn(context.Background(), sess, a.model.Card().Truncation.Normalized())
}

// Format reports the negotiated session audio format (after Connect).
func (a *RealtimeAgent) Format() AudioFormat { return a.format }

// Metrics returns the per-turn metrics snapshot of the conversation so far
// (one record per completed response turn, in completion order).
func (a *RealtimeAgent) Metrics() []TurnMetrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]TurnMetrics(nil), a.metrics...)
}

// Connect opens the session under the configured offer and returns the
// event pump. The pump owns session-event consumption: audio flows to the
// playout queue (de-duplicated), barge-ins cut at the confirmed position,
// tool calls dispatch to the handler with the confirm timeout, and every
// event (including the agent-synthesised ones) is forwarded to the caller.
func (a *RealtimeAgent) Connect(ctx context.Context) (<-chan Event, error) {
	sess, answer, err := a.model.Connect(ctx, a.cfg.Offer)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.session = sess
	a.format = answer.Format
	a.mu.Unlock()

	out := make(chan Event, 64)
	go a.pump(ctx, sess, out)
	return out, nil
}

// pump consumes the session's events, applies the agent semantics and
// forwards to out. It exits when the session stream ends (clean, error or
// dropped) or ctx is done; synthesized events flow through the same
// channel so ordering is total.
func (a *RealtimeAgent) pump(ctx context.Context, sess Session, out chan<- Event) {
	defer close(out)
	var currentResponse string
	var speaking bool
	var turn *TurnMetrics // metrics of the in-flight response (nil between turns)
	trunc := a.model.Card().Truncation.Normalized()

	for {
		var ev Event
		var ok bool
		select {
		case <-ctx.Done():
			return
		case ev, ok = <-sess.Events():
			if !ok {
				return
			}
		}
		if Terminal(ev) {
			// Session-level terminal (SessionClosed / ErrorEvent): forward
			// and end the pump.
			out <- ev
			return
		}
		switch e := ev.(type) {
		case ResponseStarted:
			currentResponse = e.ResponseID
			speaking = true
			a.mu.Lock()
			// A re-dial may replay the ResponseStarted of an in-flight
			// response: the de-dup state must SURVIVE it, or the replayed
			// audio floods the playout a second time.
			if _, seen := a.seenSeq[e.ResponseID]; !seen {
				a.seenSeq[e.ResponseID] = -1
				turn = &TurnMetrics{ResponseID: e.ResponseID, StartedAt: time.Now()}
			}
			a.mu.Unlock()

		case ResponseDone:
			// Turn boundary: late deltas for this response are backlog
			// noise; the conversation continues.
			a.mu.Lock()
			a.doneResponses[e.ResponseID] = true
			a.mu.Unlock()
			speaking = false
			if t := turn; t != nil {
				t.DoneAt = time.Now()
				t.Final = e.Final
				turn = nil
				a.mu.Lock()
				a.metrics = append(a.metrics, *t)
				a.mu.Unlock()
				if a.cfg.OnTurnMetrics != nil {
					a.cfg.OnTurnMetrics(*t)
				}
			}

		case TranscriptDelta:
			if turn != nil && turn.FirstTextAt.IsZero() {
				turn.FirstTextAt = time.Now()
			}

		case AudioOutDelta:
			a.mu.Lock()
			done := a.doneResponses[currentResponse]
			last := a.seenSeq[currentResponse]
			if !done && e.Sequence > last {
				a.seenSeq[currentResponse] = e.Sequence
			}
			a.mu.Unlock()
			if done || e.Sequence <= last {
				// Backlog replay (already-seen sequence) or a finished
				// response's late delta: drop — reconnect must not double.
				continue
			}
			a.playout.Enqueue(e.Sequence, e.Data)
			if turn != nil {
				turn.AudioChunks++
				turn.AudioBytes += len(e.Data)
				if turn.FirstAudioAt.IsZero() {
					turn.FirstAudioAt = time.Now()
				}
			}

		case UserTranscribed:
			// A complete user utterance while the assistant speaks is the
			// barge-in trigger.
			if speaking {
				a.bargeIn(ctx, sess, trunc)
			}

		case ToolCall:
			// The call itself is forwarded (vocabulary completeness); the
			// handler's outcome follows as a synthesized ToolResult.
			if turn != nil {
				turn.ToolCalls++
			}
			out <- ev
			if a.tools != nil {
				out <- a.runTool(ctx, e)
			}
			continue
		}
		out <- ev
	}
}

// bargeIn executes the interrupt cut: client cards cut at the confirmed
// playout position (the heard prefix stays, the unconfirmed tail drops);
// server cards delegate the cut; none cards only stop future generation.
func (a *RealtimeAgent) bargeIn(ctx context.Context, sess Session, trunc TruncationSupport) {
	_ = sess.Interrupt(ctx) // every mode stops further generation
	var pos PlayoutPosition
	switch trunc {
	case TruncationClient:
		a.playout.Truncate()
		pos = a.playout.Position()
	case TruncationServer:
		// The Interrupted event carries the server's cut; nothing local.
	case TruncationNone:
		// Generation stops; queued audio plays out (card declared).
	}
	if a.cfg.OnBargeIn != nil && trunc == TruncationClient {
		a.cfg.OnBargeIn(pos)
	}
}

// runTool executes one tool call under the confirm timeout. A stalled or
// declined-with-timeout handler resumes the conversation with an error
// ToolResult — the acceptance's "工具确认可超时恢复".
func (a *RealtimeAgent) runTool(parent context.Context, call ToolCall) Event {
	ctx, cancel := context.WithTimeout(parent, a.cfg.ToolConfirmTimeout)
	defer cancel()
	var out []byte
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, err = a.tools.Handle(ctx, call)
	}()
	select {
	case <-done:
		if err != nil {
			return ToolResult{ID: call.ID, Err: err.Error()}
		}
		return ToolResult{ID: call.ID, Output: out}
	case <-ctx.Done():
		return ToolResult{ID: call.ID, Err: fmt.Sprintf("tool %s: confirm timeout after %s", call.Name, a.cfg.ToolConfirmTimeout)}
	}
}

// Reconnect re-dials after a dropped stream. Already-delivered audio is
// de-duplicated by (response, sequence) via the shared backlog state; the
// returned event stream continues the conversation. The old session is
// closed best-effort (a dead socket cannot be told anything).
func (a *RealtimeAgent) Reconnect(ctx context.Context) (<-chan Event, error) {
	a.mu.Lock()
	old := a.session
	a.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return a.Connect(ctx)
}

// Close ends the conversation (idempotent).
func (a *RealtimeAgent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.session == nil {
		a.closed = true
		return nil
	}
	a.closed = true
	return a.session.Close()
}

// WithTools installs the tool handler for session tool calls.
func (a *RealtimeAgent) WithTools(h ToolHandler) *RealtimeAgent {
	a.tools = h
	return a
}
