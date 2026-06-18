// Package tts provides text-to-speech model abstractions for AgentScope.go.
//
// It mirrors Python agentscope's tts/ module (#1832): a focused TTS-only
// abstraction (separate from the chat AudioModel), supporting both
// non-realtime (Synthesize) and realtime streaming-input (Connect/Push/Close)
// models, plus YAML model cards and an agent-level TTSMiddleware.
package tts

import "context"

// Options controls speech synthesis.
type Options struct {
	// Voice selects the speaker voice (provider-specific, e.g. "alloy",
	// "longxiaochun").
	Voice string
	// Format is the audio container/encoding: "mp3", "wav", "pcm", "opus".
	Format string
	// Speed adjusts the speaking rate (1.0 = normal; provider range varies).
	Speed float64
	// Language hints the spoken language (BCP-47, e.g. "en", "zh").
	Language string
}

// Response is a chunk of synthesized audio. For streamed synthesis, each
// Response carries an *incremental* audio delta (concatenate every chunk's
// Audio to obtain the full speech); the final chunk has IsLast=true.
type Response struct {
	// Audio is the (possibly incremental) synthesized audio bytes.
	Audio []byte
	// MediaType is the MIME type of Audio, e.g. "audio/pcm", "audio/mpeg".
	MediaType string
	// IsLast marks the final chunk of a streamed synthesis.
	IsLast bool
}

// Model is a text-to-speech model. Non-realtime backends implement only
// Synthesize. Realtime (streaming-input) backends additionally satisfy the
// RealtimeModel interface.
type Model interface {
	// ModelName returns the TTS model identifier.
	ModelName() string
	// Synthesize converts text to speech and returns the full audio as a
	// single Response.
	Synthesize(ctx context.Context, text string, opts Options) (*Response, error)
}

// RealtimeModel is an optional interface for streaming-input TTS models.
// Connect initializes the realtime session; Push appends a text chunk and
// returns whatever audio is currently available (may be empty); Close drains
// any remaining audio and releases resources.
type RealtimeModel interface {
	Model
	Connect(ctx context.Context) error
	Push(ctx context.Context, text string, opts Options) (*Response, error)
	Close(ctx context.Context) (*Response, error)
}

// IsRealtime reports whether m also implements RealtimeModel.
func IsRealtime(m Model) bool {
	_, ok := m.(RealtimeModel)
	return ok
}
