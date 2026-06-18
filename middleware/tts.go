package middleware

import (
	"context"
	"encoding/base64"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tts"
)

// TTSMiddleware synthesizes speech for the assistant's reply text and appends
// it as an AudioBlock (base64-encoded) to the returned message.
//
// This is the Go-idiomatic equivalent of Python agentscope's TTSMiddleware
// (#1832), which injects DATA_BLOCK audio events into the reply stream. The Go
// synchronous reply lifecycle returns a single *message.Msg, so the synthesized
// audio is attached to that message's Content. Incremental (streaming-during-
// reasoning) TTS injection on the V2 event-stream path is a follow-up.
//
// By default TTS failures are non-fatal: the error is recorded in the message
// Metadata under "tts_error" and the text reply is still returned. Set Strict
// = true to propagate synthesis errors.
type TTSMiddleware struct {
	Base
	// Model is the TTS backend used to synthesize the reply text.
	Model tts.Model
	// Options are the synthesis options (voice, format, ...).
	Options tts.Options
	// Strict makes synthesis errors fail the reply instead of being recorded.
	Strict bool
}

// NewTTSMiddleware creates a TTSMiddleware with the given TTS model.
func NewTTSMiddleware(m tts.Model) *TTSMiddleware {
	return &TTSMiddleware{Model: m}
}

// WithOptions sets the synthesis options (builder-style).
func (m *TTSMiddleware) WithOptions(o tts.Options) *TTSMiddleware {
	m.Options = o
	return m
}

// WithStrict makes synthesis errors fail the reply (builder-style).
func (m *TTSMiddleware) WithStrict() *TTSMiddleware {
	m.Strict = true
	return m
}

// OnReply runs the reply, then synthesizes speech for the assistant's text and
// attaches the audio to the returned message.
func (m *TTSMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	msg, err := next(ctx)
	if err != nil || msg == nil {
		return msg, err
	}
	if msg.Role != message.RoleAssistant {
		return msg, nil
	}
	text := msg.GetTextContent()
	if text == "" {
		return msg, nil
	}

	resp, synthErr := m.Model.Synthesize(ctx, text, m.Options)
	if synthErr != nil {
		if m.Strict {
			return msg, synthErr
		}
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]any)
		}
		msg.Metadata["tts_error"] = synthErr.Error()
		return msg, nil
	}
	if resp != nil && len(resp.Audio) > 0 {
		mediaType := resp.MediaType
		if mediaType == "" {
			mediaType = "audio/mpeg"
		}
		msg.Content = append(msg.Content, message.NewAudioBlock(
			"", base64.StdEncoding.EncodeToString(resp.Audio), mediaType,
		))
	}
	return msg, nil
}
