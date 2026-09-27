package gateway

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// sharedAgent 是一个指针型有状态 agent：并发进入 ReplyStream 时用
// atomic 计数暴露串扰（22.4 修复前 >1）。
type sharedStatefulAgent struct {
	mockV2Agent
	inFlight atomic.Int32
	peak     atomic.Int32
}

func (a *sharedStatefulAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	n := a.inFlight.Add(1)
	for {
		p := a.peak.Load()
		if n <= p || a.peak.CompareAndSwap(p, n) {
			break
		}
	}
	defer a.inFlight.Add(-1)

	ch := make(chan event.AgentEvent, 8)
	go func() {
		defer close(ch)
		time.Sleep(30 * time.Millisecond)
		replyID := "r-" + time.Now().Format("150405.000000000")
		ch <- event.NewTextBlockDelta(replyID, 0, "ok")
		ch <- event.NewReplyEnd(replyID, "shared")
	}()
	return ch, nil
}

// TestSessionManager_SharedAgentSerializedAcrossSessions locks the 22.4
// rule: two sessions resolved to the SAME stateful agent instance must never
// run it concurrently — the manager serializes execution on the agent for
// the whole turn (the "equivalent serialization guarantee").
func TestSessionManager_SharedAgentSerializedAcrossSessions(t *testing.T) {
	sm := NewSessionManager()
	shared := &sharedStatefulAgent{}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sess := "sess-serial-" + string(rune('a'+i))
			ch, err := sm.Run(context.Background(), sess, shared, turnMsg())
			if err != nil {
				t.Errorf("run %d: %v", i, err)
				return
			}
			for range ch {
			}
		}(i)
	}
	wg.Wait()

	if got := shared.peak.Load(); got > 1 {
		t.Fatalf("shared agent ran concurrently: peak in-flight %d > 1", got)
	}
}

// TestSessionManager_DistinctAgentsStillParallel verifies the agent lock
// does not over-serialize: two different agent instances on two sessions
// still execute in parallel.
func TestSessionManager_DistinctAgentsStillParallel(t *testing.T) {
	sm := NewSessionManager()
	a1 := &sharedStatefulAgent{}
	a2 := &sharedStatefulAgent{}

	start := time.Now()
	var wg sync.WaitGroup
	for _, tc := range []struct {
		sess  string
		agent *sharedStatefulAgent
	}{{"s1", a1}, {"s2", a2}} {
		wg.Add(1)
		go func(sess string, ag *sharedStatefulAgent) {
			defer wg.Done()
			ch, err := sm.Run(context.Background(), sess, ag, turnMsg())
			if err != nil {
				t.Errorf("run %s: %v", sess, err)
				return
			}
			for range ch {
			}
		}(tc.sess, tc.agent)
	}
	wg.Wait()

	// Each turn sleeps 30ms; serialized they would take >=60ms.
	if elapsed := time.Since(start); elapsed >= 55*time.Millisecond {
		t.Fatalf("distinct agents were serialized: %v", elapsed)
	}
}

// TestSessionManager_MultiSubscriberStress drives the -race check the 22.4
// acceptance asks for: many subscribers joining and reading the same session
// concurrently must not race, and every subscriber sees the same ordered
// event sequence.
func TestSessionManager_MultiSubscriberStress(t *testing.T) {
	sm := NewSessionManager()
	shared := &sharedStatefulAgent{}

	ch, err := sm.Run(context.Background(), "sess-stress", shared, turnMsg())
	if err != nil {
		t.Fatal(err)
	}
	first := ch

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Joining mid-run replays the buffered history in order.
			sub := sm.Subscribe("sess-stress")
			var last string
			for ev := range sub {
				if ev == nil {
					continue
				}
				d := describeEvent(ev)
				if d != "" && d < last {
					t.Errorf("out-of-order replay: %q after %q", d, last)
					return
				}
				last = maxStr(last, d)
			}
		}()
	}
	for ev := range first {
		_ = ev
	}
	wg.Wait()
}

func describeEvent(ev event.AgentEvent) string {
	switch ev.EventType() {
	case event.TypeTextBlockDelta:
		return "1delta"
	case event.TypeReplyEnd:
		return "2end"
	}
	return ""
}

func maxStr(a, b string) string {
	if a >= b {
		return a
	}
	return b
}
