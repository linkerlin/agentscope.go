# AgentScope.Go v2.5.0 Release Notes

> 🚀 **AgentScope.Go v2.5.0** —— 多平台 Agent 集成 + 生态市场：Channel（Webhook/Discord/飞书）与 Hub（MCP/Skill 市场）落地，让 AgentScope.Go 从"框架"升级为"可部署到任意聊天平台的 Agent 服务"。
>
> 基于 2026-08-08 对 Python AgentScope main 分支（6cfd9f0）的深度源码审阅，针对 Python 在"多平台聊天机器人平台"阶段的新进展，用 Go 地道方式补齐两大全新维度。

---

## 核心亮点

### 1. Channel 多平台集成（对齐 Python `app/channel/` #1997）

Agent 接入外部消息平台，**3 个开箱即用适配器**：

```
外部平台 (Webhook/Discord/飞书) → Channel(归一化) → Gateway(路由) → Agent(运行) → SendText(回发)
```

- **核心抽象** `channel/`：`ChannelEvent`（归一化入站）+ `Channel` 接口（Start/SendText/Close，与 gateway 解耦）+ `Gateway`（Router.Resolve → Runner.RunUserTurn，错误不杀 listener）+ `Dispatcher`（goroutine 生命周期）+ `RouteTable`/`Binding` 路由（exact>prefix>default，session 派生 `<prefix><chat_id>`）
- **WebhookChannel**：零依赖 HTTP 通道（POST → 事件 → 路由 → 运行 → 回复）
- **DiscordChannel**：`discordgo v0.29.0`（WebSocket Gateway + REST，自消息过滤 + 附件归一化）
- **FeishuChannel**：纯 HTTP 零依赖（tenant_access_token 2h 缓存 + 事件订阅 webhook 含 challenge 验证 + REST 发送）
- **飞书 Agent 工具**：`feishu_send_message` / `feishu_list_chats`（agent 可主动操作飞书）
- **Gateway 集成**：`ChannelRunner`（SessionManager 适配，异步运行 + 回复回发 + inflight 去重）+ HTTP 管理路由
- **测试**：channel 核心 16 + Discord 5 + Feishu 10 + gateway 集成 7 = **38 测试全绿**

### 2. Hub 市场（对齐 Python Hub #2197）

从注册中心**浏览 + 安装** MCP/Skill：

- `hub/`：`Hub` 接口（MCP/Skill 卡片浏览 + 游标分页）+ 泛型 `Page`/`FilterCards`
- `InstallMCPs`：复用 `ConnectServers` 弹性连接（缺失二进制优雅跳过）
- `InstallSkill`：HTTP 下载 + zip/tar/tar.gz 解压 + **zip-slip/tar-slip 跨平台防护** + 64MiB 双上限
- `builtin.FSHub`：**目录即市场**（mcps.json + skills.json）
- Gateway 5 路由：浏览 hubs/MCP/Skill + 安装
- **测试**：hub 12 + builtin 5 + gateway 6 = **23 测试全绿**

### 3. 守护任务：Plugin 生态示例

- `examples/plugin_demo/`：三阶段生命周期（Init → Register → Shutdown）+ YAML 配置 + 工具注册，**实测运行**
- `docs/PLUGIN.md`：插件指南

---

## 新增平台适配器一览

| 平台 | 实现 | 依赖 | 工具 |
|------|------|------|------|
| Webhook | `channel/webhook.go` | 零依赖 | — |
| Discord | `channel/discord/` | discordgo | — |
| 飞书 | `channel/feishu/` | **零依赖**（纯 HTTP） | send_message / list_chats |

---

## 文档

| 新增 | 内容 |
|------|------|
| `docs/CHANNEL.md` | Channel 架构/快速开始/路由/新平台接入指南 |
| `docs/HUB.md` | Hub 浏览安装/FSHub/自定义 Hub/安全设计 |
| `docs/PLUGIN.md` | 插件生命周期/配置/Registrar/.so 加载 |

---

## 升级

```bash
go get github.com/linkerlin/agentscope.go@v2.5.0
```

无破坏性变更。新增能力均为可选接入（nil 降级）。

---

## 致谢

感谢 Python AgentScope 团队。本轮演进基于对其 2026-08-08 main 分支的深度审阅（Channel #1997 / Hub #2197 / Web UI #2171 等 30+ commits），用 Go 地道方式补齐"多平台聊天机器人平台"维度。
