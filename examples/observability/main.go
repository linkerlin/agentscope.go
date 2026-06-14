// examples/observability/main.go
//
// Demo: Event bus + LangSmith observer forwarding.
//
// This demo shows how to create an event.Bus, publish agent events, and
// forward them to a LangSmith client (stub). No real API key is needed.
//
// How to run:
//   cd examples/observability && go run main.go

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/observability"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Create an event bus with a per-subscriber buffer of 64.
	bus := event.NewBus(64)

	// 2. Create a LangSmith client (stub — no real network calls in this demo).
	client := observability.NewLangSmithClient("demo-key").WithProject("demo-project")

	// 3. Create an observer that forwards bus events to LangSmith as Runs.
	observer := observability.NewLangSmithObserver(client, "demo-project", "session-1")

	// 4. Subscribe a simple printer to verify events flow through the bus.
	id, ch, done := bus.Subscribe()
	defer bus.Unsubscribe(id)

	go func() {
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				fmt.Printf("[bus] received event type=%T reply_id=%s\n", ev, ev.ReplyID())
			case <-done:
				return
			}
		}
	}()

	// 5. Publish a few synthetic events using constructors.
	bus.Publish(event.NewReplyStart("r1", "agent-a"))
	bus.Publish(event.NewTextBlockDelta("r1", 0, "hello"))
	bus.Publish(event.NewReplyEnd("r1", "agent-a"))

	// 6. Run the observer (it blocks consuming from the bus until ctx is done).
	fmt.Println("starting observer forwarding...")
	observer.Observe(ctx, bus)
	fmt.Println("observer stopped")
}
