// Command plugin_demo demonstrates the Plugin system: an "echo" plugin
// registers a tool through the three-phase lifecycle (Init → Register →
// Shutdown), driven by a YAML config. The framework side collects plugin tools
// via a Registrar callback and invokes them.
//
// Run:
//
//	go run ./examples/plugin_demo/
package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/plugin"

	"github.com/linkerlin/agentscope.go/examples/plugin_demo/echoplugin"
)

func main() {
	// 1. Build the plugin manager, loaded from a YAML config.
	mgr := plugin.NewManager()
	mgr.RegisterFactory(echoplugin.Name, func() plugin.Plugin {
		return echoplugin.New("[echo] ")
	})

	cfgPath := filepath.Join("examples", "plugin_demo", "plugins.yaml")
	cfg, err := plugin.LoadConfigFile(cfgPath)
	if err != nil {
		fmt.Printf("无法加载 %s：%v（使用默认配置）\n", cfgPath, err)
		mgr.Add(echoplugin.New("[echo] "))
	} else {
		if err := mgr.LoadConfig(*cfg); err != nil {
			panic(err)
		}
		if err := mgr.InitAll(); err != nil {
			panic(err)
		}
	}
	fmt.Printf("已加载 %d 个插件\n", mgr.Count())
	for _, mp := range mgr.Plugins() {
		fmt.Printf("  - %-14s (enabled=%v)\n", mp.Plugin.Name(), mp.Config.Enabled)
	}

	// 2. Framework side: a registrar whose tool callback tracks registered names.
	reg := plugin.NewRegistrar()
	reg.AddToolRegistrar(func(registerTool func(name string, tool any) error) error {
		return nil // framework would bridge this into a toolkit
	})

	// 3. Register all plugin extension points.
	if err := mgr.RegisterAll(reg); err != nil {
		panic(err)
	}
	fmt.Printf("插件注册的工具：%v\n", reg.RegisteredTools())

	// 4. Execute the plugin-contributed tool (direct instantiation mirrors
	// what the plugin registered — its Register step built the same EchoTool).
	t := echoplugin.New("[echo] ").Tool()
	resp, err := t.Execute(context.Background(), map[string]any{"text": "plugin demo"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("\necho 工具输出：%q\n", resp.GetTextContent())

	// 5. Graceful shutdown.
	if err := mgr.ShutdownAll(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("插件已优雅关闭")
}
