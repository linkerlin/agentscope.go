// realtime/gemini/session.go — one live Gemini session as a
// realtime.Session. Same shape as the DashScope/OpenAI sessions: single
// read pump as the only events-channel closer, write mutex for control
// frames. Gemini has no goodbye frame: Close tears the socket and the read
// pump closes the stream on the drop.
package gemini

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

type session struct {
	conn      *websocket.Conn
	sessionID string

	events chan realtime.Event

	turn      turnState
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func newSession(conn *websocket.Conn) *session {
	return &session{
		conn:      conn,
		sessionID: newSessionID(),
		events:    make(chan realtime.Event, 64),
	}
}

// Events implements realtime.Session.
func (s *session) Events() <-chan realtime.Event { return s.events }

// SessionID reports the local session id.
func (s *session) SessionID() string { return s.sessionID }

// SendAudio implements realtime.Session: realtimeInput audio (base64 raw
// chunk, negotiated INPUT format).
func (s *session) SendAudio(ctx context.Context, chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	return s.send(audioFrame(chunk))
}

// SendText implements realtime.Session: one complete user text turn.
func (s *session) SendText(ctx context.Context, text string) error {
	return s.send(textFrame(text))
}

// Interrupt implements realtime.Session: the repo-declared near-cancel (an
// empty clientContent turn with turnComplete=true — see package doc; the
// card declares SERVER truncation, so the server's interrupted event carries
// the cut).
func (s *session) Interrupt(ctx context.Context) error {
	return s.send(cancelFrame())
}

// Close implements realtime.Session (idempotent): no goodbye frame in the
// protocol — the socket drops and the read pump ends the stream.
func (s *session) Close() error {
	s.closeOnce.Do(func() { _ = s.conn.Close() })
	return nil
}

func (s *session) send(f clientFrame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *session) readPump() {
	defer close(s.events)
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		for _, ev := range decodeFrame(data, &s.turn) {
			s.events <- ev
			if realtime.Terminal(ev) {
				return
			}
		}
	}
}

var _ realtime.Session = (*session)(nil)
