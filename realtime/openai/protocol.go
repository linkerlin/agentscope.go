// Package openai implements the realtime.Model backend for the OpenAI
// Realtime API (19.5): duplex voice sessions over WebSocket with text input,
// tool calls, cancellation and reconnection. Unlike the DashScope backend
// (19.4, repo-declared frame contract), the OpenAI Realtime event vocabulary
// is public and stable, so the mapping below follows the live protocol
// verbatim: client events session.update / input_audio_buffer.append /
// conversation.item.create / response.create / response.cancel, server events
// session.created / session.updated /
// conversation.item.input_audio_transcription.completed / response.created /
// response.audio_transcript.delta / response.audio.delta /
// response.output_item.done / response.done / error.
//
// OpenAI's audio deltas carry no sequence number: this backend assigns a
// per-response monotonic counter (reset on response.created) to satisfy the
// realtime contract's ordering/de-dup guarantee (19.1).
package openai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/realtime"
)

// Client event types (client→server).
const (
	evSessionUpdate  = "session.update"
	evAudioAppend    = "input_audio_buffer.append"
	evItemCreate     = "conversation.item.create"
	evResponseCreate = "response.create"
	evResponseCancel = "response.cancel"
)

// Server event types (server→client). The primary names are the OpenAI
// Realtime vocabulary; the alt names are the GA-era / xAI aliases accepted
// by the same decoder (xAI's realtime endpoint at api.x.ai is
// OpenAI-compatible naming with response.output_audio.delta /
// response.text.delta / response.output_text.delta).
const (
	evSessionCreated = "session.created"
	evSessionUpdated = "session.updated"
	evTranscriptDone = "conversation.item.input_audio_transcription.completed"
	evResponseStart  = "response.created"
	evTextDelta      = "response.audio_transcript.delta"
	evTextDeltaAlt   = "response.text.delta"
	evTextDeltaAlt2  = "response.output_text.delta"
	evAudioDelta     = "response.audio.delta"
	evAudioDeltaAlt  = "response.output_audio.delta"
	evItemDone       = "response.output_item.done"
	evResponseDone   = "response.done"
	evError          = "error"
)

// statusCompleted is the response.done status of a fully generated turn
// (cancelled / incomplete / failed all map to Final=false).
const statusCompleted = "completed"

// ---- client frames ----

type sessionConfig struct {
	Modalities     []string         `json:"modalities"`
	Instructions   string           `json:"instructions,omitempty"`
	Voice          string           `json:"voice,omitempty"`
	InputAudioFmt  string           `json:"input_audio_format"`
	OutputAudioFmt string           `json:"output_audio_format"`
	TurnDetection  map[string]any   `json:"turn_detection,omitempty"`
	Tools          []map[string]any `json:"tools,omitempty"`
}

type clientEvent struct {
	Type     string         `json:"type"`
	Session  *sessionConfig `json:"session,omitempty"`  // session.update
	Audio    string         `json:"audio,omitempty"`    // input_audio_buffer.append
	Item     map[string]any `json:"item,omitempty"`     // conversation.item.create
	Response map[string]any `json:"response,omitempty"` // response.create
}

// textItem builds a user text conversation item.
func textItem(text string) map[string]any {
	return map[string]any{
		"type": "message",
		"role": "user",
		"content": []map[string]any{
			{"type": "input_text", "text": text},
		},
	}
}

// ---- server frames ----

type responseObj struct {
	ID     string `json:"id"`
	Status string `json:"status"` // completed | cancelled | incomplete | failed
}

type itemObj struct {
	Type      string `json:"type"` // message | function_call
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type errorObj struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// sessionRef is the session object on session.created / session.updated.
type sessionRef struct {
	ID string `json:"id"`
}

type serverEvent struct {
	Type    string `json:"type"`
	EventID string `json:"event_id,omitempty"`

	// session.created / session.updated
	Session *sessionRef `json:"session,omitempty"`
	// conversation.item.input_audio_transcription.completed
	Transcript string `json:"transcript,omitempty"`
	// response.created / response.done
	Response *responseObj `json:"response,omitempty"`
	// response.audio_transcript.delta (text) / response.audio.delta (base64)
	Delta string `json:"delta,omitempty"`
	// response.output_item.added / .done
	Item *itemObj `json:"item,omitempty"`
	// error
	Error *errorObj `json:"error,omitempty"`
}

// formatName renders an AudioFormat as the OpenAI wire codec name
// ("pcm16", "g711_ulaw", "g711_alaw"); an exact-rate mismatch is a
// negotiation error, never a silent transcode (19.1 rule).
func formatName(f realtime.AudioFormat) (string, error) {
	switch f.Codec {
	case "pcm":
		if f.SampleRate != 24000 || f.Channels != 1 {
			return "", fmt.Errorf("openai realtime: pcm must be 24kHz mono, got %s", f)
		}
		return "pcm16", nil
	case "g711_ulaw", "g711_alaw":
		if f.SampleRate != 8000 || f.Channels != 1 {
			return "", fmt.Errorf("openai realtime: %s must be 8kHz mono, got %s", f.Codec, f)
		}
		return f.Codec, nil
	default:
		return "", fmt.Errorf("openai realtime: unsupported codec %q", f.Codec)
	}
}

// decodeEvent maps one server event to session events (0..n). Unknown event
// types map to nothing: the realtime protocol is additive and unknown events
// must not kill the stream.
func decodeEvent(data []byte, seq *audioSequencer) []realtime.Event {
	var e serverEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil
	}
	switch e.Type {
	case evTranscriptDone:
		return []realtime.Event{realtime.UserTranscribed{Text: e.Transcript}}

	case evResponseStart:
		if e.Response == nil {
			return nil
		}
		seq.reset()
		return []realtime.Event{realtime.ResponseStarted{ResponseID: e.Response.ID}}

	case evTextDelta, evTextDeltaAlt, evTextDeltaAlt2:
		return []realtime.Event{realtime.TranscriptDelta{Text: e.Delta}}

	case evAudioDelta, evAudioDeltaAlt:
		raw, err := base64.StdEncoding.DecodeString(e.Delta)
		if err != nil {
			return nil // corrupt audio event: drop, do not kill the stream
		}
		return []realtime.Event{realtime.AudioOutDelta{Sequence: seq.next(), Data: raw}}

	case evItemDone:
		if e.Item != nil && e.Item.Type == "function_call" {
			return []realtime.Event{realtime.ToolCall{
				ID:   e.Item.CallID,
				Name: e.Item.Name,
				Args: []byte(e.Item.Arguments),
			}}
		}
		return nil

	case evResponseDone:
		if e.Response == nil {
			return nil
		}
		return []realtime.Event{realtime.ResponseDone{
			ResponseID: e.Response.ID,
			Final:      e.Response.Status == statusCompleted,
		}}

	case evError:
		msg := "openai realtime: error"
		if e.Error != nil {
			msg = strings.TrimSpace(e.Error.Message + " (" + e.Error.Code + ")")
		}
		return []realtime.Event{realtime.ErrorEvent{Err: msg}}

	default:
		// session.created / session.updated (handshake), rate limits,
		// added/partial events and anything unknown carry no session
		// semantics here.
		return nil
	}
}

// audioSequencer assigns the per-response monotonic audio sequence the
// realtime contract requires (OpenAI deltas carry no sequence of their own).
type audioSequencer struct {
	nextSeq int
}

func (a *audioSequencer) reset() { a.nextSeq = 0 }

func (a *audioSequencer) next() int {
	a.nextSeq++
	return a.nextSeq
}
