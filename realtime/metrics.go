// realtime/metrics.go — TurnMetrics (19.4): per-turn observability for voice
// sessions. The agent tier collects one metrics record per assistant
// response turn (ResponseStarted → ResponseDone) on the already-ordered
// event stream, so backends need no metrics plumbing of their own.
package realtime

import "time"

// TurnMetrics summarises one assistant response turn. Zero timestamps mean
// "never happened" (e.g. a text-only turn has a zero FirstAudioAt).
type TurnMetrics struct {
	ResponseID string
	// StartedAt is the ResponseStarted arrival time.
	StartedAt time.Time
	// FirstTextAt is the first TranscriptDelta arrival time (captions).
	FirstTextAt time.Time
	// FirstAudioAt is the first (de-duplicated) AudioOutDelta arrival time.
	FirstAudioAt time.Time
	// DoneAt is the ResponseDone arrival time.
	DoneAt time.Time
	// Final mirrors ResponseDone.Final (false = interrupted turn).
	Final bool
	// AudioChunks / AudioBytes count de-duplicated audio deltas actually
	// enqueued for playback (reconnect backlog replays are not counted).
	AudioChunks int
	AudioBytes  int
	// ToolCalls counts tool calls issued during the turn.
	ToolCalls int
}

// TimeToFirstAudio is FirstAudioAt-StartedAt; 0 when the turn produced no
// audio. THE voice-latency number (time-to-first-audio).
func (m TurnMetrics) TimeToFirstAudio() time.Duration {
	if m.FirstAudioAt.IsZero() || m.StartedAt.IsZero() {
		return 0
	}
	return m.FirstAudioAt.Sub(m.StartedAt)
}

// TimeToFirstText is FirstTextAt-StartedAt; 0 when the turn produced no
// transcript.
func (m TurnMetrics) TimeToFirstText() time.Duration {
	if m.FirstTextAt.IsZero() || m.StartedAt.IsZero() {
		return 0
	}
	return m.FirstTextAt.Sub(m.StartedAt)
}

// TurnDuration is DoneAt-StartedAt; 0 for an unfinished turn.
func (m TurnMetrics) TurnDuration() time.Duration {
	if m.DoneAt.IsZero() || m.StartedAt.IsZero() {
		return 0
	}
	return m.DoneAt.Sub(m.StartedAt)
}
