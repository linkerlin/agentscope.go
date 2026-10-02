// Package dashscope implements the realtime.Model backend for DashScope
// WebSocket inference tasks (19.4): qwen-omni-realtime (asr+llm+tts duplex
// voice), qwen-audio-realtime (asr+llm, text replies) and CosyVoice realtime
// (speech synthesis).
//
// Wire shape: the frame skeleton follows the public DashScope WebSocket
// protocol (header.action/event + payload task parameters + base64 audio,
// bearer auth). The result-generated payload output types are THIS REPO'S
// declared contract (same policy as channel/feishu WS): the mock server in
// the tests locks them, and decodeFrame/encodeFrame are the single adaptation
// points if the live service wording differs.
package dashscope

import (
	"encoding/json"
	"time"

	"github.com/linkerlin/agentscope.go/realtime"
)

// Client→server actions.
const (
	ActionRunTask      = "run-task"
	ActionContinueTask = "continue-task"
	ActionFinishTask   = "finish-task"
	ActionTaskControl  = "task-control"
)

// Server→client events.
const (
	EventTaskStarted     = "task-started"
	EventResultGenerated = "result-generated"
	EventTaskFinished    = "task-finished"
	EventTaskFailed      = "task-failed"
)

// CommandInterrupt is the task-control command requesting a barge-in.
const CommandInterrupt = "interrupt"

// result-generated payload output types (repo-declared contract).
const (
	OutInputText       = "input_text" // ASR partial (definite=false) or full utterance (definite=true)
	OutText            = "text"       // assistant transcript delta
	OutAudio           = "audio"      // assistant audio delta (base64 data + sequence)
	OutToolCalls       = "tool_calls"
	OutResponseStarted = "response_started"
	OutResponseDone    = "response_done"
	OutInterrupted     = "interrupted"
)

const streamingDuplex = "duplex"

// frameHeader is the shared frame header (client actions / server events).
type frameHeader struct {
	Action       string `json:"action,omitempty"`
	Event        string `json:"event,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	Streaming    string `json:"streaming,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// clientInput carries turn input on continue-task frames.
type clientInput struct {
	Audio string `json:"audio,omitempty"` // base64-encoded raw audio chunk
	Text  string `json:"text,omitempty"`
}

// clientPayload is the payload of a client frame.
type clientPayload struct {
	TaskGroup  string         `json:"task_group,omitempty"`
	Task       string         `json:"task,omitempty"`
	Function   string         `json:"function,omitempty"`
	Model      string         `json:"model,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Input      *clientInput   `json:"input,omitempty"`
	Command    string         `json:"command,omitempty"` // task-control commands
}

// clientFrame is one client→server message.
type clientFrame struct {
	Header  frameHeader   `json:"header"`
	Payload clientPayload `json:"payload"`
}

// serverToolCall is one tool invocation inside a tool_calls output.
type serverToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"` // JSON object or JSON-encoded string
}

// serverOutput is the payload.output of a result-generated frame.
type serverOutput struct {
	Type       string           `json:"type,omitempty"`
	Text       string           `json:"text,omitempty"`
	Definite   bool             `json:"definite,omitempty"`
	Data       string           `json:"data,omitempty"` // base64 audio
	Sequence   int              `json:"sequence,omitempty"`
	ResponseID string           `json:"response_id,omitempty"`
	Final      bool             `json:"final,omitempty"`
	ToolCalls  []serverToolCall `json:"tool_calls,omitempty"`
}

// serverFrame is one server→client message.
type serverFrame struct {
	Header struct {
		Event        string `json:"event"`
		TaskID       string `json:"task_id,omitempty"`
		ErrorCode    string `json:"error_code,omitempty"`
		ErrorMessage string `json:"error_message,omitempty"`
	} `json:"header"`
	Payload struct {
		Output serverOutput `json:"output"`
	} `json:"payload"`
}

// decodeFrame maps one server frame to session events (0..n). Unparseable or
// unrecognized frames map to nothing: an unknown frame must not kill the
// stream (forward compatibility).
func decodeFrame(data []byte) []realtime.Event {
	var f serverFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	switch f.Header.Event {
	case EventResultGenerated:
		return decodeOutput(f.Payload.Output)
	case EventTaskFinished:
		return []realtime.Event{realtime.SessionClosed{}}
	case EventTaskFailed:
		msg := f.Header.ErrorMessage
		if msg == "" {
			msg = f.Header.ErrorCode
		}
		if msg == "" {
			msg = "dashscope: task failed"
		}
		return []realtime.Event{realtime.ErrorEvent{Err: msg}}
	default:
		// task-started (consumed by the Connect handshake) and unknown
		// events carry no session semantics.
		return nil
	}
}

// decodeOutput maps one result-generated output to session events.
func decodeOutput(o serverOutput) []realtime.Event {
	switch o.Type {
	case OutInputText:
		if o.Definite {
			return []realtime.Event{realtime.UserTranscribed{Text: o.Text}}
		}
		return []realtime.Event{realtime.InputTranscriptDelta{Text: o.Text}}
	case OutText:
		return []realtime.Event{realtime.TranscriptDelta{Text: o.Text}}
	case OutAudio:
		if o.Data == "" {
			return nil // empty audio frame carries nothing
		}
		raw, err := decodeBase64(o.Data)
		if err != nil {
			return nil // corrupt audio frame: drop, do not kill the stream
		}
		return []realtime.Event{realtime.AudioOutDelta{Sequence: o.Sequence, Data: raw}}
	case OutToolCalls:
		var out []realtime.Event
		for _, tc := range o.ToolCalls {
			out = append(out, realtime.ToolCall{ID: tc.ID, Name: tc.Name, Args: toolArgs(tc.Arguments)})
		}
		return out
	case OutResponseStarted:
		return []realtime.Event{realtime.ResponseStarted{ResponseID: o.ResponseID}}
	case OutResponseDone:
		return []realtime.Event{realtime.ResponseDone{ResponseID: o.ResponseID, Final: o.Final}}
	case OutInterrupted:
		return []realtime.Event{realtime.Interrupted{At: time.Now()}}
	default:
		return nil
	}
}

// toolArgs normalises tool arguments to raw JSON bytes: the wire may carry a
// JSON object or a JSON-encoded string (both shapes appear across providers).
func toolArgs(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return []byte(s)
		}
	}
	return raw
}
