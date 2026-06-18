// Example: agent team — custom subagent templates + permission inheritance
// (mirrors Python agentscope #1833 / #1815).
//
// Demonstrates the gateway AgentFactory spawning each declared SubagentTemplate
// as a SubagentTool on the leader's toolkit. Spawned subagents inherit the
// leader's permission engine and share the session workspace. Uses a mock model
// provider so it runs without any API key.
//
// Run: go run ./examples/agent_team
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/workspace"
)

type mockModel struct{ name string }

func (m *mockModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("delegated result").Build(), nil
}
func (m *mockModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, fmt.Errorf("not used")
}
func (m *mockModel) ModelName() string { return m.name }

func main() {
	factory := gateway.NewAgentFactory(nil)
	// Register a mock provider so model resolution succeeds without a key.
	factory.RegisterProvider("mock", func(key, name, url string) (model.ChatModel, error) {
		return &mockModel{name: name}, nil
	})

	dir := "/tmp/as-team-demo"
	ws := workspace.NewLocalWorkspace("sess", dir)
	sw := gateway.NewSessionWorkspace(ws, dir)

	leaderCfg := &service.AgentConfig{
		ID:           "leader",
		Name:         "leader",
		ModelID:      "mock/leader-model",
		SystemPrompt: "You are the team leader. Delegate subtasks to your subagents.",
		SubagentTemplates: []service.SubagentTemplate{
			{Name: "researcher", Description: "Research a topic.", SystemPrompt: "You research."},
			{Name: "coder", Description: "Write code.", SystemPrompt: "You code."},
		},
	}
	cred := &service.Credential{Provider: "mock", Encrypted: "key"}

	// Build the leader: templates are spawned as SubagentTools on its toolkit,
	// each inheriting the leader's permission engine (#1815).
	leader, err := factory.BuildSessionAgent(leaderCfg, cred, sw, gateway.SessionAgentDeps{PermissionMode: permission.ModeExplore})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("leader agent built:", leader.(interface{ Name() string }).Name())

	// Inspect the spawned subagent tools directly.
	permEngine := permission.NewEngine(permission.ModeExplore, nil)
	leaderModel := &mockModel{name: "leader-model"}
	tools, err := factory.BuildSubagentTools(leaderCfg, cred, leaderModel, sw, permEngine, gateway.SessionAgentDeps{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nspawned %d subagent tool(s):\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  - %s: %s\n", t.Name(), t.Description())
	}

	// Demonstrate delegation: invoking the "researcher" subagent tool runs the
	// child agent (which uses the mock model and inherits the leader context).
	for _, t := range tools {
		if t.Name() == "researcher" {
			resp, err := t.Execute(context.Background(), map[string]any{"query": "summarize Go generics"})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("\nresearcher delegation -> %s\n", resp.GetTextContent())
		}
	}
}
