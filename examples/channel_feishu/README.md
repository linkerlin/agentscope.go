# channel_feishu — 飞书（Lark）Channel 适配器示例

演示 AgentScope.Go 的飞书平台集成：事件订阅 webhook 收消息 → 路由到 agent → REST API 回发。
飞书无官方 Go SDK，本实现**纯 HTTP**（零新依赖）。

## 运行

```bash
FEISHU_APP_ID=xxx FEISHU_APP_SECRET=yyy go run ./examples/channel_feishu/
```

在[飞书开放平台](https://open.feishu.cn)配置应用：
1. 获取 App ID / App Secret（凭证与安全）
2. 添加事件订阅 `im.message.receive_v1`（接收消息）
3. 订阅 URL 指向 `http://<host>:8090/webhook`

## 能力

| 能力 | 实现 |
|------|------|
| 收消息 | 事件订阅 webhook（URL 验证 challenge + `im.message.receive_v1` 归一化） |
| 发消息 | REST `POST /im/v1/messages`（tenant_access_token 2h 缓存自动刷新） |
| Agent 工具 | `feishu_send_message`（发文本）/ `feishu_list_chats`（列群） |

## Agent 工具

```go
// 注册到 agent toolkit
tk.Register(&feishu.SendMessageTool{Channel: fc})
tk.Register(&feishu.ListChatsTool{Channel: fc})
```

agent 即可在对话中主动发消息到其他群 / 列出机器人所在的群。

## 测试

`channel/feishu/` 10 测试：httptest mock 飞书 API（token 缓存/send/webhook challenge/消息归一化/图片元数据/工具）。
