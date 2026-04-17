package transport

import "errors"

var (
	// ErrConnClosed is returned when operations are attempted on a closed connection.
	ErrConnClosed = errors.New("websocket connection is closed")
)
