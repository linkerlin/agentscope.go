//go:build linux

package plugin

import (
	"fmt"
	"plugin"
)

// SymbolName is the exported symbol name that .so plugins must expose.
// The symbol must be of type `plugin.Plugin`.
const SymbolName = "PluginInstance"

// PluginSymbol is the interface that a .so plugin must satisfy.
// The .so file must export a variable named `PluginInstance` of type `plugin.Plugin`.
//
// Example .so plugin:
//
//	package main
//
//	import "github.com/linkerlin/agentscope.go/plugin"
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Name() string { return "my-plugin" }
//	// ... implement Plugin interface ...
//
//	var PluginInstance plugin.Plugin = &MyPlugin{}
//	func main() {}
//
// Build: go build -buildmode=plugin -o myplugin.so myplugin.go
type PluginSymbol = Plugin

// LoadSO loads a Go plugin from a .so file on Linux.
// The .so file must export a symbol named "PluginInstance" implementing the Plugin interface.
func LoadSO(path string) (Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open plugin %q: %w", path, err)
	}

	sym, err := p.Lookup(SymbolName)
	if err != nil {
		return nil, fmt.Errorf("lookup %q in plugin %q: %w", SymbolName, path, err)
	}

	pluginInst, ok := sym.(*PluginSymbol)
	if !ok {
		// Try as interface directly
		pi, ok := sym.(Plugin)
		if !ok {
			return nil, fmt.Errorf("symbol %q in %q does not implement plugin.Plugin interface", SymbolName, path)
		}
		return pi, nil
	}

	return *pluginInst, nil
}
