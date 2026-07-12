// Command mcp_servers demonstrates declarative MCP server wiring: load a YAML
// config, connect each enabled server (gracefully skipping ones whose binary
// isn't installed), and list the tools the agent would gain.
//
// Run:
//
//	go run ./examples/mcp_servers/
//	# with a custom config:
//	go run ./examples/mcp_servers/ ./my-servers.yaml
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

func main() {
	cfgPath := "mcp-servers.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	specs, err := mcp.LoadSpecsFromYAML(cfgPath)
	if err != nil {
		fmt.Printf("无法加载 %s，改用内置 CommonServers 目录：%v\n", cfgPath, err)
		for _, key := range []string{"filesystem", "fetch", "playwright", "github"} {
			s := mcp.CommonServers[key]
			specs = append(specs, s)
		}
	}

	ctx := context.Background()
	fmt.Printf("连接 %d 个 MCP server 配置（未安装的会被跳过）…\n\n", len(specs))
	mgr, results := mcp.ConnectServers(ctx, specs)
	defer mcp.CloseManager(mgr)

	ok := 0
	for _, r := range results {
		if r.Err != nil {
			if r.Err.Error() == "disabled" {
				fmt.Printf("  ⏭  %-16s 跳过（disabled）\n", r.Spec.Name)
			} else {
				fmt.Printf("  ✗  %-16s 失败：%v\n", r.Spec.Name, r.Err)
			}
			continue
		}
		fmt.Printf("  ✓  %-16s 已连接，暴露 %d 个工具\n", r.Spec.Name, r.Tools)
		ok++
	}

	fmt.Printf("\n%d/%d server 连接成功。\n", ok, len(results))

	tools, err := mgr.Tools(ctx)
	if err != nil {
		fmt.Printf("列出工具失败：%v\n", err)
		return
	}
	if len(tools) == 0 {
		fmt.Println("\n无可用工具。安装 MCP server 后重试，例如：")
		fmt.Println("  npm install -g @modelcontextprotocol/server-filesystem")
		fmt.Println("  设置 WORKDIR 环境变量指向可访问目录")
		return
	}
	fmt.Printf("\n=== 可用 MCP 工具 (%d) ===\n", len(tools))
	for _, t := range tools {
		desc := t.Description()
		if len(desc) > 70 {
			desc = desc[:70] + "…"
		}
		fmt.Printf("  • %s\n    %s\n", t.Name(), desc)
	}
}
