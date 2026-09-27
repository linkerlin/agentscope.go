# TODO

未完成工作。验收标准在 [演进方案.md](演进方案.md) 的同名 ID。做完就删掉对应行，不要在这里留「已完成」清单。

已在代码里、不再排期：GoalPipeline、`sop/`、`classifier/`、AskUser、PowerShell Backend、工具重试 hint、`MaxImageNum`、中断取消语义、formatter 保真批次（20.1 / 20.2）、prompt cache 记账（20.9）、权限只读参数校验与 grep 负分页（16.7）。

## P0 · v2.6.x 架构内债

- [ ] **16.2** 拆 `gateway/`：会话、知识库、团队、工作区子包；URL 不变；根包留 `Server` + `AppConfig`
- [ ] **16.3** 可选 CostRouter：无配置时与现有 `Router` 相同；`RouteDecision` 进 tracing；默认关

## P1 · v2.8 渠道与多副本

先 18.1、18.2，再渠道。Webhook / Discord / 飞书 Webhook 已有。

- [ ] **18.1** 会话协调：`SessionRun` 锁、事件日志、cancel、purge、`BgTask` 注册表
- [ ] **18.2** `GET /sessions/{id}/status`：running / parked / idle / unknown
- [ ] **18.3** 钉钉：OpenAPI、流式卡片、卡片回调 → HITL、wiki 工具
- [ ] **18.4** 凭证绑定状态机，状态放 message bus，多副本可恢复
- [ ] **18.5** 长连接 worker：与 API 副本分离、对账、心跳、断连重连
- [ ] **18.6** 渠道能力表；飞书升级 WebSocket 长连接；长消息拆分、reaction、列会话
- [ ] **18.7** `GitHubMCPHub` + `ClawSkillHub`（分页、限流、配置模板；安装仍防 zip-slip）
- [ ] **18.8** Workspace 预热池
- [ ] **18.9** 下载令牌、`/tts-models`、`/embedding-models`

## P1 · v2.9 实时语音

`examples/voice/realtime` 是演示管道，不算本段。

- [ ] **19.1** `realtime/` 契约 + ModelCard + Mock 事件流
- [ ] **19.2** Transport / VAD / Playout
- [ ] **19.3** `RealtimeAgent`：打断改写已听前缀、断线重连、工具确认超时
- [ ] **19.4** DashScope 后端打通端到端
- [ ] **19.5** OpenAI Realtime
- [ ] **19.6** Gemini Live 与 xAI 语音
- [ ] **19.7** console 语音入口
- [ ] **19.8** TTS：Gemini TTS、CosyVoice v3、gpt-4o-mini-tts

## P1 · v2.9.x 保真与协议

- [ ] **20.3** 火山方舟 Ark 后端 + formatter
- [ ] **20.4** Moonshot Kimi K3 / K2.7 卡片与 thinking 参数（现停在 k2.6）
- [ ] **20.5** `A2AAgent` 当本地 `Agent` 用，状态可续接；`NoopClient` 退出推荐路径
- [ ] **20.6** `ToolChunk` 工具结果增量流出，对齐 AG-UI
- [ ] **20.7** 输入注入护栏 + 输出过滤中间件（可关，默认保守）
- [ ] **20.8** `memory.Facade`；向量存储父包与 `memory/vector` 收成一条路径

## P2 · v3.0

- [ ] **21.1** web_ui 补 Hub 安装、Channel 管理、workspace git status；studio 标记 deprecated
- [ ] **21.2** 全功能 TUI：语音、表单、工具分组在同一终端（在 16.1 稳定后评估）
- [ ] **21.3** 本地 Gene 仓库，无外部 MCP 可演示 Run / Reflect / Solidify
- [ ] **21.4** 主热路径基准进 CI，超阈值失败
- [ ] **21.5** Team `Mode=peer` 接到 worker（`integrations/coordbuslease` 已有）
- [ ] **21.6** 从 gateway 抽出 ChatService
- [ ] **21.7** 单核与拆包稳定后收敛公开 API；`Call` 文档标明为收集器
- [ ] **21.8** 非 Linux 插件的官方路径：进程内注册 + 示例

## 不做

Apple Container、React 重写 UI、ONNX/LightGBM 路由、把治理写进 agent 内核、ES/Mongo/S3/NATS/K8s Operator、删除 `Call`、复刻全部模型卡、重做附录里已关闭的 ID。理由见演进方案第 3 节。
