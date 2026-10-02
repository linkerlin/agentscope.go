// realtime/mock.go — a scripted Model for contract tests (19.1) and the
// test base of RealtimeAgent (19.3). A script is a list of steps; each step
// either emits events when the session advances, or reacts to a control
// call (audio/text/interrupt). The mock records every control call in order
// so tests can assert the full interaction, not just the event stream.
package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockStep is one scripted beat. Exactly one field is set:
//   - OnAudio / OnText / OnInterrupt: the step fires when that control
//     call arrives (events are emitted for it);
//   - Auto: the step fires on session start.
type MockStep struct {
	OnAudio     []Event
	OnText      []Event
	OnInterrupt []Event
	Auto        []Event
}

// MockModel is a realtime.Model playing a fixed script.
type MockModel struct {
	mu         sync.Mutex
	card       ModelCard
	supported  []AudioFormat
	script     []MockStep
	sessions   []*MockSession
	connectErr error
}

// NewMockModel builds a mock over the given script and supported formats
// (empty = accept any offered format).
func NewMockModel(card ModelCard, supported []AudioFormat, script []MockStep) *MockModel {
	return &MockModel{card: card, supported: supported, script: script}
}

// FailConnect makes Connect return the error (assembly-failure tests).
func (m *MockModel) FailConnect(err error) *MockModel {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectErr = err
	return m
}

// ModelName implements Model.
func (m *MockModel) ModelName() string { return m.card.Model }

// Card implements Model.
func (m *MockModel) Card() *ModelCard {
	c := m.card
	return &c
}

// Sessions returns every session opened (for per-session assertions).
func (m *MockModel) Sessions() []*MockSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*MockSession(nil), m.sessions...)
}

// Connect implements Model: negotiates the format and starts the script.
func (m *MockModel) Connect(ctx context.Context, offer NegotiateOffer) (Session, NegotiateAnswer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return nil, NegotiateAnswer{}, m.connectErr
	}
	answer, err := Negotiate(offer, m.supported)
	if err != nil {
		return nil, NegotiateAnswer{}, err
	}
	s := &MockSession{
		events: make(chan Event, 64),
		format: answer.Format,
	}
	s.model = m
	s.script = m.script
	m.sessions = append(m.sessions, s)
	// Session-open event plus any Auto steps, in order.
	s.events <- SessionStarted{SessionID: fmt.Sprintf("mock-%d", len(m.sessions)), Format: answer.Format, At: time.Now()}
	s.advance(func(st MockStep) bool { return len(st.Auto) > 0 }, func(st MockStep) []Event { return st.Auto })
	return s, answer, nil
}

// MockSession is one scripted session with full call recording.
type MockSession struct {
	model *MockModel

	mu        sync.Mutex
	script    []MockStep
	pos       int
	events    chan Event
	format    AudioFormat
	closed    bool
	calls     []string // control-call recording in order
	closeOnce sync.Once
}

// Events implements Session.
func (s *MockSession) Events() <-chan Event { return s.events }

// Format reports the negotiated session format.
func (s *MockSession) Format() AudioFormat { return s.format }

// Calls returns the recorded control calls in order
// ("audio", "text:…", "interrupt", "close").
func (s *MockSession) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

// SendAudio implements Session.
func (s *MockSession) SendAudio(ctx context.Context, chunk []byte) error {
	s.mu.Lock()
	s.calls = append(s.calls, "audio")
	s.mu.Unlock()
	s.advance(func(st MockStep) bool { return len(st.OnAudio) > 0 }, func(st MockStep) []Event { return st.OnAudio })
	return nil
}

// SendText implements Session.
func (s *MockSession) SendText(ctx context.Context, text string) error {
	s.mu.Lock()
	s.calls = append(s.calls, "text:"+text)
	s.mu.Unlock()
	s.advance(func(st MockStep) bool { return len(st.OnText) > 0 }, func(st MockStep) []Event { return st.OnText })
	return nil
}

// Interrupt implements Session.
func (s *MockSession) Interrupt(ctx context.Context) error {
	s.mu.Lock()
	s.calls = append(s.calls, "interrupt")
	s.mu.Unlock()
	s.advance(func(st MockStep) bool { return len(st.OnInterrupt) > 0 }, func(st MockStep) []Event { return st.OnInterrupt })
	return nil
}

// Close implements Session (idempotent; closes the stream).
func (s *MockSession) Close() error {
	s.mu.Lock()
	s.calls = append(s.calls, "close")
	s.mu.Unlock()
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.events)
	})
	return nil
}

// advance walks the script from the current position, consuming every step
// that matches pick (a step is consumed even when it emits nothing —
// skip steps are how timing gaps are modelled). It stops at the first
// non-matching step (waiting for its control call) or the end of script
// (which closes the stream with a terminal event if none was emitted).
func (s *MockSession) advance(pick func(MockStep) bool, eventsOf func(MockStep) []Event) {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		if s.pos >= len(s.script) {
			s.mu.Unlock()
			// Script exhausted without a terminal event: close cleanly so
			// consumers never hang on a half-scripted session.
			s.Close()
			return
		}
		st := s.script[s.pos]
		if !pick(st) {
			s.mu.Unlock()
			return
		}
		s.pos++
		s.mu.Unlock()
		for _, ev := range eventsOf(st) {
			s.events <- ev
		}
	}
}

var (
	_ Model   = (*MockModel)(nil)
	_ Session = (*MockSession)(nil)
)
