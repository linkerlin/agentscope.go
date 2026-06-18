// Example: message bus (mirrors Python agentscope's message_bus refactor #1849)
// for cross-process / distributed coordination.
//
// Demonstrates an in-process LocalBus and a Redis-backed RedisBus (using an
// embedded miniredis, so no external Redis is required). The same pattern
// scales to real multiprocess deployments by pointing RedisBus at a shared
// Redis instance.
//
// Run: go run ./examples/messagebus
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkerlin/agentscope.go/messagebus"
	bredis "github.com/redis/go-redis/v9"
)

func main() {
	// --- LocalBus (single process) ---
	local := messagebus.NewLocalBus()
	defer local.Close()

	ch, cancel, _ := local.Subscribe(context.Background(), "task.cancel")
	_ = local.Publish(context.Background(), "task.cancel", []byte("cancel job-42"))
	m := <-ch
	fmt.Printf("[local] %s: %s\n", m.Channel, string(m.Payload))
	cancel()

	// --- RedisBus (multiprocess-ready; embedded miniredis here) ---
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatal(err)
	}
	defer mr.Close()

	client := bredis.NewClient(&bredis.Options{Addr: mr.Addr()})
	defer client.Close()

	bus := messagebus.NewRedisBus(client, "")
	rch, rcancel, err := bus.Subscribe(context.Background(), "tool.offload.done")
	if err != nil {
		log.Fatal(err)
	}
	defer rcancel()

	time.Sleep(50 * time.Millisecond) // let the subscription register
	if err := bus.Publish(context.Background(), "tool.offload.done", []byte(`{"task":"build","ok":true}`)); err != nil {
		log.Fatal(err)
	}

	rm := <-rch
	fmt.Printf("[redis] %s: %s\n", rm.Channel, string(rm.Payload))
	fmt.Println("\nmessage bus demo complete (swap miniredis for a shared Redis to coordinate multiple gateway processes)")
}
