// realtime/event.go — the session event vocabulary (19.1). Events are
// plain structs; consumers type-switch. The stream ordering contract:
// SessionStarted first; AudioOutDelta/TranscriptDelta interleaved as
// produced; ToolCall before its Result; a terminal event
// (ResponseDone{Final:true} / SessionClosed / Error) last, after which the
// channel closes.
package realtime

import "time"

// Event is the sealed base of every realtime session event.
type Event interface{ isRealtimeEvent() }

// SessionStarted opens the session (always the first event).
type SessionStarted struct {
	SessionID string
	Format    AudioFormat // the negotiated session format
	At        time.Time
}

// InputTranscriptDelta is an incremental speech-recognition chunk for the
// user's in-flight utterance (ASR partials).
type InputTranscriptDelta struct{ Text string }

// UserTranscribed finalises one user utterance (barge-in detection point:
// a complete utterance while the assistant speaks is an interrupt trigger).
type UserTranscribed struct{ Text string }

// ResponseStarted marks the model beginning a response turn.
type ResponseStarted struct{ ResponseID string }

// AudioOutDelta is one chunk of assistant audio in the negotiated OUTPUT
// format. Sequence is the monotonic chunk index within the response
// (order/loss detection, 19.2 backpressure tests).
type AudioOutDelta struct {
	Sequence int
	Data     []byte
}

// TranscriptDelta is an incremental text chunk of the assistant's reply
// (captions alongside audio).
type TranscriptDelta struct{ Text string }

// ToolCall requests a tool execution mid-response.
type ToolCall struct {
	ID   string
	Name string
	Args []byte // raw JSON arguments
}

// ToolResult carries the tool outcome back into the conversation (emitted
// by the agent tier, echoed here for transcript completeness).
type ToolResult struct {
	ID     string
	Output []byte
	Err    string
}

// Interrupted confirms a barge-in: generation stopped and queued playback
// was cut at the server's truncation point. For client-truncation cards
// the position arrives via the playout layer (19.2) instead.
type Interrupted struct{ At time.Time }

// ResponseDone closes one response turn. Final=true means the turn
// completed fully; Final=false means it was cut (interrupted / cancelled).
type ResponseDone struct {
	ResponseID string
	Final      bool
}

// ErrorEvent is a terminal, recoverable-per-session failure. The stream
// closes after it.
type ErrorEvent struct{ Err string }

// SessionClosed ends the session cleanly (after this the channel closes).
type SessionClosed struct{}

func (SessionStarted) isRealtimeEvent()       {}
func (InputTranscriptDelta) isRealtimeEvent() {}
func (UserTranscribed) isRealtimeEvent()      {}
func (ResponseStarted) isRealtimeEvent()      {}
func (AudioOutDelta) isRealtimeEvent()        {}
func (TranscriptDelta) isRealtimeEvent()      {}
func (ToolCall) isRealtimeEvent()             {}
func (ToolResult) isRealtimeEvent()           {}
func (Interrupted) isRealtimeEvent()          {}
func (ResponseDone) isRealtimeEvent()         {}
func (ErrorEvent) isRealtimeEvent()           {}
func (SessionClosed) isRealtimeEvent()        {}

// Terminal reports whether e ends the event stream.
func Terminal(e Event) bool {
	switch e.(type) {
	case ResponseDone:
		return e.(ResponseDone).Final
	case SessionClosed, ErrorEvent:
		return true
	}
	return false
}
