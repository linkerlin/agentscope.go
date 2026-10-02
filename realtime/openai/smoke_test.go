// realtime/openai/smoke_test.go — opt-in live-service smoke (19.5 acceptance:
// "真实服务冒烟为 opt-in" per the phase's shared clause). Set OPENAI_API_KEY
// to run; the test asserts only the handshake (session.created →
// session.update → session.updated) and first-event liveness — protocol
// details are locked by the mock contract tests.
package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/realtime"
)

func TestOpenAIRealtimeLiveSmoke(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set (opt-in live smoke)")
	}

	model := NewGPTRealtime(key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, _, err := model.Connect(ctx, realtime.NegotiateOffer{
		Formats: []realtime.AudioFormat{pcm24k},
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
	// A quiet live session is healthy: the handshake is the smoke target.
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
			return
		}
	}
}

// TestXAIRealtimeLiveSmoke runs the same smoke against xAI's OpenAI-compatible
// realtime endpoint (19.6): XAI_API_KEY gates it.
func TestXAIRealtimeLiveSmoke(t *testing.T) {
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		t.Skip("XAI_API_KEY not set (opt-in live smoke)")
	}

	model := NewGrokRealtime(key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, _, err := model.Connect(ctx, realtime.NegotiateOffer{
		Formats: []realtime.AudioFormat{pcm24k},
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
}
