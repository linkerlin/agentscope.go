package transport

import (
	"context"
	"fmt"
)

// MessageType indicates whether the WebSocket message is text or binary.
type MessageType int

const (
	MessageText MessageType = iota
	MessageBinary
)

// WebSocketMessage represents a single WebSocket frame.
type WebSocketMessage struct {
	Type MessageType
	Data []byte
}

// String returns the message data as a string (convenient for text protocols).
func (m *WebSocketMessage) String() string {
	if m == nil {
		return ""
	}
	return string(m.Data)
}

// WebSocketConnection is an active WebSocket connection.
type WebSocketConnection interface {
	// Send writes a text message to the connection.
	Send(ctx context.Context, data []byte) error
	// SendBinary writes a binary message to the connection.
	SendBinary(ctx context.Context, data []byte) error
	// Receive reads the next message from the connection.
	Receive(ctx context.Context) (*WebSocketMessage, error)
	// Close closes the connection gracefully.
	Close() error
	// IsOpen reports whether the connection is still open.
	IsOpen() bool
}

// WebSocketTransport creates WebSocket connections.
type WebSocketTransport interface {
	// Connect establishes a WebSocket connection to the given URL with optional headers.
	Connect(ctx context.Context, url string, headers map[string]string) (WebSocketConnection, error)
}

// WebSocketTransportException is returned when WebSocket operations fail.
type WebSocketTransportException struct {
	Op   string
	URL  string
	Code int
	Err  error
}

func (e *WebSocketTransportException) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("websocket %s %s failed (code=%d): %v", e.Op, e.URL, e.Code, e.Err)
	}
	return fmt.Sprintf("websocket %s %s failed: %v", e.Op, e.URL, e.Err)
}

func (e *WebSocketTransportException) Unwrap() error { return e.Err }
