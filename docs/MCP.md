# MCP 集成

AgentScope.Go 通过 `toolkit/mcp/` 包接入 Model Context Protocol (MCP) 生态，
让 Agent 调用任意 MCP server 暴露的工具（文件系统、浏览器、GitHub、搜索…）。

## 快速接入（声明式 YAML）

```yaml
# mcp-servers.yaml
servers:
  - name: filesystem
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "$WORKDIR"]
  - name: github
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "$GITHUB_TOKEN"
```

```go
specs, _ := mcp.LoadSpecsFromYAML("mcp-servers.yaml")
mgr, results := mcp.ConnectServers(ctx, specs)   // 未安装的 server 会被跳过
defer mcp.CloseManager(mgr)

tools, _ := mgr.Tools(ctx)   // 所有已连接 server 的工具，已适配为 tool.Tool
tk := toolkit.NewToolkit()
for _, t := range tools {
    tk.Register(t)
}
```

## 传输方式

| Transport | 场景 | 配置 |
|-----------|------|------|
| `stdio` | 本地子进程（最常用） | `command` + `args` + 可选 `env` |
| `sse` | 远程 SSE MCP server | `url` |
| `http` | 远程 Streamable HTTP MCP server | `url` + 可选 `headers` |

## 内置常用 Server 目录

`mcp.CommonServers` 提供常用 MCP server 的配置模板，直接复制即可：

| 名称 | 说明 | 命令 |
|------|------|------|
| `filesystem` | 文件系统读写 | `npx -y @modelcontextprotocol/server-filesystem $WORKDIR` |
| `fetch` | 网页抓取 | `npx -y @modelcontextprotocol/server-fetch` |
| `playwright` | 浏览器自动化 | `npx @playwright/mcp@latest` |
| `github` | GitHub 操作（需 token） | `npx -y @modelcontextprotocol/server-github` |
| `sqlite` | SQLite 数据库（需 uvx） | `uvx mcp-server-sqlite --db-path $DB_PATH` |
| `brave-search` | Brave 搜索（需 key） | `npx -y @modelcontextprotocol/server-brave-search` |

## 环境变量展开

配置中的 `$VAR` / `${VAR}` 会自动用 `os.ExpandEnv` 展开，无需硬编码密钥：

```yaml
env:
  GITHUB_PERSONAL_ACCESS_TOKEN: "$GITHUB_TOKEN"   # 运行时从环境读取
args: ["-y", "server", "$WORKDIR"]                 # 路径也可引用
```

## 弹性连接

`ConnectServers` 对每个 server 独立连接：
- 二进制未安装（`exec.LookPath` 失败）→ 记录错误并跳过
- 连接/初始化失败 → 关闭该 client 并跳过
- 其他 server 不受影响

这意味着 **一个可选 server 缺失永远不会破坏整个 Agent**。

## 工具命名

MCP 工具自动命名为 `mcp__{server}__{tool}`（PyV2 兼容），避免多 server 间命名冲突：
- `mcp__filesystem__read_file`
- `mcp__github__create_issue`

只读工具（`ReadOnlyHint`）自动放行；写工具默认需用户确认（权限系统 `PermAsk`）。

## 编程式接入

不用 YAML，直接用 ClientBuilder：

```go
c, _ := mcp.NewClientBuilder("my-server").
    StdioTransport("npx", "-y", "@modelcontextprotocol/server-filesystem", "/data").
    Build()
c.Connect(ctx, mcp.MCPConfig{Name: "my-server"})

mgr := mcp.NewManager()
mgr.Register("my-server", c)
tools, _ := mgr.Tools(ctx)
```

## 运行示例

```bash
cd examples/mcp_servers
go run .                        # 用内置 CommonServers 目录
go run . ./mcp-servers.yaml     # 用自定义配置
```

输出每个 server 的连接状态 + 暴露的工具列表。
