package realtime

import "testing"

// TestVADEdges locks the detector's edge contract (19.2): exactly one
// SpeechStart after sustained speech, exactly one SpeechEnd after
// sustained silence, clicks filtered, intra-word pauses bridged.
func TestVADEdges(t *testing.T) {
	cfg := DefaultVADConfig()
	v := NewVAD(cfg)
	frame := 320                   // 20ms @ 16kHz
	loud := ToneSynth(frame, 8000) // RMS well above 350
	quiet := SilenceSynth(frame)

	// Pure silence: no events ever.
	for i := 0; i < 30; i++ {
		if ev := v.Process(quiet); ev != VADNone {
			t.Fatalf("silence must not emit, got %v", ev)
		}
	}

	// A single loud click (1 frame) does not start speech.
	_ = v.Process(loud)
	for i := 0; i < 20; i++ {
		_ = v.Process(quiet)
	}
	if v.Speaking() {
		t.Fatal("a click must not trigger speech start")
	}

	// Sustained speech: start fires exactly on the MinSpeechFrames-th
	// frame, and never again while speaking.
	starts := 0
	for i := 0; i < cfg.MinSpeechFrames+10; i++ {
		if ev := v.Process(loud); ev == VADSpeechStart {
			starts++
			if i != cfg.MinSpeechFrames-1 {
				t.Fatalf("start fired on frame %d, want %d", i, cfg.MinSpeechFrames-1)
			}
		} else if ev != VADNone {
			t.Fatalf("unexpected event %v", ev)
		}
	}
	if starts != 1 {
		t.Fatalf("exactly one start, got %d", starts)
	}

	// Intra-word pause shorter than hangover: no end.
	for i := 0; i < cfg.HangoverFrames-1; i++ {
		if ev := v.Process(quiet); ev != VADNone {
			t.Fatalf("short pause must not end speech, got %v", ev)
		}
	}
	if !v.Speaking() {
		t.Fatal("hangover must bridge intra-word pauses")
	}

	// Sustained silence: end fires exactly on the HangoverFrames-th frame.
	// (The silence run already has HangoverFrames-1 from the pause test;
	// one more silent frame completes it.)
	if ev := v.Process(quiet); ev != VADSpeechEnd {
		t.Fatalf("sustained silence must end speech, got %v", ev)
	}
	// No repeated ends.
	for i := 0; i < 10; i++ {
		if ev := v.Process(quiet); ev != VADNone {
			t.Fatalf("idle silence must not emit, got %v", ev)
		}
	}
}

// TestVADLevelReporting: LastLevel tracks the frame RMS (observability).
func TestVADLevelReporting(t *testing.T) {
	v := NewVAD(DefaultVADConfig())
	_ = v.Process(SilenceSynth(320))
	if v.LastLevel() != 0 {
		t.Fatalf("silence RMS: %v", v.LastLevel())
	}
	_ = v.Process(ToneSynth(320, 8000))
	if v.LastLevel() < 350 {
		t.Fatalf("loud RMS too low: %v", v.LastLevel())
	}
}

// TestPlayoutConfirmedPosition: Advance confirms consumption across chunk
// boundaries; Position is the confirmed clock; overrun clamps.
func TestPlayoutConfirmedPosition(t *testing.T) {
	p := &Playout{}

	// Nothing confirmed yet.
	if pos := p.Position(); pos.Sequence != 0 || pos.Offset != 0 {
		t.Fatalf("initial position: %+v", pos)
	}

	p.Enqueue(0, make([]byte, 100)) // seq 0: 100 bytes
	p.Enqueue(1, make([]byte, 60))  // seq 1: 60 bytes

	// Confirm 50 bytes inside chunk 0.
	if pos := p.Advance(50); pos.Sequence != 0 || pos.Offset != 50 {
		t.Fatalf("partial chunk: %+v", pos)
	}
	if p.Pending() != 110 {
		t.Fatalf("pending: %d", p.Pending())
	}

	// Confirm 60 more: finishes chunk 0 (50 remaining) + 10 into chunk 1.
	if pos := p.Advance(60); pos.Sequence != 1 || pos.Offset != 10 {
		t.Fatalf("cross-boundary: %+v", pos)
	}

	// Overrun: confirm more than queued — clamps to the end of the last
	// chunk, never fabricates.
	if pos := p.Advance(1000); pos.Sequence != 1 || pos.Offset != 60 {
		t.Fatalf("overrun must clamp to end: %+v", pos)
	}
	if p.Pending() != 0 {
		t.Fatalf("pending after drain: %d", p.Pending())
	}
	if pos := p.Position(); pos.Sequence != 1 || pos.Offset != 60 {
		t.Fatalf("drained position: %+v", pos)
	}
}

// TestPlayoutTruncateOnInterrupt: the 19.2 acceptance — the interrupt cut
// happens at the CONFIRMED position: the pending tail is dropped, the
// confirmed prefix stands, and the position clock is unchanged by the cut.
func TestPlayoutTruncateOnInterrupt(t *testing.T) {
	p := &Playout{}
	p.Enqueue(0, make([]byte, 100))
	p.Enqueue(1, make([]byte, 100))
	p.Enqueue(2, make([]byte, 100))

	// Device confirmed 130 bytes (all of chunk 0 + 30 of chunk 1).
	confirmed := p.Advance(130)
	if confirmed.Sequence != 1 || confirmed.Offset != 30 {
		t.Fatalf("confirmed: %+v", confirmed)
	}

	// Barge-in: truncate at the confirmed position.
	dropped := p.Truncate()
	if dropped != 170 {
		t.Fatalf("truncate must drop the unconfirmed tail (70+100), got %d", dropped)
	}
	if p.Pending() != 0 {
		t.Fatalf("post-truncate pending: %d", p.Pending())
	}
	// The confirmed position stands — played audio cannot be unplayed.
	if pos := p.Position(); pos.Sequence != 1 || pos.Offset != 30 {
		t.Fatalf("confirmed position must survive the cut: %+v", pos)
	}
	if p.Dropped() != 170 {
		t.Fatalf("dropped accounting: %d", p.Dropped())
	}

	// Audio arriving AFTER the interrupt queues normally (the next turn).
	p.Enqueue(3, make([]byte, 10))
	if p.Pending() != 10 {
		t.Fatalf("post-interrupt queue: %d", p.Pending())
	}
}
