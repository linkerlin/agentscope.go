// Package echoplugin demonstrates the Plugin system: a plugin with the
// Init → Register → Shutdown lifecycle that registers an "echo" tool into the
// framework via the Registrar.
package echoplugin

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/plugin"
	"github.com/linkerlin/agentscope.go/tool"
)

// Name is the plugin identifier.
const Name = "echo-plugin"

// EchoTool is the tool the plugin contributes to the framework.
type EchoTool struct {
	// Prefix prepends every echoed message (configurable via plugin config).
	Prefix string
}

// Name returns the tool name.
func (e *EchoTool) Name() string { return "echo" }

// Description returns the tool description.
func (e *EchoTool) Description() string {
	return "Echo back the input text (demo plugin tool)."
}

// Spec returns the tool JSON schema.
func (e *EchoTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        e.Name(),
		Description: e.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "Text to echo"},
			},
			"required": []string{"text"},
		},
	}
}

// Execute echoes the input back with the configured prefix.
func (e *EchoTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	text, _ := input["text"].(string)
	if text == "" {
		return nil, nil
	}
	return tool.NewTextResponse(e.Prefix + text), nil
}

// echoConfig is the plugin's configuration section (parsed from YAML).
type echoConfig struct {
	Prefix string `yaml:"prefix"`
}

// Plugin implements the plugin.Plugin three-phase lifecycle.
type Plugin struct {
	prefix string
	tool   *EchoTool
}

// New creates an echo plugin with the given default prefix.
func New(prefix string) *Plugin {
	return &Plugin{prefix: prefix}
}

// Tool returns the echo tool the plugin registers (for demo/inspection).
func (p *Plugin) Tool() *EchoTool {
	return &EchoTool{Prefix: p.prefix}
}

// Name returns "echo-plugin".
func (p *Plugin) Name() string { return Name }

// Init stores the configured prefix (mirroring plugin config wiring).
func (p *Plugin) Init(cfg plugin.PluginConfig) error {
	if raw, ok := cfg.Params["prefix"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			p.prefix = s
		}
	}
	return nil
}

// Register contributes the echo tool to the framework registrar.
func (p *Plugin) Register(r *plugin.Registrar) error {
	p.tool = &EchoTool{Prefix: p.prefix}
	return r.RegisterTool(p.tool.Name(), p.tool)
}

// Shutdown releases the tool reference (no external resources here).
func (p *Plugin) Shutdown() error {
	p.tool = nil
	return nil
}
