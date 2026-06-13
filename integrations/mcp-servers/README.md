# MCP Server 集成指南

AgentScope.Go 支持通过 MCP（Model Context Protocol）连接外部工具和服务。以下是常用 MCP Server 的配置示例。

## 快速开始

1. 安装 Node.js 和 npx（用于大多数 MCP Server）
2. 配置 `mcp-config.json`
3. 在 Gateway 中启用 MCP

```go
srv := gateway.NewApp(gateway.AppConfig{
    MCPConfigPath: "./integrations/mcp-servers/mcp-config.json",
})
```

## 常用 MCP Server

### 1. 文件系统 (filesystem)

```json
{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/workspace"]
  }
}
```

**工具**：
- `read_file` — 读取文件内容
- `write_file` — 写入文件
- `list_directory` — 列出目录
- `search_files` — 搜索文件

### 2. Web 搜索 (web-search)

```json
{
  "web-search": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-web-search"],
    "env": { "SERPAPI_KEY": "your-key" }
  }
}
```

**工具**：
- `search` — 执行搜索查询
- `get_page` — 获取页面内容

### 3. 浏览器 (browser)

```json
{
  "browser": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-browser"]
  }
}
```

**工具**：
- `navigate` — 导航到 URL
- `screenshot` — 截图
- `click` — 点击元素
- `get_text` — 提取页面文本

### 4. GitHub

```json
{
  "github": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "your-token" }
  }
}
```

**工具**：
- `search_repositories` — 搜索仓库
- `get_issue` — 获取 issue
- `create_issue` — 创建 issue
- `get_pull_request` — 获取 PR

### 5. SQLite

```json
{
  "sqlite": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-sqlite", "/path/to/db.sqlite"]
  }
}
```

**工具**：
- `query` — 执行 SQL 查询
- `execute` — 执行 SQL 语句
- `schema` — 获取表结构

### 6. HTTP Fetch (Python)

```json
{
  "fetch": {
    "command": "uvx",
    "args": ["mcp-server-fetch"]
  }
}
```

**工具**：
- `fetch` — HTTP GET/POST

## 与 A2A Registry 集成

MCP Server 可以通过 A2A Registry 自动注册为 Agent 工具：

```go
// 启动 MCP Server 并注册到 A2A Registry
registry := a2a.NewRegistry(a2a.NewRedisRegistryStore("redis://localhost:6379"))

// 将 MCP 工具暴露为 A2A AgentCard
for _, tool := range mcpClient.ListTools() {
    registry.Register(ctx, &a2a.AgentCard{
        Name:        "mcp-" + tool.Name,
        Description: tool.Description,
        Capabilities: []string{"mcp", tool.Name},
    })
}
```

## 安全注意事项

- 文件系统 MCP Server 应限制在特定目录
- API Key 应通过环境变量注入，不要硬编码
- 生产环境建议使用 MCP Gateway 的权限控制
- 敏感操作（写文件、执行命令）应经过 HITL 确认

## 更多资源

- [MCP 官方文档](https://modelcontextprotocol.io/)
- [MCP Server 列表](https://github.com/modelcontextprotocol/servers)
- AgentScope.Go MCP 集成：`gateway/session_mcp_gateway.go`
