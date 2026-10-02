// Package realtime defines the voice-conversation contract (19.1): the
// model/session interfaces, the session event vocabulary, audio format
// negotiation, truncation capability declaration and a scripted Mock for
// contract tests. Backends (19.4 DashScope, 19.5 OpenAI Realtime) implement
// the Model interface; RealtimeAgent (19.3) consumes it.
//
// The card schema is SHARED with tts (19.1 acceptance): a realtime model
// card is a tts.ModelCard plus realtime-only optional fields, and the same
// YAML file parses under both.
package realtime

import (
	"context"

	"github.com/linkerlin/agentscope.go/tts"
)

// ModelCard is a realtime-capable voice model card. It embeds the TTS card
// schema verbatim (shared schema acceptance): the YAML shape is one schema,
// realtime fields are additive and optional.
type ModelCard struct {
	tts.ModelCard `yaml:",inline" json:",inline"`

	// Truncation declares how a barge-in cuts queued playback (三态).
	Truncation TruncationSupport `yaml:"truncation,omitempty" json:"truncation,omitempty"`
	// InputModalities lists accepted input kinds ("audio", "text");
	// empty defaults to ["audio", "text"].
	InputModalities []string `yaml:"input_modalities,omitempty" json:"input_modalities,omitempty"`
	// AudioIn lists acceptable input audio formats (negotiation defaults);
	// empty means the backend decides at Connect.
	AudioIn []AudioFormat `yaml:"audio_in,omitempty" json:"audio_in,omitempty"`
	// AudioOut lists produced output formats; empty means the backend
	// decides at Connect.
	AudioOut []AudioFormat `yaml:"audio_out,omitempty" json:"audio_out,omitempty"`
	// Tools marks tool-calling capable realtime models.
	Tools bool `yaml:"tools,omitempty" json:"tools,omitempty"`
}

// Model is a realtime voice backend: Connect opens one conversation session
// under a negotiated audio format.
type Model interface {
	// ModelName returns the backend model identifier.
	ModelName() string
	// Card returns the model's capability card.
	Card() *ModelCard
	// Connect negotiates the session audio format (client offer, server
	// answer) and opens the session.
	Connect(ctx context.Context, offer NegotiateOffer) (Session, NegotiateAnswer, error)
}

// Session is one live voice conversation. All session output arrives on the
// single event stream; ordering within the channel is the contract.
type Session interface {
	// Events is the session's event stream. It is closed after a terminal
	// event (ResponseDone with final=true, SessionClosed or Error).
	Events() <-chan Event
	// SendAudio appends one input audio chunk in the negotiated INPUT
	// format.
	SendAudio(ctx context.Context, chunk []byte) error
	// SendText injects a text input turn (text-in sessions, or text
	// alongside audio).
	SendText(ctx context.Context, text string) error
	// Interrupt requests a barge-in: stop generating and (per the card's
	// truncation capability) cut queued playback.
	Interrupt(ctx context.Context) error
	// Close ends the session; the event stream is closed with no further
	// events. Idempotent.
	Close() error
}

// NegotiateOffer carries the client's acceptable formats in preference
// order (index 0 = most preferred).
type NegotiateOffer struct{ Formats []AudioFormat }

// NegotiateAnswer is the server's binding choice for the session.
type NegotiateAnswer struct{ Format AudioFormat }
