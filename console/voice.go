// console/voice.go — the voice entry point for the console tier (19.7):
// VoiceSession bridges a realtime.RealtimeAgent to console interaction.
// The controller owns the interaction policy — text/voice input switching,
// barge-in triggers (manual + VAD speech-start edge) and the tool
// confirmation (HITL) bridge — while actual device I/O hides behind two
// narrow interfaces: automated tests drive fakes (a scripted MockModel is
// the fake transport, per the acceptance), and the manual smoke example
// wires real capture/playback processes.
package console

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/realtime"
)

// Input modes (VoiceSession.SetMode).
const (
	VoiceModeText  = "text"
	VoiceModeVoice = "voice"
)

// MicSource captures microphone audio (manual smoke wires a real capture
// process; tests fake it). Start begins capture; emit delivers raw PCM
// chunks in the negotiated INPUT format from the capturer's goroutine until
// Stop returns.
type MicSource interface {
	Start(ctx context.Context, emit func(pcm []byte)) error
	Stop() error
}

// AudioSink plays assistant audio chunks. The sink reports consumed bytes
// back on the session's shared playout (realtime.RealtimeAgent.Playout()
// Advance) so a barge-in cuts at the actually-heard position; the fake sink
// in tests advances deterministically.
type AudioSink interface {
	Play(seq int, chunk []byte)
}

// VoiceOptions configures a VoiceSession. Every callback is optional.
type VoiceOptions struct {
	Model realtime.Model
	Offer realtime.NegotiateOffer

	// Mic enables voice mode (nil: text-only session, SetMode("voice")
	// refuses).
	Mic MicSource
	// Sink plays assistant audio (nil: audio discarded, captions only).
	Sink AudioSink
	// VAD is optional: a speech-start edge from the mic while the assistant
	// speaks triggers the barge-in (the natural voice interrupt).
	VAD *realtime.VAD

	OnText        func(delta string) // assistant transcript delta
	OnUser        func(text string)  // one completed user utterance
	OnToolResult  func(r realtime.ToolResult)
	OnTurnMetrics func(realtime.TurnMetrics)
	OnError       func(msg string)
	OnClosed      func()

	// ConfirmTool is the HITL bridge: one realtime ToolCall, answered by the
	// human decision. approve=false delivers a refusal ToolResult back into
	// the conversation. When nil, tool calls pass through unconfirmed
	// (automated deployments).
	ConfirmTool func(call realtime.ToolCall) (approve bool, output []byte)
}

// errToolDeclined marks a declined tool call (surfaces as ToolResult.Err).
var errToolDeclined = errors.New("tool call declined by user")

// VoiceSession is one live voice conversation for a console front end.
type VoiceSession struct {
	ctx   context.Context
	opts  VoiceOptions
	agent *realtime.RealtimeAgent

	mu         sync.Mutex
	mode       string // "text" | "voice"
	speaking   bool
	micRunning bool
	closed     bool
}

// StartVoiceSession connects the agent and starts the event pump.
func StartVoiceSession(ctx context.Context, opts VoiceOptions) (*VoiceSession, error) {
	if opts.Model == nil {
		return nil, errors.New("console voice: model is required")
	}
	agent := realtime.NewRealtimeAgent(opts.Model, realtime.AgentConfig{
		Offer:         opts.Offer,
		OnTurnMetrics: opts.OnTurnMetrics,
	})
	if opts.ConfirmTool != nil {
		confirm := opts.ConfirmTool
		agent = agent.WithTools(realtime.ToolHandlerFunc(func(ctx context.Context, call realtime.ToolCall) ([]byte, error) {
			approve, out := confirm(call)
			if !approve {
				return nil, errToolDeclined
			}
			return out, nil
		}))
	}
	out, err := agent.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("console voice: connect: %w", err)
	}
	v := &VoiceSession{ctx: ctx, opts: opts, agent: agent, mode: VoiceModeText}
	go v.pump(out)
	return v, nil
}

// pump consumes the agent stream and fans out to the sinks/callbacks. It
// ends on a terminal event or stream close.
func (v *VoiceSession) pump(out <-chan realtime.Event) {
	for ev := range out {
		switch e := ev.(type) {
		case realtime.ResponseStarted:
			v.setSpeaking(true)
		case realtime.ResponseDone:
			v.setSpeaking(false)
		case realtime.TranscriptDelta:
			if v.opts.OnText != nil {
				v.opts.OnText(e.Text)
			}
		case realtime.UserTranscribed:
			if v.opts.OnUser != nil {
				v.opts.OnUser(e.Text)
			}
		case realtime.AudioOutDelta:
			if v.opts.Sink != nil {
				v.opts.Sink.Play(e.Sequence, e.Data)
			}
		case realtime.ToolResult:
			if v.opts.OnToolResult != nil {
				v.opts.OnToolResult(e)
			}
		case realtime.ErrorEvent:
			if v.opts.OnError != nil {
				v.opts.OnError(e.Err)
			}
		}
		if realtime.Terminal(ev) {
			if v.opts.OnClosed != nil {
				v.opts.OnClosed()
			}
			return
		}
	}
	if v.opts.OnClosed != nil {
		v.opts.OnClosed()
	}
}

// Mode reports the active input mode ("text" or "voice").
func (v *VoiceSession) Mode() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.mode
}

// SetMode switches the input mode. Switching to voice starts the mic (and
// refuses when no mic source was configured); switching to text stops it —
// text input (SubmitText) stays available in BOTH modes (the acceptance's
// 文本切换: voice users may still type).
func (v *VoiceSession) SetMode(mode string) error {
	if mode != VoiceModeText && mode != VoiceModeVoice {
		return fmt.Errorf("console voice: unknown mode %q", mode)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return errors.New("console voice: session closed")
	}
	if mode == v.mode {
		return nil
	}
	if mode == VoiceModeVoice {
		if v.opts.Mic == nil {
			return errors.New("console voice: no mic source configured")
		}
		if err := v.opts.Mic.Start(v.ctx, v.micEmit); err != nil {
			return fmt.Errorf("console voice: start mic: %w", err)
		}
		v.micRunning = true
	} else if v.micRunning {
		_ = v.opts.Mic.Stop()
		v.micRunning = false
	}
	v.mode = mode
	return nil
}

// SubmitText sends one text input turn (both modes).
func (v *VoiceSession) SubmitText(text string) error {
	return v.agent.SendText(v.ctx, text)
}

// BargeIn manually interrupts the assistant (the voice "stop talking"
// action; the VAD path triggers it automatically on speech-start).
func (v *VoiceSession) BargeIn() {
	v.agent.BargeIn()
}

// Playout exposes the shared confirmed-position clock for the sink to
// advance.
func (v *VoiceSession) Playout() *realtime.Playout { return v.agent.Playout() }

// Close stops the mic and ends the session (idempotent).
func (v *VoiceSession) Close() error {
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	micRunning := v.micRunning
	v.micRunning = false
	v.mu.Unlock()
	if micRunning && v.opts.Mic != nil {
		_ = v.opts.Mic.Stop()
	}
	return v.agent.Close()
}

// micEmit is the capturer callback: feed the VAD (barge-in detection) and
// forward the chunk to the session.
func (v *VoiceSession) micEmit(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	if v.opts.VAD != nil {
		if v.opts.VAD.Process(bytesToInt16(pcm)) == realtime.VADSpeechStart && v.isSpeaking() {
			v.agent.BargeIn()
		}
	}
	_ = v.agent.SendAudio(v.ctx, pcm)
}

func (v *VoiceSession) setSpeaking(b bool) {
	v.mu.Lock()
	v.speaking = b
	v.mu.Unlock()
}

func (v *VoiceSession) isSpeaking() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.speaking
}

// bytesToInt16 reinterprets little-endian PCM bytes as samples (the VAD
// frame format).
func bytesToInt16(b []byte) []int16 {
	n := len(b) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2 : i*2+2]))
	}
	return out
}
