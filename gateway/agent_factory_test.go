package gateway

import (
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
)

func TestParseModelID(t *testing.T) {
	cases := []struct {
		modelID, credProvider, wantProvider, wantModel string
	}{
		{"openai/gpt-4", "", "openai", "gpt-4"},
		{"anthropic:claude-3", "", "anthropic", "claude-3"},
		{"gpt-4", "openai", "openai", "gpt-4"},
		{"deepseek/deepseek-chat", "", "deepseek", "deepseek-chat"},
	}
	for _, c := range cases {
		p, m := parseModelID(c.modelID, c.credProvider)
		if p != c.wantProvider || m != c.wantModel {
			t.Fatalf("parseModelID(%q, %q) = (%q, %q), want (%q, %q)",
				c.modelID, c.credProvider, p, m, c.wantProvider, c.wantModel)
		}
	}
}

func TestAgentFactory_Build_MissingConfig(t *testing.T) {
	f := NewAgentFactory(nil)
	_, err := f.Build(nil, &service.Credential{})
	if err == nil || err.Error() != "agent_factory: agent config is nil" {
		t.Fatalf("expected nil config error, got: %v", err)
	}
}

func TestAgentFactory_Build_MissingCredential(t *testing.T) {
	f := NewAgentFactory(nil)
	_, err := f.Build(&service.AgentConfig{ModelID: "openai/gpt-4"}, nil)
	if err == nil || err.Error() != "agent_factory: credential is nil" {
		t.Fatalf("expected nil credential error, got: %v", err)
	}
}

func TestAgentFactory_Build_UnknownProvider(t *testing.T) {
	f := NewAgentFactory(nil)
	_, err := f.Build(
		&service.AgentConfig{ModelID: "unknown/model"},
		&service.Credential{Provider: "unknown", Encrypted: "key"},
	)
	if err == nil || err.Error() != `agent_factory: unsupported provider "unknown"` {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}

func TestAgentFactory_RegisterProvider(t *testing.T) {
	f := NewAgentFactory(nil)
	f.RegisterProvider("custom", func(key, name, url string) (model.ChatModel, error) {
		return nil, nil
	})
	// Just verify registration does not panic and builder is callable.
	if _, ok := f.modelBuilders["custom"]; !ok {
		t.Fatal("expected custom provider to be registered")
	}
}

func TestAgentFactory_DefaultAgentClassRegistered(t *testing.T) {
	f := NewAgentFactory(nil)
	classes := f.RegisteredAgentClasses()
	found := false
	for _, c := range classes {
		if c == "react" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected default 'react' agent class, got %v", classes)
	}
}

func TestAgentFactory_RegisterAgentClass(t *testing.T) {
	f := NewAgentFactory(nil)
	mock := &smMockAgent{}
	f.RegisterAgentClass("noop", func(cfg *service.AgentConfig, chatModel model.ChatModel) (agent.Agent, error) {
		return mock, nil
	})

	// Custom class is used.
	got, err := f.buildAgentForClass(&service.AgentConfig{AgentClass: "noop"}, nil)
	if err != nil || got != mock {
		t.Fatalf("expected custom class builder to be used, got %v err=%v", got, err)
	}

	// Unknown class errors clearly.
	if _, err := f.buildAgentForClass(&service.AgentConfig{AgentClass: "missing"}, nil); err == nil {
		t.Fatal("expected error for unknown agent class")
	}
}

func TestAgentFactory_EmptyClassDefaultsToReact(t *testing.T) {
	f := NewAgentFactory(nil)
	sentinel := &smMockAgent{}
	// Override the default "react" builder with a sentinel to prove "" resolves to "react".
	f.RegisterAgentClass("react", func(cfg *service.AgentConfig, chatModel model.ChatModel) (agent.Agent, error) {
		return sentinel, nil
	})
	got, err := f.buildAgentForClass(&service.AgentConfig{AgentClass: ""}, nil)
	if err != nil || got != sentinel {
		t.Fatalf("expected empty AgentClass to resolve to 'react', got %v err=%v", got, err)
	}
}
