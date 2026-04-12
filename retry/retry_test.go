package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSuccessFirst(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := Do(ctx, Options{MaxAttempts: 3}, func() error {
		n++
		return nil
	})
	if err != nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := Do(ctx, Options{MaxAttempts: 3, Backoff: time.Millisecond}, func() error {
		n++
		if n < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil || n != 2 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestDoPermanent(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := Do(ctx, Options{MaxAttempts: 3}, func() error {
		n++
		return Permanent(errors.New("bad"))
	})
	if err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}
