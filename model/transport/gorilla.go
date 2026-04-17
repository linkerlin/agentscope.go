package transport

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// GorillaWebSocketTransport is a WebSocketTransport implementation based on gorilla/websocket.
type GorillaWebSocketTransport struct {
	dialer websocket.Dialer
}

// NewGorillaWebSocketTransport creates a new transport with default settings.
func NewGorillaWebSocketTransport() *GorillaWebSocketTransport {
	return &GorillaWebSocketTransport{
		dialer: websocket.Dialer{
			HandshakeTimeout: 30 * time.Second,
		},
	}
}

// NewGorillaWebSocketTransportWithDialer creates a transport with a custom dialer.
func NewGorillaWebSocketTransportWithDialer(dialer websocket.Dialer) *GorillaWebSocketTransport {
	return &GorillaWebSocketTransport{dialer: dialer}
}

// Connect implements WebSocketTransport.
func (t *GorillaWebSocketTransport) Connect(ctx context.Context, url string, headers map[string]string) (WebSocketConnection, error) {
	dialer := t.dialer
	if ctx != nil {
		// Respect context deadline if set
		if deadline, ok := ctx.Deadline(); ok {
			dialer.HandshakeTimeout = time.Until(deadline)
		}
	}

	httpHeader := make(http.Header, len(headers))
	for k, v := range headers {
		httpHeader.Set(k, v)
	}

	ws, resp, err := dialer.DialContext(ctx, url, httpHeader)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		return nil, &WebSocketTransportException{Op: "connect", URL: url, Code: code, Err: err}
	}

	return &gorillaConn{conn: ws}, nil
}

type gorillaConn struct {
	conn   *websocket.Conn
	closed bool
	mu     sync.RWMutex
}

func (c *gorillaConn) Send(ctx context.Context, data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return &WebSocketTransportException{Op: "send", Err: ErrConnClosed}
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *gorillaConn) SendBinary(ctx context.Context, data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return &WebSocketTransportException{Op: "send", Err: ErrConnClosed}
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *gorillaConn) Receive(ctx context.Context) (*WebSocketMessage, error) {
	// gorilla/websocket does not support context cancellation directly on ReadMessage.
	// We run ReadMessage in a goroutine and abort via SetReadDeadline when context is done.
	type result struct {
		msgType int
		data    []byte
		err     error
	}
	done := make(chan result, 1)

	go func() {
		msgType, data, err := c.conn.ReadMessage()
		done <- result{msgType: msgType, data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		// Try to unblock the reader by setting a past deadline.
		_ = c.conn.SetReadDeadline(time.Now())
		// Wait for the goroutine to finish to avoid leaking it.
		<-done
		// Reset deadline
		_ = c.conn.SetReadDeadline(time.Time{})
		return nil, ctx.Err()
	case res := <-done:
		if res.err != nil {
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
			return nil, &WebSocketTransportException{Op: "receive", Err: res.err}
		}
		mt := MessageText
		if res.msgType == websocket.BinaryMessage {
			mt = MessageBinary
		}
		return &WebSocketMessage{Type: mt, Data: res.data}, nil
	}
}

func (c *gorillaConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	// Send close frame and close underlying connection.
	_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return c.conn.Close()
}

func (c *gorillaConn) IsOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed
}
