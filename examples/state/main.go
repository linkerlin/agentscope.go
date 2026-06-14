// examples/state/main.go
//
// Demo: JSON file-based state persistence.
//
// This demo shows how to create a JSONFileStore, save state, and load it back.
// The state is stored as one JSON file per key.
//
// How to run:
//   cd examples/state && go run main.go

package main

import (
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/state"
)

// demoState is a concrete state implementation.
type demoState struct {
	Counter int               `json:"counter"`
	Labels  []string          `json:"labels"`
	Meta    map[string]string `json:"meta"`
}

// StateType satisfies the state.State interface.
func (d *demoState) StateType() string { return "demo" }

// Validate satisfies the state.State interface.
func (d *demoState) Validate() error { return nil }

func main() {
	// 1. Create a JSON file store in a temporary directory.
	storeDir := ".cache/state_demo"
	_ = os.RemoveAll(storeDir)
	defer os.RemoveAll(storeDir)

	store, err := state.NewJSONStore(storeDir)
	if err != nil {
		fmt.Println("store error:", err)
		return
	}

	// 2. Save some state.
	s1 := &demoState{
		Counter: 42,
		Labels:  []string{"go", "agentscope"},
		Meta:    map[string]string{"region": "ap-east"},
	}
	if err := store.Save("session-1", s1); err != nil {
		fmt.Println("save error:", err)
		return
	}
	fmt.Println("saved session-1")

	// 3. Load the state back into a new value.
	var loaded demoState
	if err := store.Get("session-1", &loaded); err != nil {
		fmt.Println("load error:", err)
		return
	}
	fmt.Printf("loaded: counter=%d labels=%v meta=%v\n", loaded.Counter, loaded.Labels, loaded.Meta)

	// 4. List keys and check existence.
	fmt.Println("keys:", store.ListKeys())
	fmt.Println("exists session-1:", store.Exists("session-1"))
	fmt.Println("exists session-2:", store.Exists("session-2"))
}
