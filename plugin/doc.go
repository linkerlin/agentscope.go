// Package plugin provides a plugin system for extending AgentScope.Go
// without modifying core code.
//
// # Overview
//
// A Plugin is a self-contained module that registers one or more extension
// points (Models, Tools, Memories, Hooks, Middleware, Formatters) into the
// framework at runtime. Plugins follow a strict lifecycle:
//
//  1. Init(config) — initialize with plugin-specific configuration
//  2. Register(registrar) — register extension points
//  3. Shutdown() — cleanup resources
//
// # Usage
//
//	// Create a plugin manager
//	mgr := plugin.NewManager()
//
//	// Register plugins programmatically
//	mgr.Add(&MyPlugin{})
//
//	// Or load from YAML config
//	mgr.LoadConfigFile("plugins.yaml")
//
//	// Initialize all plugins
//	mgr.InitAll()
//
//	// Register all plugins into the framework
//	registrar := mgr.Registrar()
//	mgr.RegisterAll(registrar)
//
//	// ... use the framework ...
//
//	// Shutdown
//	mgr.ShutdownAll()
//
// # Writing a Plugin
//
//	type MyToolPlugin struct {
//		// fields
//	}
//
//	func (p *MyToolPlugin) Name() string { return "my-tool" }
//
//	func (p *MyToolPlugin) Init(cfg plugin.PluginConfig) error {
//		// parse config
//		return nil
//	}
//
//	func (p *MyToolPlugin) Register(r *plugin.Registrar) error {
//		r.Tools().Register(myTool)
//		return nil
//	}
//
//	func (p *MyToolPlugin) Shutdown() error { return nil }
package plugin
