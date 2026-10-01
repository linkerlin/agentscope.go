// gateway/credential_binding_handlers.go realises the HTTP face of the
// interactive-credential state machine (18.4). Three idempotent,
// callback-facing transitions on top of service.TransitionCredential:
//
//	POST /api/v1/credentials/{id}/authorize — deliver the secret (or accept
//	    the binding as complete); PENDING → AUTHORIZED.
//	POST /api/v1/credentials/{id}/fail      — the external flow failed;
//	    PENDING → FAILED.
//	POST /api/v1/credentials/{id}/cancel    — the user backed out;
//	    PENDING → CANCELLED.
//
// Semantics: repeating the same transition is a no-op success (duplicate
// callbacks, retries); a different terminal state after finalization is
// 409; ownership is enforced like every other credential route; the secret
// never echoes back (22.1 json:"-").
package gateway

import (
	"errors"
	"net/http"

	"github.com/linkerlin/agentscope.go/service"
)

// credentialTransitionRequest is the (mostly empty) body of the fail/cancel
// transitions; authorize carries optional credential material.
type credentialTransitionRequest struct {
	// Value delivers the credential secret at authorize time (encrypted at
	// rest when a cipher is configured). Optional: flows that hand the
	// secret out-of-band authorize with an empty body.
	Value string `json:"value,omitempty"`
	// BindingRef optionally updates the external reference (e.g. the
	// platform-side id learned during the flow).
	BindingRef string `json:"binding_ref,omitempty"`
}

// transitionCredential is the shared ownership + dispatch path.
func (s *Server) transitionCredential(w http.ResponseWriter, r *http.Request, id string, target service.CredentialStatus, apply func(*service.Credential) error) {
	cred, err := s.storage.GetCredential(r.Context(), id)
	if err != nil {
		http.Error(w, "credential not found", http.StatusNotFound)
		return
	}
	if cred.UserID != service.UserIDFromContext(r.Context()) {
		http.Error(w, "credential not found", http.StatusNotFound) // no existence leak
		return
	}
	out, err := service.TransitionCredential(r.Context(), s.storage, id, target, apply)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCredentialNotFound):
			http.Error(w, "credential not found", http.StatusNotFound)
		case errors.Is(err, service.ErrCredentialFinalized):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     out.ID,
		"status": out.NormalizedStatus(),
	})
}

func (s *Server) handleAuthorizeCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req credentialTransitionRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		writeBodyLimitError(w, err)
		return
	}
	// Production mode: a delivered secret needs the cipher, same as the
	// create path (22.1). An empty-body authorize (out-of-band secret) is
	// exempt.
	if req.Value != "" && s.production && s.cipher == nil {
		http.Error(w, "credential storage requires a cipher in production mode (configure AppConfig.Cipher)", http.StatusBadRequest)
		return
	}
	s.transitionCredential(w, r, id, service.CredentialAuthorized, func(c *service.Credential) error {
		if req.Value != "" {
			encrypted := req.Value
			if s.cipher != nil {
				enc, err := s.cipher.Encrypt(req.Value)
				if err != nil {
					return err
				}
				encrypted = enc
			}
			c.Encrypted = encrypted
		}
		if req.BindingRef != "" {
			c.BindingRef = req.BindingRef
		}
		return nil
	})
}

func (s *Server) handleFailCredential(w http.ResponseWriter, r *http.Request) {
	var req credentialTransitionRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		writeBodyLimitError(w, err)
		return
	}
	s.transitionCredential(w, r, r.PathValue("id"), service.CredentialFailed, func(c *service.Credential) error {
		if req.BindingRef != "" {
			c.BindingRef = req.BindingRef
		}
		return nil
	})
}

func (s *Server) handleCancelCredential(w http.ResponseWriter, r *http.Request) {
	var req credentialTransitionRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		writeBodyLimitError(w, err)
		return
	}
	s.transitionCredential(w, r, r.PathValue("id"), service.CredentialCancelled, func(c *service.Credential) error {
		if req.BindingRef != "" {
			c.BindingRef = req.BindingRef
		}
		return nil
	})
}
