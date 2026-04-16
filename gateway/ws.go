package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

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

func (s *Server) handleChatWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req chatRequest
		if err := json.Unmarshal(data, &req); err != nil {
			_ = writeWSError(conn, fmt.Sprintf("parse error: %v", err))
			continue
		}
		if req.Text == "" {
			_ = writeWSError(conn, "text is required")
			continue
		}

		msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
		ch, err := s.agent.CallStream(r.Context(), msg)
		if err != nil {
			_ = writeWSError(conn, fmt.Sprintf("stream error: %v", err))
			continue
		}

		for chunk := range ch {
			if chunk == nil {
				continue
			}
			ev := streamEvent{Delta: chunk.GetTextContent()}
			if err := conn.WriteJSON(ev); err != nil {
				break
			}
		}
		if err := conn.WriteJSON(streamEvent{Done: true}); err != nil {
			break
		}
	}
}

func writeWSError(conn *websocket.Conn, text string) error {
	return conn.WriteJSON(map[string]string{"error": text})
}
