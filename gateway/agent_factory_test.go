package gateway

import (
	"testing"

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
