// examples/memory_facade demonstrates the single-entry memory stack (20.8):
// window, ReMe, agentic and long-term tiers assemble from memory.NewFacade
// alone — the example never touches the internal tier constructors.
//
// Run:
//
//	go run ./examples/memory_facade
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

func main() {
	base, err := os.MkdirTemp("", "memory-facade-*")
	if err != nil {
		fmt.Println("tempdir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(base)

	// Single entry: every tier is optional (nil options = tier off).
	// LongTerm needs a vector-backed ReMe tier (ReMeOptions with Store+Embed);
	// this local demo uses the file-backed ReMe, so the long-term tier stays off.
	f, err := memory.NewFacade(memory.FacadeOptions{
		Window:  &memory.WindowOptions{MaxMessages: 5},
		ReMe:    &memory.ReMeOptions{WorkingDir: filepath.Join(base, "reme")},
		Agentic: &memory.AgenticOptions{Dir: filepath.Join(base, "agentic")},
	})
	if err != nil {
		fmt.Println("facade:", err)
		os.Exit(1)
	}

	// In-conversation memory: attach to the agent as its memory.
	win := f.Window()
	_ = win.Add(message.NewMsg().Role(message.RoleUser).TextContent("你好，帮我记住这个演示").Build())
	_ = win.Add(message.NewMsg().Role(message.RoleAssistant).TextContent("好的，已记下。").Build())

	// Cross-session retrieval memory + its hook: attach the hook to the agent.
	fmt.Printf("hooks to attach: %d (ReMe)\n", len(f.Hooks()))

	// Agent-managed markdown memory + (when enabled) long-term memory:
	// attach the middleware chain to the agent.
	fmt.Printf("middlewares to attach: %d (Agentic)\n", len(f.Middlewares()))

	// The agentic tier ensured the markdown layout on build.
	entries, _ := os.ReadDir(filepath.Join(base, "agentic"))
	for _, e := range entries {
		fmt.Println("agentic layout:", e.Name())
	}

	fmt.Println("memory stack assembled from one entry")
}
