package shutdown

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout != 0 {
		t.Fatalf("expected zero timeout, got %v", cfg.Timeout)
	}
	if cfg.PartialReasoningPolicy != Save {
		t.Fatalf("expected SAVE policy, got %s", cfg.PartialReasoningPolicy)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateInvalidPolicy(t *testing.T) {
	cfg := GracefulShutdownConfig{
		Timeout:                time.Second,
		PartialReasoningPolicy: "INVALID",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid policy")
	}
}

func TestValidateNegativeTimeout(t *testing.T) {
	cfg := GracefulShutdownConfig{
		Timeout:                -1 * time.Second,
		PartialReasoningPolicy: Save,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for negative timeout")
	}
}
