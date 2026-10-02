package realtime

import (
	"context"
	"testing"
	"time"
)

func audioFrame(seq int64) Frame {
	return Frame{Sequence: seq, Kind: FrameAudio, Audio: []byte{byte(seq)}}
}

// TestTransportOrderAndClose: frames arrive in send order; the stream
// closes on Close; sends after Close refuse.
func TestTransportOrderAndClose(t *testing.T) {
	tr := NewBoundedTransport(8, BlockPolicy).Start()
	ctx := context.Background()
	for i := int64(0); i < 4; i++ {
		if err := tr.Send(ctx, audioFrame(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Send(ctx, Frame{Sequence: 100, Kind: FrameControl, Control: ControlInterrupt}); err != nil {
		t.Fatal(err)
	}
	_ = tr.Close()
	// Sends after close refuse.
	if err := tr.Send(ctx, audioFrame(99)); err != ErrTransportClosed {
		t.Fatalf("post-close send: %v", err)
	}
	want := []int64{0, 1, 2, 3, 100}
	for i, w := range want {
		select {
		case f := <-tr.Recv():
			if f.Sequence != w {
				t.Fatalf("frame %d: seq %d want %d", i, f.Sequence, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("frame %d never arrived", i)
		}
	}
	if _, open := <-tr.Recv(); open {
		t.Fatal("stream must close after Close")
	}
}

// TestTransportBackpressureFail: FailPolicy refuses the send at capacity.
func TestTransportBackpressureFail(t *testing.T) {
	tr := NewBoundedTransport(2, FailPolicy)
	ctx := context.Background()
	if err := tr.Send(ctx, audioFrame(0)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Send(ctx, audioFrame(1)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Send(ctx, audioFrame(2)); err != ErrTransportFull {
		t.Fatalf("full transport must refuse, got %v", err)
	}
}

// TestTransportBackpressureBlock: BlockPolicy waits for space and honours
// context cancellation while blocked.
func TestTransportBackpressureBlock(t *testing.T) {
	tr := NewBoundedTransport(1, BlockPolicy)
	ctx := context.Background()
	if err := tr.Send(ctx, audioFrame(0)); err != nil {
		t.Fatal(err)
	}
	blocked, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := tr.Send(blocked, audioFrame(1)); err == nil {
		t.Fatal("blocked send must fail on ctx cancel")
	} else if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancel must be prompt: %v", elapsed)
	}
}

// TestTransportBackpressureDropOldest: DropOldestPolicy sheds the oldest
// AUDIO frame under pressure and never sheds control frames.
func TestTransportBackpressureDropOldest(t *testing.T) {
	tr := NewBoundedTransport(2, DropOldestPolicy)
	ctx := context.Background()
	_ = tr.Send(ctx, audioFrame(0))
	_ = tr.Send(ctx, audioFrame(1))
	// Full: sending frame 2 drops frame 0 (oldest audio).
	if err := tr.Send(ctx, audioFrame(2)); err != nil {
		t.Fatal(err)
	}
	if tr.Dropped() != 1 {
		t.Fatalf("expected 1 dropped frame, got %d", tr.Dropped())
	}
	// Fill with control frames only: dropping is refused (controls are
	// never shed).
	ctl := NewBoundedTransport(1, DropOldestPolicy)
	_ = ctl.Send(ctx, Frame{Sequence: 0, Kind: FrameControl, Control: ControlInterrupt})
	if err := ctl.Send(ctx, Frame{Sequence: 1, Kind: FrameControl, Control: ControlPause}); err != ErrTransportFull {
		t.Fatalf("all-control buffer must refuse rather than drop, got %v", err)
	}
}

// TestReorderBuffer: in-window gaps are repaired in order; a gap beyond the
// window emits a Skip and resynchronises; duplicates are ignored.
func TestReorderBuffer(t *testing.T) {
	r := NewReorderBuffer(4, 0)

	// In-order frame emits immediately.
	out := r.Push(audioFrame(0))
	if len(out) != 1 || out[0].Frame.Sequence != 0 || out[0].IsSkip {
		t.Fatalf("seq0 must emit: %+v", out)
	}
	// Out-of-order arrival held, then gap closes and both emit in order.
	out = r.Push(audioFrame(2))
	if len(out) != 0 {
		t.Fatalf("seq2 must be held: %+v", out)
	}
	out = r.Push(audioFrame(1))
	if len(out) != 2 || out[0].Frame.Sequence != 1 || out[1].Frame.Sequence != 2 {
		t.Fatalf("gap close must emit 1 then 2: %+v", out)
	}
	// Duplicate ignored.
	if out = r.Push(audioFrame(1)); len(out) != 0 {
		t.Fatalf("duplicate must be ignored: %+v", out)
	}
	// Loss beyond the window: seq 3..6 lost, 7 arrives — one Skip declaring
	// the first lost sequence (3), then the held 7 emits after the jump.
	out = r.Push(audioFrame(7))
	if len(out) != 2 || !out[0].IsSkip || out[0].Skip != 3 {
		t.Fatalf("window-exceeded gap must declare a skip from 3: %+v", out)
	}
	if out[1].IsSkip || out[1].Frame.Sequence != 7 {
		t.Fatalf("held 7 must emit right after the skip: %+v", out)
	}
	// Resynchronised: subsequent frames flow in order again.
	out = r.Push(audioFrame(8))
	if len(out) != 1 || out[0].Frame.Sequence != 8 {
		t.Fatalf("post-skip order broken: %+v", out)
	}
}
