// realtime/gemini/smoke_test.go — opt-in live-service smoke (19.6 acceptance
// shares the phase's opt-in smoke clause). Set GEMINI_API_KEY to run; the
// test asserts only the setup/setupComplete handshake and stream liveness.
package gemini

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/realtime"
)

func TestGeminiLiveSmoke(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set (opt-in live smoke)")
	}

	model := NewLiveFlash(key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, _, err := model.Connect(ctx, realtime.NegotiateOffer{
		Formats: []realtime.AudioFormat{pcm16k},
	})
	if err != nil {
		t.Fatalf("live connect: %v", err)
	}
	defer sess.Close()

	select {
	case ev, ok := <-sess.Events():
		if !ok {
			t.Fatalf("stream closed before SessionStarted")
		}
		if _, isStart := ev.(realtime.SessionStarted); !isStart {
			t.Fatalf("first event: %T (%v)", ev, ev)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("no SessionStarted within 20s")
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok || realtime.Terminal(ev) {
				if e, isErr := ev.(realtime.ErrorEvent); isErr {
					t.Fatalf("live stream errored: %s", e.Err)
				}
				return
			}
		case <-deadline:
			return // quiet live stream: the handshake is the smoke target
		}
	}
}
