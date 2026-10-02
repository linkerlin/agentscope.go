// realtime/gemini/model.go — the realtime.Model implementation for Gemini
// Live: dial the GenerativeService BiDiStream WebSocket (API key rides the
// ?key= query), push setup, wait for setupComplete.
package gemini

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
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

// DefaultBaseURL is the Gemini Live BiDiStream endpoint (the API key and
// model ride the query string).
const DefaultBaseURL = "wss://generativelanguage.googleapis.com/google.ai.generativelanguage.v1beta.GenerativeServiceBidiStream"

// handshakeTimeout bounds the setupComplete wait.
const handshakeTimeout = 15 * time.Second

//go:embed cards/*.yaml
var cardsFS embed.FS

// ListModelCards loads the embedded Gemini Live cards (sorted by id).
func ListModelCards() ([]realtime.ModelCard, error) {
	entries, err := cardsFS.ReadDir("cards")
	if err != nil {
		return nil, fmt.Errorf("gemini live cards: %w", err)
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

// ToolDef declares one function exposed in the setup tools.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON schema
}

// Model is a Gemini Live backend session factory.
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
func WithBaseURL(url string) Option { return func(m *Model) { m.baseURL = url } }

// WithVoice overrides the session voice (default from the card).
func WithVoice(v string) Option { return func(m *Model) { m.voice = v } }

// WithInstructions sets the system instruction.
func WithInstructions(s string) Option { return func(m *Model) { m.instructions = s } }

// WithTools declares callable functions.
func WithTools(tools ...ToolDef) Option {
	return func(m *Model) { m.tools = append(m.tools, tools...) }
}

// NewLiveFlash targets the GA live model ("gemini-live-2.5-flash-preview").
func NewLiveFlash(apiKey string, opts ...Option) *Model {
	return New(apiKey, "gemini-live-2.5-flash-preview", opts...)
}

// New builds a Model for a custom live model id. The card comes from the
// embedded cards when the id matches; otherwise a conservative fallback.
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
		m.card = realtime.ModelCard{ModelCard: tts.ModelCard{Provider: "google", Model: model}}
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

	header := http.Header{}
	// Gemini Live authenticates via the key query param; a bearer-style
	// header is harmless and keeps proxies happy.
	header.Set("Authorization", "Bearer "+m.apiKey)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, m.endpoint(), header)
	if err != nil {
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("gemini live: dial: %w", err)
	}

	setup := clientFrame{Setup: newSetup("models/"+m.model, m.voice, m.instructions, m.tools)}
	if err := writeJSON(conn, setup); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, fmt.Errorf("gemini live: setup: %w", err)
	}
	if err := expectSetupComplete(conn); err != nil {
		_ = conn.Close()
		return nil, realtime.NegotiateAnswer{}, err
	}

	s := newSession(conn)
	s.events <- realtime.SessionStarted{SessionID: s.sessionID, Format: answer.Format, At: time.Now()}
	go s.readPump()
	return s, answer, nil
}

// endpoint builds the dial URL: base + ?key= (the key rides the query per
// the Live API; wsURL normalizes http(s) test bases).
func (m *Model) endpoint() string {
	return wsURL(m.baseURL) + "?key=" + m.apiKey
}

// expectSetupComplete reads until setupComplete (skipping unrelated
// messages); an error message fails the handshake.
func expectSetupComplete(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("gemini live: handshake read: %w", err)
		}
		var f serverFrame
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}
		if f.SetupComplete != nil {
			return nil
		}
		if f.Error != nil {
			return fmt.Errorf("gemini live: handshake: %s", f.Error.Message)
		}
	}
}

func writeJSON(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
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

// newSessionID mints a local session id (the Live API has no session id in
// setupComplete; session resumption handles are a separate opt-in).
func newSessionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "gemini-local"
	}
	return "gl_" + hex.EncodeToString(b[:])
}

var _ realtime.Model = (*Model)(nil)
