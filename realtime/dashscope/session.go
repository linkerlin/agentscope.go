// realtime/dashscope/session.go — one live DashScope task as a
// realtime.Session: a single read pump maps server frames to the session
// event stream; control calls (audio/text/interrupt/close) serialize on a
// write mutex. The events channel is closed ONLY by the read pump (single
// closer rule): Close signals it via conn teardown.
package dashscope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/realtime"
)

// decodeBase64 is split out for clarity next to its encode counterpart.
func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// closeGrace bounds how long Close waits for the server's task-finished so
// the stream can end with SessionClosed; past it the socket is torn down
// (a dead server cannot be waited for indefinitely).
const closeGrace = 2 * time.Second

// session implements realtime.Session over one WebSocket task.
type session struct {
	conn   *websocket.Conn
	taskID string
	format realtime.AudioFormat

	events chan realtime.Event

	readDone  chan struct{} // closed by readPump on exit
	writeMu   sync.Mutex    // serializes conn writes (gorilla: one writer)
	closeOnce sync.Once
}

func newSession(conn *websocket.Conn, taskID string, format realtime.AudioFormat) *session {
	return &session{
		conn:     conn,
		taskID:   taskID,
		format:   format,
		events:   make(chan realtime.Event, 64),
		readDone: make(chan struct{}),
	}
}

// Events implements realtime.Session.
func (s *session) Events() <-chan realtime.Event { return s.events }

// Format reports the negotiated session (input) audio format.
func (s *session) Format() realtime.AudioFormat { return s.format }

// TaskID reports the DashScope task id (server-side dedup key).
func (s *session) TaskID() string { return s.taskID }

// SendAudio implements realtime.Session: continue-task with one base64 chunk.
func (s *session) SendAudio(ctx context.Context, chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	return s.send(clientFrame{
		Header:  frameHeader{Action: ActionContinueTask, TaskID: s.taskID, Streaming: streamingDuplex},
		Payload: clientPayload{Input: &clientInput{Audio: base64.StdEncoding.EncodeToString(chunk)}},
	})
}

// SendText implements realtime.Session: continue-task with a text input.
func (s *session) SendText(ctx context.Context, text string) error {
	return s.send(clientFrame{
		Header:  frameHeader{Action: ActionContinueTask, TaskID: s.taskID, Streaming: streamingDuplex},
		Payload: clientPayload{Input: &clientInput{Text: text}},
	})
}

// Interrupt implements realtime.Session: task-control interrupt (server-side
// generation stop; the server answers with an interrupted result).
func (s *session) Interrupt(ctx context.Context) error {
	return s.send(clientFrame{
		Header:  frameHeader{Action: ActionTaskControl, TaskID: s.taskID, Streaming: streamingDuplex},
		Payload: clientPayload{Command: CommandInterrupt},
	})
}

// Close implements realtime.Session (idempotent): finish-task best-effort,
// then a bounded grace wait for the server's task-finished (the stream then
// ends with SessionClosed); on timeout the socket is torn down and the read
// pump closes the stream on the drop.
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		_ = s.send(clientFrame{
			Header:  frameHeader{Action: ActionFinishTask, TaskID: s.taskID, Streaming: streamingDuplex},
			Payload: clientPayload{},
		})
		select {
		case <-s.readDone:
		case <-time.After(closeGrace):
		}
		_ = s.conn.Close()
	})
	return nil
}

// send marshals and writes one frame under the write mutex.
func (s *session) send(f clientFrame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// readPump is the single events-channel closer: it maps server frames until
// a terminal event (task-finished/task-failed), a read error (socket drop,
// Close teardown) or a decode that yields nothing further. Non-terminal
// frames that decode to zero events are skipped without touching the stream.
func (s *session) readPump() {
	defer close(s.events)
	defer close(s.readDone)
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return // dropped/closed socket: stream ends (Close semantics)
		}
		for _, ev := range decodeFrame(data) {
			s.events <- ev
			if realtime.Terminal(ev) {
				return
			}
		}
	}
}
