package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/message"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for demo; tighten in production
	},
}

const (
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 10 * time.Second
)

func (s *Server) handleChatWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	room := r.URL.Query().Get("room")

	ws := &wsSession{
		id:       sessionID,
		room:     room,
		conn:     conn,
		lastPing: time.Now(),
	}
	s.registerSession(ws)
	defer func() {
		s.unregisterSession(ws)
		ws.close()
	}()

	// Heartbeat goroutine
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.writeControl(websocket.PingMessage, []byte{}, time.Now().Add(heartbeatTimeout)); err != nil {
					return
				}
			case <-stopHeartbeat:
				return
			}
		}
	}()

	// Message read loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req chatRequest
		if err := json.Unmarshal(data, &req); err != nil {
			_ = ws.writeJSON(map[string]string{"error": fmt.Sprintf("parse error: %v", err)})
			continue
		}
		if req.Text == "" {
			_ = ws.writeJSON(map[string]string{"error": "text is required"})
			continue
		}

		msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
		ch, err := s.agent.CallStream(r.Context(), msg)
		if err != nil {
			_ = ws.writeJSON(map[string]string{"error": fmt.Sprintf("stream error: %v", err)})
			continue
		}

		for chunk := range ch {
			if chunk == nil {
				continue
			}
			ev := streamEvent{Delta: chunk.GetTextContent()}
			if err := ws.writeJSON(ev); err != nil {
				break
			}
		}
		if err := ws.writeJSON(streamEvent{Done: true}); err != nil {
			break
		}
	}
	close(stopHeartbeat)
}
