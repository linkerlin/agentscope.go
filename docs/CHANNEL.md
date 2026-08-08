# 多平台 Channel 集成

AgentScope.Go 的 Channel 子系统让 Agent 接入外部消息平台（Discord、飞书、Webhook），
从"框架"升级为"可部署的聊天机器人平台"。对齐 Python agentscope 的 `app/channel`（#1997）。

## 架构

```
外部平台 (Discord/飞书/Webhook)
   │  入站消息 (webhook/WS/REST)
   ▼
Channel (平台适配器: 长连接 + 归一化)
   │  emit(ChannelEvent)
   ▼
Gateway (路由 + 派发)
   │  Router.Resolve(event) → (agentID, sessionID)
   │  Runner.RunUserTurn(...) → 异步运行 agent
   ▼
Agent 运行 (经 SessionManager)
   │  事件流 → 收集最终文本
   ▼
SendText(chatID, reply) 回发到平台
```

| 组件 | 包 | 职责 |
|------|----|------|
| `ChannelEvent` | `channel/` | 归一化入站消息（ChannelID/UserID/ChatID/Text/MediaURLs） |
| `Channel` | `channel/` | 平台适配器接口（Start/SendText/Close） |
| `Gateway` | `channel/` | 路由→运行编排（错误不杀 listener） |
| `Router`/`Runner` | `channel/` | 解耦接口（gateway 提供适配器） |
| `Dispatcher` | `channel/` | channel 生命周期（goroutine 启动/关闭） |
| `RouteTable`/`Binding` | `channel/` | chat→agent 路由（exact>prefix>default） |
| `WebhookChannel` | `channel/` | 零依赖 HTTP 通道（POST → ChannelEvent） |
| `ChannelRunner` | `gateway/` | AgentRegistry+SessionManager 适配器（异步运行+回发） |

## 快速开始（Webhook 通道）

```go
// 1. 构建 channel 子系统
wh := channel.NewWebhookChannel("webhook-1")
reg := channel.NewRegistry()
reg.Register(wh)

router := channel.NewChatRouter(channel.RouteTable{
    ChannelID: "webhook-1",
    Bindings: []channel.Binding{
        {ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"},
        {AgentID: "default-agent", SessionPrefix: "d-"},  // 兜底绑定
    },
})
gateway := channel.NewGateway(router, runner)
dispatcher := channel.NewDispatcher(gateway, reg)
dispatcher.StartAll(ctx)   // 启动所有 channel

// 2. 服务 webhook 端点
mux.Handle("/webhook", wh)
```

POST `{"chat_id":"dev-1","user_id":"u1","text":"hello"}` →
ChannelEvent → 路由到 `dev-agent` → 异步运行 → 回复经 SendText 回发。

## Gateway 集成

```go
// gateway 侧：ChannelRunner 适配 SessionManager + AgentRegistry
runner := gateway.NewChannelRunner(reg, sessions).
    WithLookup(func(channelID string) channel.Channel { return wh })

srv := gateway.NewServer(agent)
srv.WithChannelGateway(channel.NewRegistry(), gw)
srv.RegisterChannelRoutes()   // GET /api/v1/channels + POST /api/v1/channels/{id}/webhook
srv.Start()                   // 自动拉起 dispatcher
```

## 路由绑定

| 规则 | 示例 | 优先级 |
|------|------|--------|
| 精确 ChatID | `{ChatID: "dev-123", AgentID: "a1"}` | 最高 |
| 前缀 ChatID | `{ChatIDPrefix: "dev-", AgentID: "a2"}` | 中 |
| 默认 | `{AgentID: "a3"}` | 兜底 |

Session ID 派生：`<SessionPrefix><ChatID>`（同一 chat 的所有用户共享一个 session，对齐 Python per-chat 派生）。

## 可运行示例

```bash
go run ./examples/channel_webhook/
# POST http://localhost:8090/webhook {"chat_id":"dev-1","user_id":"u1","text":"hi"}
```

## 接入新平台（Discord/飞书等）

实现 `channel.Channel` 接口即可：

```go
type MyChannel struct{ id string }

func (c *MyChannel) ID() string { return c.id }
func (c *MyChannel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
    // 长连接：接收平台消息 → emit(ChannelEvent{...})
}
func (c *MyChannel) SendText(ctx context.Context, chatID, text string) error {
    // 平台 REST API 发送回复
}
func (c *MyChannel) Close() error { ... }
```

Discord 用 `discordgo`（WebSocket Gateway + REST），飞书用 HTTP webhook + REST API（可复用 WebhookChannel 骨架 + 卡片模板）。
