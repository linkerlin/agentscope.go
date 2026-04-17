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


func TestDoExhausted(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := Do(ctx, Options{MaxAttempts: 2}, func() error {
		n++
		return errors.New("transient")
	})
	if err == nil || n != 2 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestDoContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	err := Do(ctx, Options{MaxAttempts: 5, Backoff: 50 * time.Millisecond}, func() error {
		n++
		return errors.New("transient")
	})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoMaxAttemptsZero(t *testing.T) {
	ctx := context.Background()
	n := 0
	err := Do(ctx, Options{MaxAttempts: 0}, func() error {
		n++
		return errors.New("fail")
	})
	if err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestPermanentNil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("expected Permanent(nil) == nil")
	}
}

func TestPermanentErrorMethods(t *testing.T) {
	inner := errors.New("boom")
	p := Permanent(inner)
	if p.Error() != "boom" {
		t.Fatalf("expected Error() boom, got %s", p.Error())
	}
	if !errors.Is(p, inner) {
		t.Fatal("expected Unwrap to work")
	}
}
