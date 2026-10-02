// realtime/transport.go — the session transport layer (19.2): sequenced
// frames (audio + control) with reorder and explicit backpressure policy.
// The transport is the "wire" between a realtime backend adapter and the
// local playout/VAD pipeline; everything here is deterministic and testable
// without sockets.
package realtime

import (
	"context"
	"errors"
	"sync"
)

// FrameKind discriminates transport frames.
type FrameKind string

const (
	FrameAudio   FrameKind = "audio"
	FrameControl FrameKind = "control"
)

// ControlKind is one control-frame command.
type ControlKind string

const (
	ControlInterrupt ControlKind = "interrupt"
	ControlPause     ControlKind = "pause"
	ControlResume    ControlKind = "resume"
)

// Frame is one sequenced unit on the transport. Audio frames carry raw
// bytes in the negotiated format; control frames carry a command. Sequence
// is per-direction monotonic.
type Frame struct {
	Sequence int64
	Kind     FrameKind
	Audio    []byte
	Control  ControlKind
}

// BackpressurePolicy selects what a full bounded transport does.
type BackpressurePolicy string

const (
	// BlockPolicy waits for buffer space (cancellable via Send's ctx).
	BlockPolicy BackpressurePolicy = "block"
	// DropOldestPolicy discards the oldest AUDIO frame (control frames are
	// never dropped — an interrupt that gets dropped defeats its purpose).
	DropOldestPolicy BackpressurePolicy = "drop-oldest"
	// FailPolicy refuses the send (ErrTransportFull).
	FailPolicy BackpressurePolicy = "fail"
)

// ErrTransportFull is returned under FailPolicy when the buffer is full.
var ErrTransportFull = errors.New("realtime transport: buffer full")

// ErrTransportClosed is returned after Close.
var ErrTransportClosed = errors.New("realtime transport: closed")

// BoundedTransport is an in-memory transport with a bounded frame buffer
// and an explicit backpressure policy. Safe for concurrent use.
type BoundedTransport struct {
	mu      sync.Mutex
	cond    *sync.Cond
	buffer  []Frame
	cap     int
	policy  BackpressurePolicy
	recv    chan Frame
	closed  bool
	dropped int // audio frames dropped (DropOldestPolicy) — observable
}

// NewBoundedTransport builds a transport holding up to capacity frames.
func NewBoundedTransport(capacity int, policy BackpressurePolicy) *BoundedTransport {
	if capacity <= 0 {
		capacity = 32
	}
	t := &BoundedTransport{cap: capacity, policy: policy, recv: make(chan Frame, capacity)}
	t.cond = sync.NewCond(&t.mu)
	return t
}

// Send queues one frame under the transport's backpressure policy.
func (t *BoundedTransport) Send(ctx context.Context, f Frame) error {
	for {
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return ErrTransportClosed
		}
		if len(t.buffer) < t.cap {
			t.buffer = append(t.buffer, f)
			t.mu.Unlock()
			t.cond.Signal()
			return nil
		}
		switch t.policy {
		case DropOldestPolicy:
			// Drop the oldest AUDIO frame; if the buffer is all control
			// frames, refuse rather than drop a control.
			dropIdx := -1
			for i, old := range t.buffer {
				if old.Kind == FrameAudio {
					dropIdx = i
					break
				}
			}
			if dropIdx < 0 {
				t.mu.Unlock()
				return ErrTransportFull
			}
			t.buffer = append(t.buffer[:dropIdx], t.buffer[dropIdx+1:]...)
			t.dropped++
			t.buffer = append(t.buffer, f)
			t.mu.Unlock()
			t.cond.Signal()
			return nil
		case FailPolicy:
			t.mu.Unlock()
			return ErrTransportFull
		default: // BlockPolicy
			t.mu.Unlock()
			// Wake on space or close; also poll ctx cancellation.
			stop := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					t.cond.Broadcast()
				case <-stop:
				}
			}()
			t.mu.Lock()
			t.cond.Wait()
			t.mu.Unlock()
			close(stop)
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
}

// Recv is the receive stream. Frames arrive in send order; the channel
// closes on Close. One consumer drains it (multi-consumer races are the
// caller's problem, as with any Go channel).
func (t *BoundedTransport) Recv() <-chan Frame { return t.recv }

// pump moves buffered frames into the recv channel whenever a receiver is
// ready; launched by Start.
func (t *BoundedTransport) Start() *BoundedTransport {
	go func() {
		for {
			t.mu.Lock()
			for len(t.buffer) == 0 && !t.closed {
				t.cond.Wait()
			}
			if len(t.buffer) == 0 && t.closed {
				t.mu.Unlock()
				close(t.recv)
				return
			}
			f := t.buffer[0]
			t.buffer = t.buffer[1:]
			t.cond.Signal() // space opened for a blocked sender
			t.mu.Unlock()
			t.recv <- f
		}
	}()
	return t
}

// Dropped reports how many audio frames were dropped (DropOldestPolicy).
func (t *BoundedTransport) Dropped() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dropped
}

// Close stops the transport; Recv's channel closes after draining.
func (t *BoundedTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	t.cond.Broadcast()
	return nil
}

// ReorderBuffer repairs out-of-order arrival on the receive side (19.2
// acceptance: 乱序可测). Frames inside the window are emitted in sequence
// order; a gap wider than the window emits a Skip marker (declared loss)
// and resynchronises — the pipeline never stalls on a lost frame.
type ReorderBuffer struct {
	window int
	mu     sync.Mutex
	next   int64
	held   map[int64]Frame
}

// ReorderResult is one emit: either a repaired Frame, or a Skip that
// declares lost sequences resynchronised past.
type ReorderResult struct {
	Frame  Frame
	Skip   int64 // first lost sequence when Frame is zero
	IsSkip bool
}

// NewReorderBuffer starts sequence numbering at first.
func NewReorderBuffer(window int, first int64) *ReorderBuffer {
	if window <= 0 {
		window = 8
	}
	return &ReorderBuffer{window: window, next: first, held: map[int64]Frame{}}
}

// Push feeds one arrived frame; results are the frames (or skips) that
// became emittable, in order.
func (r *ReorderBuffer) Push(f Frame) []ReorderResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ReorderResult
	if f.Sequence < r.next {
		// Duplicate / ancient: ignore.
		return nil
	}
	r.held[f.Sequence] = f
	for {
		if nf, ok := r.held[r.next]; ok {
			delete(r.held, r.next)
			r.next++
			out = append(out, ReorderResult{Frame: nf})
			continue
		}
		// next is missing: hold unless the earliest HELD frame has fallen
		// outside the window (minHeld is the smallest held sequence, not
		// next — the gap being measured is next..minHeld).
		var minHeld int64 = -1
		for s := range r.held {
			if minHeld < 0 || s < minHeld {
				minHeld = s
			}
		}
		if minHeld >= 0 && minHeld-r.next >= int64(r.window) {
			// Declare the lost range (from next) and jump to the earliest
			// held frame.
			out = append(out, ReorderResult{Skip: r.next, IsSkip: true})
			r.next = minHeld
			continue
		}
		return out
	}
}
