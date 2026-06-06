package gateway

import (
	"fmt"
	"time"
)

// generateID creates a simple unique ID with the given prefix.
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
