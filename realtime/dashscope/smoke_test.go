// realtime/dashscope/smoke_test.go — opt-in live-service smoke (19.4
// acceptance: "真实服务冒烟为 opt-in"). Set DASHSCOPE_API_KEY to run; the
// test asserts only the handshake and stream liveness — protocol details are
// locked by the mock contract tests, and a live drift shows up here as a
// clear handshake/first-event failure worth adapting decodeFrame for.
package dashscope

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/realtime"
)

func TestDashScopeLiveSmoke(t *testing.T) {
	key := os.Getenv("DASHSCOPE_API_KEY")
	if key == "" {
		t.Skip("DASHSCOPE_API_KEY not set (opt-in live smoke)")
	}

	model := NewQwenOmniRealtime(key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, _, err := model.Connect(ctx, realtime.NegotiateOffer{
		Formats: []realtime.AudioFormat{pcm16k},
	})
	if err != nil {
		t.Fatalf("live connect: %v", err)
	}
	defer sess.Close()

	// First event must be SessionStarted; then observe the stream briefly
	// (a live server may stay quiet until input arrives — silence is OK).
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
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				return // clean end
			}
			if realtime.Terminal(ev) {
				if e, isErr := ev.(realtime.ErrorEvent); isErr {
					t.Fatalf("live stream errored: %s", e.Err)
				}
				return
			}
		case <-deadline:
			return // quiet live stream: handshake proven, done
		}
	}
}
