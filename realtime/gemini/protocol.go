// Package gemini implements the realtime.Model backend for the Google
// Gemini Live API (19.6): duplex voice over the GenerativeService
// BiDiStream WebSocket. The event vocabulary follows the live protocol:
// setup / setupComplete handshake, realtimeInput audio in, clientContent
// text turns, serverContent model turns (text transcript parts + inline
// audio parts), inputTranscription, interrupted, turnComplete, toolCall.
//
// Two mapping notes (the encode/decode layer is the single adaptation
// point, as with every backend in this repo):
//
//   - Gemini has no response.cancel: Interrupt maps to an empty
//     clientContent turn with turnComplete=true (repo-declared near-cancel
//     — in VAD mode the user's speech is the primary interrupt and the
//     server reports serverContent.interrupted).
//   - Gemini has no response.created: the first modelTurn part of a turn
//     synthesizes ResponseStarted with a locally-minted response id; a
//     turnComplete closes it (Final=false when an interrupted preceded it).
package gemini

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/realtime"
)

// Input/output PCM mime types (the Live wire format).
const (
	inputAudioMime  = "audio/pcm;rate=16000"
	outputAudioMime = "audio/pcm;rate=24000"
)

// ---- client frames ----

type prebuiltVoice struct {
	VoiceName string `json:"voiceName,omitempty"`
}

type speechConfig struct {
	VoiceConfig prebuiltVoice `json:"voiceConfig"`
}

type generationConfig struct {
	ResponseModalities []string      `json:"responseModalities"`
	SpeechConfig       *speechConfig `json:"speechConfig,omitempty"`
}

type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type toolDecl struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

type setupRequest struct {
	Model             string           `json:"model"`
	GenerationConfig  generationConfig `json:"generationConfig"`
	SystemInstruction string           `json:"systemInstruction,omitempty"`
	Tools             []toolDecl       `json:"tools,omitempty"`
	// The transcription toggles are EMPTY-OBJECT flags in the Live protocol
	// ({} = on); they must NOT be omitempty — encoding/json drops an empty
	// map under omitempty and the toggle would never reach the wire.
	InputAudioTranscription  map[string]any `json:"inputAudioTranscription"`
	OutputAudioTranscription map[string]any `json:"outputAudioTranscription"`
}

type contentPart struct {
	Text       string `json:"text,omitempty"`
	InlineData *struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	} `json:"inlineData,omitempty"`
}

type contentTurn struct {
	Role  string        `json:"role"`
	Parts []contentPart `json:"parts"`
}

// clientFrame is one client→server message; exactly one field is set.
type clientFrame struct {
	Setup *setupRequest `json:"setup,omitempty"`

	// realtimeInput carries raw input audio (repo-declared shape: `audio`
	// is the live field; `mediaChunks` is the earlier bidi preview form —
	// encode stays on `audio`, the adaptation point note above).
	RealtimeInput *struct {
		Audio string `json:"audio,omitempty"`
	} `json:"realtimeInput,omitempty"`

	ClientContent *struct {
		Turns        []contentTurn `json:"turns,omitempty"`
		TurnComplete bool          `json:"turnComplete"`
	} `json:"clientContent,omitempty"`
}

// newSetup builds the handshake frame for a model + voice + tools.
func newSetup(model, voice, instructions string, tools []ToolDef) *setupRequest {
	req := &setupRequest{
		Model: model,
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"AUDIO"},
		},
		SystemInstruction:        instructions,
		InputAudioTranscription:  map[string]any{},
		OutputAudioTranscription: map[string]any{},
	}
	if voice != "" {
		req.GenerationConfig.SpeechConfig = &speechConfig{VoiceConfig: prebuiltVoice{VoiceName: voice}}
	}
	for _, t := range tools {
		req.Tools = append(req.Tools, toolDecl{FunctionDeclarations: []functionDeclaration{{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}}})
	}
	return req
}

// audioFrame builds one realtimeInput audio chunk (base64 raw pcm).
func audioFrame(chunk []byte) clientFrame {
	f := clientFrame{}
	f.RealtimeInput = &struct {
		Audio string `json:"audio,omitempty"`
	}{Audio: base64.StdEncoding.EncodeToString(chunk)}
	return f
}

// textFrame builds one complete user text turn.
func textFrame(text string) clientFrame {
	f := clientFrame{}
	f.ClientContent = &struct {
		Turns        []contentTurn `json:"turns,omitempty"`
		TurnComplete bool          `json:"turnComplete"`
	}{
		Turns:        []contentTurn{{Role: "user", Parts: []contentPart{{Text: text}}}},
		TurnComplete: true,
	}
	return f
}

// cancelFrame is the repo-declared near-cancel (see package doc).
func cancelFrame() clientFrame {
	f := clientFrame{}
	f.ClientContent = &struct {
		Turns        []contentTurn `json:"turns,omitempty"`
		TurnComplete bool          `json:"turnComplete"`
	}{TurnComplete: true}
	return f
}

// ---- server frames ----

type serverContent struct {
	ModelTurn          *struct{ Parts []contentPart } `json:"modelTurn,omitempty"`
	InputTranscription *struct{ Text string }         `json:"inputTranscription,omitempty"`
	Interrupted        bool                           `json:"interrupted,omitempty"`
	TurnComplete       bool                           `json:"turnComplete,omitempty"`
	GenerationComplete bool                           `json:"generationComplete,omitempty"`
}

type functionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type serverFrame struct {
	SetupComplete *struct{}      `json:"setupComplete,omitempty"`
	ServerContent *serverContent `json:"serverContent,omitempty"`
	ToolCall      *struct {
		FunctionCalls []functionCall `json:"functionCalls"`
	} `json:"toolCall,omitempty"`
	UsageMetadata map[string]any `json:"usageMetadata,omitempty"`
	GoAway        map[string]any `json:"goAway,omitempty"`
	Error         *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// turnState tracks the synthesized response-turn lifecycle (Gemini has no
// response.created/done of its own).
type turnState struct {
	inTurn      bool // a model turn is open (ResponseStarted synthesized)
	interrupted bool // an interrupted arrived during this turn
	nextID      int
	seq         int // audio sequence within the turn
}

// decodeFrame maps one server message to session events (0..n). Unknown
// messages (usageMetadata, goAway, toolCallCancellation, …) map to nothing:
// unknown events must not kill the stream.
func decodeFrame(data []byte, st *turnState) []realtime.Event {
	var f serverFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	var out []realtime.Event
	if f.Error != nil {
		msg := strings.TrimSpace(f.Error.Message)
		if msg == "" {
			msg = fmt.Sprintf("gemini live: error %d", f.Error.Code)
		}
		return append(out, realtime.ErrorEvent{Err: msg})
	}
	if sc := f.ServerContent; sc != nil {
		if sc.Interrupted {
			st.interrupted = true
			out = append(out, realtime.Interrupted{})
		}
		if it := sc.InputTranscription; it != nil && it.Text != "" {
			out = append(out, realtime.UserTranscribed{Text: it.Text})
		}
		if mt := sc.ModelTurn; mt != nil {
			for _, p := range mt.Parts {
				if p.Text != "" || (p.InlineData != nil && strings.HasPrefix(p.InlineData.MimeType, "audio/")) {
					out = append(out, st.openTurn()...)
				}
				if p.Text != "" {
					out = append(out, realtime.TranscriptDelta{Text: p.Text})
				}
				if p.InlineData != nil {
					raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
					if err == nil && len(raw) > 0 {
						st.seq++
						out = append(out, realtime.AudioOutDelta{Sequence: st.seq, Data: raw})
					}
				}
			}
		}
		if sc.TurnComplete {
			out = append(out, st.closeTurn())
		}
	}
	if tc := f.ToolCall; tc != nil {
		for i, fc := range tc.FunctionCalls {
			id := fc.ID
			if id == "" {
				id = fmt.Sprintf("%s-%d", fc.Name, i)
			}
			out = append(out, realtime.ToolCall{ID: id, Name: fc.Name, Args: fc.Args})
		}
	}
	return out
}

// openTurn synthesizes ResponseStarted once per model turn.
func (st *turnState) openTurn() []realtime.Event {
	if st.inTurn {
		return nil
	}
	st.inTurn = true
	st.seq = 0
	st.nextID++
	return []realtime.Event{realtime.ResponseStarted{ResponseID: fmt.Sprintf("gemini-turn-%d", st.nextID)}}
}

// closeTurn closes the synthesized turn; an interrupted turn is NOT final.
func (st *turnState) closeTurn() realtime.Event {
	id := fmt.Sprintf("gemini-turn-%d", st.nextID)
	final := !st.interrupted
	st.inTurn = false
	st.interrupted = false
	return realtime.ResponseDone{ResponseID: id, Final: final}
}
