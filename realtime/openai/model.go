// realtime/openai/model.go — the realtime.Model implementation for OpenAI
// Realtime: dial wss://api.openai.com/v1/realtime?model=<model> with bearer
// auth, wait for session.created, push session.update (formats, voice,
// server VAD, tools) and wait for session.updated before returning the
// session.
package openai

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
	"github.com/linkerlin/agentscope.go/tts"
)

// DefaultBaseURL is the OpenAI Realtime WebSocket endpoint (the model rides
// in the query string).
const DefaultBaseURL = "wss://api.openai.com/v1/realtime"

// handshakeTimeout bounds the session.created / session.updated waits.
const handshakeTimeout = 15 * time.Second

//go:embed cards/*.yaml
var cardsFS embed.FS

// ListModelCards loads the embedded OpenAI realtime cards (sorted by id).
func ListModelCards() ([]realtime.ModelCard, error) {
	entries, err := cardsFS.ReadDir("cards")
	if err != nil {
		return nil, fmt.Errorf("openai realtime cards: %w", err)
	}
	var cards []realtime.ModelCard
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := cardsFS.ReadFile("cards/" + e.Name())
		if err != nil {
			continue
		}
		var c realtime.ModelCard
		if err := yaml.Unmarshal(data, &c); err != nil {
			continue
		}
		cards = append(cards, c)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, nil
}

func cardByModel(model string) (realtime.ModelCard, bool) {
	cards, err := ListModelCards()
	if err != nil {
		return realtime.ModelCard{}, false
	}
	for _, c := range cards {
		if c.Model == model {
			return c, true
		}
	}
	return realtime.ModelCard{}, false
}

// ToolDef declares one function exposed in session.update tools.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON schema
}

// Model is an OpenAI Realtime backend session factory.
type Model struct {
	apiKey       string
	baseURL      string
	model        string
	card         realtime.ModelCard
	voice        string
	instructions string
	tools        []ToolDef
}

// Option tunes a Model.
type Option func(*Model)

// WithBaseURL overrides the WebSocket endpoint (tests point it at a local
// mock).
func WithBaseURL(url string) Option {
	return func(m *Model) { m.baseURL = url }
}

// WithVoice overrides the session voice (default from the card).
func WithVoice(v string) Option { return func(m *Model) { m.voice = v } }

// WithInstructions sets the session instructions (system prompt).
func WithInstructions(s string) Option { return func(m *Model) { m.instructions = s } }

// WithTools declares callable functions.
func WithTools(tools ...ToolDef) Option {
	return func(m *Model) { m.tools = append(m.tools, tools...) }
}

// NewGPTRealtime targets the GA realtime model ("gpt-realtime").
func NewGPTRealtime(apiKey string, opts ...Option) *Model {
	return New(apiKey, "gpt-realtime", opts...)
}

// NewGrokRealtime targets xAI's Grok realtime voice model
// ("grok-voice-latest" at wss://api.x.ai/v1/realtime — OpenAI-compatible
// event naming, accepted by the same decoder; 19.6).
func NewGrokRealtime(apiKey string, opts ...Option) *Model {
	return New(apiKey, "grok-voice-latest", append([]Option{WithBaseURL("wss://api.x.ai/v1/realtime")}, opts...)...)
}

// New builds a Model for a custom realtime model id (e.g.
// "gpt-4o-realtime-preview"). The card comes from the embedded cards when
// the id matches; otherwise a conservative fallback card is derived.
func New(apiKey, model string, opts ...Option) *Model {
	m := &Model{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		model:   model,
	}
	if c, ok := cardByModel(model); ok {
		m.card = c
		m.voice = c.DefaultVoice
	} else {
		// Conservative fallback: no capability claims beyond identity.
		m.card = realtime.ModelCard{ModelCard: tts.ModelCard{Provider: "openai", Model: model}}
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

// Connect implements realtime.Model.
func (m *Model) Connect(ctx context.Context, offer realtime.NegotiateOffer) (realtime.Session, realtime.NegotiateAnswer, error) {
	answer, err := realtime.Negotiate(offer, m.card.AudioIn)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, err
	}
	inFmt, err := formatName(answer.Format)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, err
	}
	outFmt := inFmt
	if len(m.card.AudioOut) > 0 {
		if f, err := formatName(m.card.AudioOut[0]); err == nil {
			outFmt = f
		}
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+m.apiKey)
	header.Set("OpenAI-Beta", "realtime=v1")

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL(m.baseURL)+"?model="+m.model, header)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("openai realtime: dial: %w", err)
	}

	created, err := m.readUntil(conn, evSessionCreated)
	if err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, err
	}
	if err := writeEvent(conn, clientEvent{Type: evSessionUpdate, Session: m.sessionConfig(inFmt, outFmt)}); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("openai realtime: session.update: %w", err)
	}
	if _, err := m.readUntil(conn, evSessionUpdated); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, err
	}

	s := newSession(conn)
	if created.Session != nil && created.Session.ID != "" {
		s.sessionID = created.Session.ID
	}
	s.events <- realtime.SessionStarted{SessionID: s.sessionID, Format: answer.Format, At: time.Now()}
	go s.readPump()
	return s, answer, nil
}

// sessionConfig builds the session.update payload.
func (m *Model) sessionConfig(inFmt, outFmt string) *sessionConfig {
	cfg := &sessionConfig{
		Modalities:     []string{"text", "audio"},
		Instructions:   m.instructions,
		Voice:          m.voice,
		InputAudioFmt:  inFmt,
		OutputAudioFmt: outFmt,
		TurnDetection:  map[string]any{"type": "server_vad"},
	}
	for _, t := range m.tools {
		tool := map[string]any{"type": "function", "name": t.Name}
		if t.Description != "" {
			tool["description"] = t.Description
		}
		if len(t.Parameters) > 0 {
			tool["parameters"] = t.Parameters
		}
		cfg.Tools = append(cfg.Tools, tool)
	}
	return cfg
}

// readUntil reads server events until one of the wanted type arrives
// (skipping unrelated ones) and returns it; an error event aborts the
// handshake with its message.
func (m *Model) readUntil(conn *websocket.Conn, want string) (serverEvent, error) {
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return serverEvent{}, fmt.Errorf("openai realtime: handshake read: %w", err)
		}
		var e serverEvent
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		switch e.Type {
		case want:
			return e, nil
		case evError:
			msg := "openai realtime: error"
			if e.Error != nil {
				msg = strings.TrimSpace(e.Error.Message + " (" + e.Error.Code + ")")
			}
			return serverEvent{}, fmt.Errorf("openai realtime: handshake: %s", msg)
		}
	}
}

func writeEvent(conn *websocket.Conn, ev clientEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
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

var _ realtime.Model = (*Model)(nil)
