// gateway/bodylimit.go realises the request-body half of 18.11: every JSON
// write endpoint shares one cap (default 1 MiB, matching the controlplane
// precedent). http.MaxBytesReader stops reading at the limit — an
// over-limit client cannot park a handler on an unbounded upload — and the
// handler's next Read fails with *http.MaxBytesError, which decodeJSONBody
// maps to 413 Payload Too Large.
package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// DefaultMaxBodyBytes caps every JSON write endpoint (18.11). 1 MiB matches
// the controlplane handlers' existing limit.
const DefaultMaxBodyBytes = 1 << 20

// maxBodyBytes is the mutable package default (WithMaxBodyBytes overrides).
var maxBodyBytes int64 = DefaultMaxBodyBytes

// WithMaxBodyBytes overrides the default body cap for every protected
// endpoint. Mostly for tests; deployments use the middleware with an
// explicit size.
func WithMaxBodyBytes(n int64) {
	if n > 0 {
		maxBodyBytes = n
	}
}

// limitBody wraps r.Body in an http.MaxBytesReader bound to w, so exceeding
// the cap both stops reading and poisons the connection (Go closes it after
// the handler writes the response).
func limitBody(w http.ResponseWriter, r *http.Request, n int64) {
	r.Body = http.MaxBytesReader(w, r.Body, n)
}

// isMaxBytesError reports whether err came from an http.MaxBytesReader
// boundary (directly or wrapped).
func isMaxBytesError(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// decodeJSONBody decodes the request body under the given byte cap. It is
// the single decode path for protected JSON write endpoints: over-limit
// bodies stop reading at the cap and map to 413; malformed JSON stays 400.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any, n int64) error {
	limitBody(w, r, n)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if isMaxBytesError(err) {
			return errTooLarge{}
		}
		return err
	}
	return nil
}

// errTooLarge marks a body that crossed the cap.
type errTooLarge struct{}

func (errTooLarge) Error() string { return "request body too large" }

// writeBodyLimitError responds 413 for errTooLarge and 400 for anything
// else, keeping per-handler boilerplate down.
func writeBodyLimitError(w http.ResponseWriter, err error) {
	var tl errTooLarge
	if errors.As(err, &tl) || isMaxBytesError(err) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, fmt.Sprintf("parse error: %v", err), http.StatusBadRequest)
}

// decodeJSONLimit decodes a JSON write endpoint's body under the package
// body cap (18.11). Over-limit → errTooLarge (map with
// writeBodyLimitError → 413); malformed JSON stays the caller's 400. This is
// the root package's uniform decode path for protected endpoints.
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeJSONBody(w, r, v, maxBodyBytes)
}

// BodyLimitMiddleware wraps every handler with the body cap. Endpoints that
// stream uploads (multipart KB ingest, audio) use their own explicit limits
// and are excluded by not being wrapped — JSON write endpoints are the
// protected population. Call once at assembly (RegisterAppRoutes and friends)
// or per-handler in tests.
func BodyLimitMiddleware(next http.Handler) http.Handler {
	return bodyLimitWith(next, maxBodyBytes)
}

func bodyLimitWith(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			limitBody(w, r, n)
		}
		next.ServeHTTP(w, r)
	})
}
