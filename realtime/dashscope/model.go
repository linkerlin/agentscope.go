// realtime/dashscope/model.go — the realtime.Model implementation: dials the
// DashScope WebSocket endpoint, negotiates the input audio format against the
// card, runs the run-task/task-started handshake and returns the session.
package dashscope

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
	"github.com/linkerlin/agentscope.go/tts"
)

// DefaultWSSURL is the DashScope WebSocket inference endpoint.
const DefaultWSSURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"

// handshakeTimeout bounds waiting for task-started after run-task.
const handshakeTimeout = 15 * time.Second

// ToolDef declares one tool exposed to a realtime task (run-task
// parameters.functions entry).
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // JSON schema
}

// Model is a DashScope realtime backend task. Build via the presets
// (NewQwenOmniRealtime / NewQwenAudioRealtime / NewCosyVoiceRealtime) or New
// for custom task/function pairs.
type Model struct {
	apiKey   string
	baseURL  string
	task     string
	function string
	model    string
	card     realtime.ModelCard
	params   map[string]any
	tools    []ToolDef
}

// Option tunes a Model.
type Option func(*Model)

// WithBaseURL overrides the WebSocket endpoint (tests point it at a local
// mock; mirrors can pin an enterprise gateway).
func WithBaseURL(url string) Option {
	return func(m *Model) { m.baseURL = url }
}

// WithParameters merges extra run-task parameters (preset keys win).
func WithParameters(p map[string]any) Option {
	return func(m *Model) {
		if m.params == nil {
			m.params = map[string]any{}
		}
		for k, v := range p {
			m.params[k] = v
		}
	}
}

// WithTools declares tools the task may call (ToolCall events flow back).
func WithTools(tools ...ToolDef) Option {
	return func(m *Model) { m.tools = append(m.tools, tools...) }
}

// WithCard overrides the embedded card for this model.
func WithCard(c realtime.ModelCard) Option {
	return func(m *Model) { m.card = c }
}

// NewQwenOmniRealtime opens a duplex voice task (asr+llm+tts).
func NewQwenOmniRealtime(apiKey string, opts ...Option) *Model {
	return New(apiKey, "qwen-omni-realtime", "asr+llm+tts", "qwen-omni-turbo-realtime", opts...)
}

// NewQwenAudioRealtime opens a speech-in / text-out task (asr+llm).
func NewQwenAudioRealtime(apiKey string, opts ...Option) *Model {
	return New(apiKey, "qwen-audio-realtime", "asr+llm", "qwen-audio-realtime", opts...)
}

// NewCosyVoiceRealtime opens a streaming speech-synthesis task (tts).
func NewCosyVoiceRealtime(apiKey string, opts ...Option) *Model {
	return New(apiKey, "cosyvoice-v2", "tts", "cosyvoice-v2", opts...)
}

// New builds a Model for a custom task/function/model triple. The capability
// card comes from the embedded cards when the wire model matches; otherwise a
// conservative fallback card is derived from the arguments.
func New(apiKey, task, function, model string, opts ...Option) *Model {
	m := &Model{
		apiKey:   apiKey,
		baseURL:  DefaultWSSURL,
		task:     task,
		function: function,
		model:    model,
	}
	if c, ok := cardByModel(model); ok {
		m.card = c
	} else {
		// Conservative fallback: no capability claims beyond identity.
		m.card = realtime.ModelCard{ModelCard: tts.ModelCard{Provider: "dashscope", Model: model}}
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// ModelName implements realtime.Model.
func (m *Model) ModelName() string { return m.model }

// Card implements realtime.Model.
func (m *Model) Card() *realtime.ModelCard {
	c := m.card
	return &c
}

// Connect implements realtime.Model: negotiate the input audio format
// against the card, dial, run-task and wait for task-started.
func (m *Model) Connect(ctx context.Context, offer realtime.NegotiateOffer) (realtime.Session, realtime.NegotiateAnswer, error) {
	answer, err := realtime.Negotiate(offer, m.card.AudioIn)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, err
	}
	taskID := newTaskID()

	header := http.Header{}
	header.Set("Authorization", "bearer "+m.apiKey)
	header.Set("X-DashScope-DataInspection", "enable")

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL(m.baseURL), header)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("dashscope realtime: dial: %w", err)
	}

	if err := m.sendRunTask(conn, taskID, answer.Format); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("dashscope realtime: run-task: %w", err)
	}
	if err := expectTaskStarted(conn); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, err
	}

	s := newSession(conn, taskID, answer.Format)
	s.events <- realtime.SessionStarted{SessionID: taskID, Format: answer.Format, At: time.Now()}
	go s.readPump()
	return s, answer, nil
}

// sendRunTask writes the opening frame carrying the negotiated input format.
func (m *Model) sendRunTask(conn *websocket.Conn, taskID string, in realtime.AudioFormat) error {
	params := map[string]any{
		"format":      in.Codec,
		"sample_rate": in.SampleRate,
		"channels":    in.Channels,
	}
	for k, v := range m.params {
		params[k] = v
	}
	if len(m.tools) > 0 {
		params["functions"] = m.tools
	}
	f := clientFrame{
		Header: frameHeader{Action: ActionRunTask, TaskID: taskID, Streaming: streamingDuplex},
		Payload: clientPayload{
			TaskGroup:  "audio",
			Task:       m.task,
			Function:   m.function,
			Model:      m.model,
			Parameters: params,
			Input:      &clientInput{},
		},
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// expectTaskStarted reads the first server frame synchronously: it must be
// task-started; task-failed surfaces its error message.
func expectTaskStarted(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("dashscope realtime: handshake read: %w", err)
	}
	var f serverFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("dashscope realtime: handshake frame: %w", err)
	}
	switch f.Header.Event {
	case EventTaskStarted:
		return nil
	case EventTaskFailed:
		msg := f.Header.ErrorMessage
		if msg == "" {
			msg = f.Header.ErrorCode
		}
		return fmt.Errorf("dashscope realtime: task failed to start: %s", msg)
	default:
		return fmt.Errorf("dashscope realtime: unexpected handshake event %q", f.Header.Event)
	}
}

// wsURL converts an http(s) base URL into its ws(s) form (test mocks speak
// http://127.0.0.1 URLs; bare hosts get ws://).
func wsURL(base string) string {
	switch {
	case strings.HasPrefix(base, "wss://"), strings.HasPrefix(base, "ws://"):
		return base
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://")
	default:
		return "ws://" + base
	}
}

// newTaskID mints a random 32-hex task id (server-side dedup key).
func newTaskID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is effectively fatal; fall back to a time-based
		// id rather than blocking session setup.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

var _ realtime.Model = (*Model)(nil)
