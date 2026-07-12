package gateway

import (
	"net/http"
	"time"

	"github.com/linkerlin/agentscope.go/logging"
	"github.com/linkerlin/agentscope.go/service"
)

// RequestLoggingMiddleware returns an http.Handler that enriches each request
// context with a structured logger (request_id, user_id, method, path) and
// logs the completed request at info level with status + duration. It is the
// gateway's standard access log, using the logging/ package convention.
func RequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateID("req")
		}
		userID := service.UserIDFromContext(r.Context())

		log := logging.Default()
		ctx := logging.WithLogger(r.Context(), log)

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		log.Info("request",
			logging.KeyRequestID, reqID,
			logging.KeyUserID, userID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			logging.KeyDuration, time.Since(start).Milliseconds(),
		)
	})
}
