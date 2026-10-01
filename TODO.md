# TODO

仅列尚未完成的工作。验收标准在 [演进方案.md](演进方案.md) 的同名 ID；合入后从两份文件同时删除，不保留完成记录。实施顺序以依赖为准，不以历史 ID 的数字为准。

## P1 · 先于渠道功能

- [ ] **18.10** 内置控制台会话身份：首包不提交客户端自造 ID，采纳响应头里的服务端 ID；接上 storage 与认证后聊天、steer、interrupt 仍可用

## P1 · v2.8 渠道、运行环境与网关收口

- [ ] **18.4** 凭证交互绑定：`PENDING` / `AUTHORIZED` / `FAILED` / `CANCELLED` 状态机使用凭证引用而非明文，并支持多副本幂等恢复
- [ ] **18.3** 钉钉闭环：凭证绑定式企业应用配置；受认证保护的卡片回调幂等注入 HITL；Agent 事件流节流写入卡片并正确收尾
- [ ] **18.6** 渠道能力模型：能力声明驱动发送策略；飞书 WebSocket；长消息拆分、reaction、会话列表及 fake-API 契约测试
- [ ] **16.2** 拆 `gateway/` 剩余簇：在 18.5 调用边界稳定后将渠道 / wakeup / 定时任务注册函数化为窄依赖子包；外部 URL、认证和协调语义不变，根包保留 `Server` 与 `AppConfig`
- [ ] **18.7** 远程 Hub：`GitHubMCPHub` 与 `ClawSkillHub` 支持 cursor 分页、429/5xx 限流重试、配置模板校验与填充，安装继续复用安全解包
- [ ] **18.8** Workspace 预热池：后端化的池大小和最大创建数；借还、失败销毁、关闭排空与冷启动基准
- [ ] **18.9** 模型发现：统一 chat / TTS / embedding 卡片枚举；绑定用户、资源和过期时间的下载令牌；未授权、过期和跨租户访问拒绝

## P1 · v2.9 实时语音与协议保真

- [ ] **19.1** `realtime/` 契约：统一 realtime 与 TTS ModelCard schema、截断能力三态、模型事件和 Mock 事件流
- [ ] **19.2** Transport / VAD / Playout：格式协商、控制帧背压、VAD 边沿与基于播放确认时钟的 `PlayoutPosition`
- [ ] **20.6** `ToolChunk`：工具执行真实增量输出并保留顺序、错误、取消语义，映射到 AG-UI
- [ ] **20.7** 护栏中间件：输入注入检测和输出过滤覆盖模型、工具与二进制内容；默认保守且可关闭
- [ ] **19.3** `RealtimeAgent`：打断仅保留已听前缀；断线重连不丢 backlog；工具确认超时；与 18.5 的多副本语义一致
- [ ] **19.4** DashScope realtime：qwen-omni / qwen-audio + CosyVoice realtime 端到端、工具调用与 `TurnMetrics`
- [ ] **19.5** OpenAI Realtime：文本与双向音频路径、取消和重连的协议 mock
- [ ] **19.6** Gemini Live / xAI：复用同一契约测试集，不接受仅能创建客户端
- [ ] **19.7** console 语音入口：假 Transport 覆盖文本切换、打断和 HITL，设备 I/O 仅做显式手工冒烟
- [ ] **19.8** TTS：Gemini TTS、CosyVoice v3、gpt-4o-mini-tts 卡片与请求 / 错误契约；音频隐私、留存与成本指标有明确默认策略
- [ ] **20.3** 火山方舟 Ark：doubao-seed 后端、formatter、流式和结构化输出契约
- [ ] **20.4** Moonshot：Kimi K3 / K2.7 卡片与跨后端统一 thinking 参数
- [ ] **20.5** `A2AAgent`：远程 Agent 作为本地 Agent 使用，状态按 task 续接，运行中拒绝二次发送；`ClusterManager` 发送走真实 Client，移出 `NoopClient` 推荐路径
- [ ] **20.8** `memory.Facade`：窗口、ReMe、Agentic、长期记忆从单一入口构造；示例不直连内部实现；撤下 Elasticsearch / pgvector 占位构造函数，并明确其余兼容包装的弃用策略

## P2 · v3.0 平台收敛

- [ ] **21.1** 单一 Web UI：Hub 安装、Channel 管理、workspace git status；标记 studio deprecated
- [ ] **21.2** 全功能 TUI：语音、AskUser 表单、工具分组在同一终端
- [ ] **21.3** 本地 Gene 仓库：无需外部 MCP 即可 Run / Reflect / Solidify 并在 UI 列出
- [ ] **21.4** 主热路径性能门禁：基准进入 CI，超出经确认的阈值即失败
- [ ] **21.5** Team `Mode=peer`：worker 使用租约领取不同 todo；同一 todo 以 CAS 拒绝重复执行
- [ ] **21.6** `ChatService`：从 gateway 提取对话编排，继续缩小 HTTP 层；治理与 workspace 的 HTTP 簇随此离开根包
- [ ] **21.7** v3 API 收敛：在 23.7 与网关拆分稳定后整理公开 API；`Call` 保留并明确是事件流收集器
- [ ] **21.8** 非 Linux 插件：官方进程内注册路径与完整示例，`.so` 继续限定 Linux

## 不做

Apple Container、React / npm 重写内置 UI、ONNX / LightGBM 路由、把治理强耦合进 `agent/` 内核、ES / Mongo / S3 / NATS / K8s Operator（含补齐现有 ES / pgvector 占位实现）、删除 `Call`、复刻全部模型卡。
