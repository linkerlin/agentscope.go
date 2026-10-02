// realtime/playout.go — the playout queue and its confirmed-position clock
// (19.2). The core idea: playback progress is only real once the audio
// device has CONFIRMED consuming it — that confirmed position is the
// PlayoutPosition, and on a barge-in the cut happens exactly there: the
// played-out prefix cannot be taken back, everything after it is dropped.
package realtime

import "sync"

// PlayoutPosition is where playback has actually reached: the chunk
// sequence plus the byte offset within it.
type PlayoutPosition struct {
	Sequence int
	Offset   int
}

type playoutChunk struct {
	sequence int
	data     []byte
	played   int // bytes confirmed consumed
}

// Playout is the confirmed-consumption clock plus the pending queue.
// Enqueue mirrors the backend's AudioOutDelta stream; Advance is what the
// audio device callback calls as it consumes bytes; Truncate is what an
// interrupt calls for TruncationClient cards (19.1).
type Playout struct {
	mu     sync.Mutex
	chunks []playoutChunk
	// lastEnd is the confirmed end of the most recent fully-consumed
	// chunk; the position reported when the queue runs dry.
	lastEnd PlayoutPosition
	dropped int // bytes dropped by Truncate calls — observable
}

// Enqueue appends one audio chunk (sequence order is the caller's contract
// with the event stream).
func (p *Playout) Enqueue(sequence int, audio []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chunks = append(p.chunks, playoutChunk{sequence: sequence, data: audio})
}

// Advance confirms n bytes consumed by the playback device, crossing chunk
// boundaries, and returns the new confirmed PlayoutPosition. Fully played
// chunks are retired. Confirming more than is queued clamps to what exists
// (a device overrun never fabricates positions).
func (p *Playout) Advance(n int) PlayoutPosition {
	p.mu.Lock()
	defer p.mu.Unlock()
	remaining := n
	for remaining > 0 && len(p.chunks) > 0 {
		c := &p.chunks[0]
		avail := len(c.data) - c.played
		if remaining < avail {
			c.played += remaining
			return PlayoutPosition{Sequence: c.sequence, Offset: c.played}
		}
		remaining -= avail
		// Retire: the whole chunk is confirmed; remember its end.
		p.lastEnd = PlayoutPosition{Sequence: c.sequence, Offset: len(c.data)}
		p.chunks = p.chunks[1:]
	}
	if len(p.chunks) > 0 {
		// Confirmation stopped exactly at a chunk boundary: the position is
		// the start of the next pending chunk.
		return PlayoutPosition{Sequence: p.chunks[0].sequence, Offset: 0}
	}
	return p.lastEnd
}

// Position returns the currently confirmed position without consuming.
func (p *Playout) Position() PlayoutPosition {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.chunks) == 0 {
		return p.lastEnd
	}
	c := p.chunks[0]
	return PlayoutPosition{Sequence: c.sequence, Offset: c.played}
}

// Truncate drops everything after the confirmed position (the interrupt
// cut for TruncationClient cards) and reports how many pending bytes were
// dropped. The confirmed position itself stands — played audio cannot be
// unplayed.
func (p *Playout) Truncate() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Freeze the confirmed position FIRST: the partially-played head chunk
	// (if any) ends at its played offset — that is the final confirmed
	// position of this turn, and Position() must keep reporting it after
	// the queue is gone.
	if len(p.chunks) > 0 {
		head := p.chunks[0]
		if head.played > 0 {
			p.lastEnd = PlayoutPosition{Sequence: head.sequence, Offset: head.played}
		}
	}
	dropped := 0
	for _, c := range p.chunks {
		dropped += len(c.data) - c.played
	}
	p.chunks = nil
	p.dropped += dropped
	return dropped
}

// Pending reports queued-but-unconfirmed bytes.
func (p *Playout) Pending() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, c := range p.chunks {
		total += len(c.data) - c.played
	}
	return total
}

// Dropped reports bytes dropped across all Truncate calls.
func (p *Playout) Dropped() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dropped
}
