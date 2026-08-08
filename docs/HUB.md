# Hub 市场

AgentScope.Go 的 Hub 子系统让用户从注册中心**浏览 + 安装** MCP server 和 Skill。
对齐 Python agentscope 的 Hub 系统（HubBase / MCPHubBase / SkillHubBase，#2197）。

## 架构

```
Hub (市场: 实现 hub.Hub 接口)
   │  ListMCPCards / ListSkillCards (游标分页 + 关键词过滤)
   ▼
卡片 (MCPCard: 连接配置模板 / SkillCard: 压缩包下载地址)
   │
   ├─ InstallMCPs  → toolkit/mcp.ConnectServers (弹性连接, 缺失二进制跳过)
   └─ InstallSkill → HTTP 下载 + 解压 (zip/tar/tar.gz, zip-slip 防护)
```

## 快速开始（内置 FSHub）

目录即市场：一个目录放 `mcps.json` + `skills.json` 就是 Hub：

```json
// mcps.json
[
  {"id":"filesystem","name":"Filesystem","description":"Local file access",
   "spec":{"name":"filesystem","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","$WORKDIR"]}}
]
```

```json
// skills.json
[
  {"id":"code-review","name":"Code Review","description":"Review patches",
   "archive_url":"https://example.com/code-review.zip"}
]
```

```go
h, _ := builtin.NewFSHub("./catalog", "demo", "Demo Hub", "A marketplace")

// 浏览（游标分页）
mcps, nextCursor, _ := h.ListMCPCards(ctx, "github", 0, 20)
skills, _, _ := h.ListSkillCards(ctx, "", 0, 20)

// 安装
hub.InstallSkill(ctx, skills[0], "./skills")
mgr, results := hub.InstallMCPs(ctx, mcps)  // 弹性: 缺失二进制优雅跳过
```

## Gateway 集成

```go
srv := gateway.NewServer(agent)
srv.WithHubs(myHub)             // 可注册多个 hub
srv.RegisterHubRoutes()
```

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/hubs` | 列出所有 hub |
| GET | `/api/v1/hubs/{id}/mcps?q=&cursor=&limit=` | 浏览 MCP 卡片（分页） |
| GET | `/api/v1/hubs/{id}/skills?q=&cursor=&limit=` | 浏览 Skill 卡片 |
| POST | `/api/v1/hubs/{id}/mcps/{card}/install` | 安装 MCP（弹性跳过缺失） |
| POST | `/api/v1/hubs/{id}/skills/{card}/install?dest=` | 安装 Skill（下载解压） |

## 自定义 Hub

实现 `hub.Hub` 接口（3 个方法），对接任意后端（ClawHub、GitHub、对象存储等）：

```go
type MyHub struct{}

func (MyHub) ID() string                        { return "my-hub" }
func (MyHub) DisplayName() string               { return "My Hub" }
func (MyHub) ListMCPCards(ctx, q string, cursor, limit int) ([]hub.MCPCard, int, error) {
    // 游标分页 + 关键词过滤（可用 hub.Page / hub.FilterCards 助手）
}
func (MyHub) ListSkillCards(ctx, q string, cursor, limit int) ([]hub.SkillCard, int, error) { ... }
```

## 安全设计

- **zip-slip / tar-slip 防护**：`safeJoin` 拒绝绝对路径 + `..` 遍历（跨平台：Windows 反斜杠 + POSIX 正斜杠双检查）
- **大小上限**：下载 64 MiB + 解压总量 64 MiB 双重限制
- **MCP 弹性**：二进制缺失/连接失败优雅跳过，一个坏卡片不破坏市场

## 可运行示例

```bash
go run ./examples/hub_demo/
# 浏览 2 个 MCP 卡片 + 1 个 skill 卡片 → 下载解压 skill
```
