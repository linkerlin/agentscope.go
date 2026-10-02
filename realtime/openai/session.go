// realtime/openai/session.go — one live OpenAI Realtime session as a
// realtime.Session. Mirrors the DashScope session shape (19.4): single read
// pump as the only events-channel closer, write mutex for control frames.
// OpenAI has no goodbye frame: Close tears the socket and the read pump
// closes the stream on the drop (no SessionClosed event — the stream simply
// ends, which the contract allows for Close).
package openai

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

type session struct {
	conn      *websocket.Conn
	sessionID string

	events chan realtime.Event

	seq       audioSequencer
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

// SessionID reports the live OpenAI session id (or a local fallback).
func (s *session) SessionID() string { return s.sessionID }

// SendAudio implements realtime.Session: input_audio_buffer.append.
func (s *session) SendAudio(ctx context.Context, chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	return s.send(clientEvent{
		Type:  evAudioAppend,
		Audio: base64.StdEncoding.EncodeToString(chunk),
	})
}

// SendText implements realtime.Session: a user text item plus response.create
// (a text turn is both injected AND answered — one round trip on the wire).
func (s *session) SendText(ctx context.Context, text string) error {
	if err := s.send(clientEvent{Type: evItemCreate, Item: textItem(text)}); err != nil {
		return err
	}
	return s.send(clientEvent{Type: evResponseCreate, Response: map[string]any{}})
}

// Interrupt implements realtime.Session: response.cancel (the card declares
// client truncation — the local playout cut is the agent's, 19.3).
func (s *session) Interrupt(ctx context.Context) error {
	return s.send(clientEvent{Type: evResponseCancel, Response: map[string]any{}})
}

// Close implements realtime.Session (idempotent): no goodbye frame in the
// protocol — the socket drops and the read pump ends the stream.
func (s *session) Close() error {
	s.closeOnce.Do(func() { _ = s.conn.Close() })
	return nil
}

func (s *session) send(ev clientEvent) error {
	data, err := json.Marshal(ev)
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
		for _, ev := range decodeEvent(data, &s.seq) {
			s.events <- ev
			if realtime.Terminal(ev) {
				return
			}
		}
	}
}

// newSessionID mints a local fallback session id.
func newSessionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "sess-local"
	}
	return "sess_" + hex.EncodeToString(b[:])
}

var _ realtime.Session = (*session)(nil)
