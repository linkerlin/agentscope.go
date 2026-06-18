package tts

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/model"
)

// speechSynthesizer is the minimal contract the OpenAI adapter depends on.
// *model.OpenAITTS satisfies it (via AudioModel.SynthesizeSpeech), so the
// adapter reuses the existing implementation with zero duplication.
type speechSynthesizer interface {
	SynthesizeSpeech(ctx context.Context, text string, opts model.AudioOptions) ([]byte, error)
	ModelName() string
}

// OpenAIAdapter wraps an OpenAI-compatible TTS backend (model.OpenAITTS) as a
// tts.Model. It is a non-realtime model.
type OpenAIAdapter struct {
	backend  speechSynthesizer
	defaults Options
}

// NewOpenAIAdapter wraps an existing *model.OpenAITTS as a tts.Model.
func NewOpenAIAdapter(tts *model.OpenAITTS) *OpenAIAdapter {
	return &OpenAIAdapter{backend: tts}
}

// WithDefaults sets default Options applied when a caller omits a field.
func (a *OpenAIAdapter) WithDefaults(o Options) *OpenAIAdapter {
	a.defaults = o
	return a
}

// ModelName returns the underlying TTS model identifier.
func (a *OpenAIAdapter) ModelName() string {
	return a.backend.ModelName()
}

// Synthesize converts text to speech via the wrapped OpenAI TTS backend.
func (a *OpenAIAdapter) Synthesize(ctx context.Context, text string, opts Options) (*Response, error) {
	merged := mergeOptions(a.defaults, opts)
	resp, err := a.backend.SynthesizeSpeech(ctx, text, model.AudioOptions{
		Voice:    merged.Voice,
		Format:   merged.Format,
		Speed:    merged.Speed,
		Language: merged.Language,
	})
	if err != nil {
		return nil, fmt.Errorf("tts openai: %w", err)
	}
	return &Response{
		Audio:     resp,
		MediaType: mediaTypeForFormat(merged.Format),
		IsLast:    true,
	}, nil
}

// mediaTypeForFormat maps an audio format token to its MIME type.
func mediaTypeForFormat(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/ogg"
	case "pcm":
		return "audio/pcm"
	case "", "mp3":
		return "audio/mpeg"
	default:
		return "audio/" + format
	}
}

// mergeOptions overlays per-call opts on top of defaults (call wins).
func mergeOptions(defaults, call Options) Options {
	out := defaults
	if call.Voice != "" {
		out.Voice = call.Voice
	}
	if call.Format != "" {
		out.Format = call.Format
	}
	if call.Speed != 0 {
		out.Speed = call.Speed
	}
	if call.Language != "" {
		out.Language = call.Language
	}
	if out.Format == "" {
		out.Format = "mp3"
	}
	return out
}
