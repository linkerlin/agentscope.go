package shutdown

import (
	"errors"
	"time"
)

// PartialReasoningPolicy controls whether incomplete reasoning content is
// preserved or discarded when a shutdown-related interruption occurs.
type PartialReasoningPolicy string

const (
	// Save retains partial reasoning results (e.g., accumulated history) so
	// that the agent can resume or summarise its work.
	Save PartialReasoningPolicy = "SAVE"
	// Discard drops any in-flight reasoning content.
	Discard PartialReasoningPolicy = "DISCARD"
)

// GracefulShutdownConfig defines how the system behaves during shutdown.
type GracefulShutdownConfig struct {
	// Timeout is the maximum duration to wait for ongoing calls to finish.
	// A zero or negative value means "indefinite" (no forced timeout).
	Timeout time.Duration
	// PartialReasoningPolicy decides what to do with unfinished reasoning.
	PartialReasoningPolicy PartialReasoningPolicy
}

// DefaultConfig returns the out-of-box shutdown behaviour:
//   - Timeout: indefinite
//   - Policy: Save partial reasoning
func DefaultConfig() GracefulShutdownConfig {
	return GracefulShutdownConfig{
		Timeout:                0,
		PartialReasoningPolicy: Save,
	}
}

// Validate checks the configuration for invalid values.
func (c GracefulShutdownConfig) Validate() error {
	if c.PartialReasoningPolicy != Save && c.PartialReasoningPolicy != Discard {
		return errors.New("shutdown: partialReasoningPolicy must be SAVE or DISCARD")
	}
	if c.Timeout < 0 {
		return errors.New("shutdown: timeout must be >= 0 (0 = indefinite)")
	}
	return nil
}
