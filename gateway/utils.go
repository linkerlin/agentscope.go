package gateway

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// generateID creates a simple unique ID with the given prefix.
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// readAllAndClose reads the entire body and closes it. Returns the raw bytes.
func readAllAndClose(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	return io.ReadAll(body)
}

func parseQueryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return n
}
