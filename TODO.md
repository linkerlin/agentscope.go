# TODO

仅列尚未完成的工作。验收标准在 [演进方案.md](演进方案.md) 的同名 ID；合入后从两份文件同时删除，不保留完成记录。实施顺序以依赖为准，不以历史 ID 的数字为准。

## P1 · v2.8 渠道、运行环境与网关收口


## P1 · v2.9 实时语音与协议保真

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
