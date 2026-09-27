package sessionapi

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// newSessionID mints an unguessable server-side session ID (22.2). Clients
// never contribute to ID generation: a request either references an existing
// session (checked by checkSessionAccess) or lets the server mint one.
func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is fatal-grade; fall back to a time-based ID
		// rather than panicking inside a request handler.
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return "sess-" + base64.RawURLEncoding.EncodeToString(buf)
}

// ensureSession is invoked when a request starts a new session (no incoming
// session ID). It mints a server-side ID and, when storage is configured,
// persists the session record owned by the authenticated user so that later
// requests referencing the ID resolve ownership. Without storage the ID is
// still minted (and returned via the session header) but nothing is written.
func (h *Handlers) ensureSession(r *http.Request) (string, error) {
	id := newSessionID()
	if h.d.Storage == nil {
		return id, nil
	}
	sess := &service.Session{
		ID:        id,
		UserID:    service.UserIDFromContext(r.Context()),
		CreatedAt: time.Now().UTC(),
	}
	if err := h.d.Storage.SaveSession(r.Context(), sess); err != nil {
		return "", fmt.Errorf("persist new session: %w", err)
	}
	return id, nil
}
