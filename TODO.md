# AgentScope.Go 后续 TODO

> 本文件记录 agentscope.go 项目中的剩余工作项，按优先级排序。  
> 最后更新：2026-06-06

---

## 归档：已完成项（2026-04-15 之前）

### P0 - 架构级缺失
- [x] **AgentBase 统一基类** — `agent/base.go` `Base` struct，ReActAgent 嵌入 `Base`（commit `c762455`）
- [x] **Formatter 层设计与实现** — `Formatter` / `TruncatedFormatter` interface，5 后端实现
- [x] **Hook / 事件生命周期统一** — `HookPreReply`/`HookPostReply`/`HookPreObserve`/`HookPostObserve`，Base 自动触发

### P1 - 功能级缺失
- [x] **消息块对齐 Python 2.0** — `DataBlock` 统一多媒体、`ToolResultBlock` 增强、`message/json.go` 兼容
- [x] **扩展模型后端** — Anthropic、Gemini 原生 HTTP + SSE 实现
- [x] **ToolResponse 规范类型** — `tool.Response` struct 替换裸 `any`
- [x] **Memory 自动集成到 ReActAgent** — `buildHistory` 自动调用 `PreReasoningPrepare`

### P2 - 运行时与扩展
- [x] **多模态工具封装** — `OpenAIMultiModalTool` + `DashScopeMultiModalTool`
- [x] **中断与优雅关闭策略** — `InterruptContext` + `PartialReasoningPolicy`
- [x] **A2A 协议补全** — `AgentAdapter` + `/task/cancel` + SSE 规范化
- [x] **分布式 Agent 协调** — `dist/registry.go` + `dist/coordinator.go`
- [x] **性能优化** — ReActAgent 并发工具执行（errgroup）+ 结果保序
- [x] **更多 Hook 点** — `HookPreCall`

---

## 归档：已完成项（2026-04-15 至 2026-06-06 密集 V2 演进）

### P0 - 事件驱动范式重构
- [x] `event/` — 20+ 事件类型 + `Bus` + `MetricsHandler` HTTP 端点 + Block 生命周期
- [x] `agent/react` — `ReplyStream()` 真事件流 + `AgentState` 挂起恢复 + HITL 注入
- [x] `model/` — `StreamChunk.IsThinking` + `ThinkingBlockDeltaEvent`

### P1 - 生产级能力补齐
- [x] `workspace/` — `LocalWorkspace` 完整实现 + `DockerWorkspace` CLI 实现
- [x] `permission/` — 规则引擎（glob/regex/substring）+ 4 种决策模式
- [x] `toolkit/middleware` — Logging + Metrics + Permission + Tracing + Offload 洋葱链
- [x] `toolkit/mcp` — MCP Server 适配器 + `fullSession` + `RegisterMCPManager` + `reset_equipped_tools`
- [x] `formatter/` — `ThinkingFormatter` + `extractThinkingBlocks` + 5 后端实现
- [x] `message/block` — `HintBlock` + JSON 序列化 + `GetHintContent` / `GetThinkingContent`
- [x] `event/block` — `HintBlockStart/Delta/End` 事件类型
- [x] `model/router` — 重试/回退/退避路由
- [x] `model/audio` — `AudioModel` 接口 + `MockAudioModel` + `OpenAITTS`

### P2 - 服务化与生态扩展
- [x] `service/` — `Storage` interface + `MemoryStorage` + `RedisStorage` + Auth + 管理端点 CRUD
- [x] `schedule/` — Cron 调度器 `Schedule/Cancel/NextRun`
- [x] `a2a/` — 动态发现 `Registry` + V2 适配器 + 健康检查
- [x] `gateway/` — V2 SSE `/v2/chat/stream` + WebSocket `/v2/chat/ws` + 多租户认证
- [x] `service/auth` — API Key + JWT 多租户认证中间件
- [x] `event/metrics` — 事件流 MetricsCollector + `BusWithMetrics` + HTTP endpoint
- [x] `gateway/auth` — `/api/v1/auth/register` + `/login` + `/me`
- [x] `observability/otel` — `InitTracerProvider` + `OtelTracer` + Gateway HTTP 追踪中间件
- [x] `gateway/service` — `/api/v1/agents` + `/sessions` + `/credentials` CRUD + 多租户隔离

---

## 进行中 / 剩余工作项

### 🔴 P1 - 高优先级（架构完整性 / 生产阻塞）

#### 1. `state/redis_store.go` — RedisStore 生产级后端
- **状态**：✅ **已完成**

#### 2. `gateway/session_state.go` — Gateway 跨请求挂起恢复
- **状态**：✅ **已完成**

#### 3. `docs/` — 开发者文档体系建设
- **状态**：✅ **已完成（基础文档骨架）**

#### 4. `workspace/e2b.go` — E2B 云端沙箱 REST API 生命周期管理
- **目标**：补齐 E2B Workspace，实现云端沙箱隔离
- **关键交付物**：
  - `workspace/e2b.go`：`E2BClient`（REST API）+ `E2BWorkspace`
  - 支持 sandbox 创建 / 删除 / 刷新 / 超时设置
  - `CreateE2BWorkspace()` 工厂函数
  - `workspace/e2b_test.go`：4 个测试通过（基于 httptest mock）
- **剩余阻塞**：文件读写和命令执行需要 E2B `envd` gRPC 连接（protobuf + connect），尚无官方 Go SDK 支持
- **状态**：🟡 **REST API 生命周期管理已完成；文件/命令操作待 envd gRPC 实现**

#### 5. `schedule/scheduler.go` — `Schedule` 重复 ID 替换 + flaky test 修复
- **问题**：`Schedule` 未移除同 ID 旧 cron entry，导致重复调度
- **修复**：`Schedule` 先 `Remove` 旧 entry 再添加新 entry
- **测试**：`TestScheduler_DuplicateID` 重写为基于 `NextRun` 的非时间敏感验证；20 次连续通过
- **状态**：✅ **已完成**

### 🟡 P2 - 中优先级（功能补齐 / 生态扩展）

#### 5. `model/deepseek/` — DeepSeek 模型后端
- **状态**：✅ **已完成**

#### 6. `model/vllm/` — vLLM 模型后端
- **状态**：✅ **已完成**

#### 7. `tool/subagent/` — SubagentTool 实现
- **状态**：✅ **已完成**

#### 8. `model/moonshot/` — Moonshot (Kimi) 模型后端
- **目标**：补齐国内高频使用的 Moonshot 模型支持
- **说明**：Moonshot API 兼容 OpenAI 格式，可复用 `model/openai` 的 formatter 和 SSE 解析逻辑
- **状态**：✅ **已完成**

#### 9. `model/xai/` — xAI (Grok) 模型后端
- **目标**：补齐 xAI Grok 模型支持
- **说明**：xAI API 兼容 OpenAI 格式
- **状态**：✅ **已完成**

#### 10. `examples/langsmith/` — LangSmith 可观测性示例
- **目标**：演示如何将 Agent 事件流发送到 LangSmith
- **关键交付物**：`examples/langsmith/main.go`：mock model + `LangSmithObserver` + `event.Bus`
- **状态**：✅ **已完成**

#### 11. `README.md` — 同步更新模型列表与示例索引
- **更新内容**：新增 DeepSeek / Moonshot / xAI / vLLM 模型；新增 LangSmith / OTel / Voice / 多租户示例
- **状态**：✅ **已完成**

#### 8. `observability/langsmith.go` + `observability/langsmith_observer.go` — LangSmith 可观测性集成
- **目标**：将 Agent 事件流转发到 LangSmith 进行追踪分析
- **关键交付物**：
  - `observability/langsmith.go`：`LangSmithClient` + `Run` struct + `CreateRunsBatch`
  - `observability/langsmith_observer.go`：`LangSmithObserver` 订阅 `event.Bus`，映射 `ReplyStart`/`ReplyEnd`/`Error`/`ToolCallStart`/`ToolCallEnd` 为 LangSmith Runs
  - `observability/langsmith_observer_test.go`：4 个单元测试通过（httptest mock）
- **状态**：✅ **已完成**

#### 8. `message/json.go` — `source` 嵌套结构跨语言兼容
- **目标**：与 PyV2 的媒体块 JSON 序列化格式完全兼容
- **关键交付物**：
  - `rawBlock` 增加 `source` 嵌套结构的序列化路径
  - 向后兼容现有扁平字段格式
  - 跨语言契约测试：Go 生成 → Python 解析 → Python 生成 → Go 解析
- **状态**：✅ **代码已完成（单元测试通过）；跨语言契约测试待建**

#### 9. `service/cipher.go` + `gateway/service_handlers.go` — AES-GCM 加密存储
- **状态**：✅ **已完成**

#### 10. `examples/multi_tenant_workspace/` — 端到端多租户示例
- **状态**：✅ **已完成**

#### 11. `pipeline/parallel.go` — Multi-agent 并发启动 (ParallelAgent)
- **状态**：✅ **已完成**

#### 12. `model/model.go` + `model/openai/openai.go` — 结构化输出 (Structured Output)
- **状态**：✅ **已完成**

#### 13. `async/pool.go` — 异步任务执行池
- **状态**：✅ **已完成**

#### 14. `loader/` — 文档加载器
- **状态**：✅ **已完成**

### 🔵 P3 - 低优先级（前瞻 / 工程优化）

#### 15. `tests/cross_lang/` — 跨语言契约测试
- **目标**：验证 Go ↔ Python v2 消息 JSON 双向互解析
- **关键交付物**：
  - `tests/cross_lang/generate_go.go` + `validate_py.py`：Go 生成 → Python 验证
  - `tests/cross_lang/generate_py.py` + `validate_go.go`：Python 生成 → Go 验证
  - `tests/cross_lang/cross_lang_test.go`：`CROSS_LANG=1` 时自动运行双向测试
  - `message/json.go` 重大更新：统一 `type: "data"`、`type: "tool_call"`、`output` 字段、`name` 必填等，与 PyV2 格式对齐
- **状态**：✅ **已完成**

#### 16. Benchmark 基准测试骨架
- **目标**：建立核心路径的基准测试
- **关键交付物**：
  - `agent/react/reply_stream_bench_test.go`：纯文本流式 ~965μs/op；工具调用 ~156μs/op
  - `pipeline/pipeline_bench_test.go`：顺序 pipeline ~5ms/op；并行 pipeline ~6.6ms/op
  - `gateway/gateway_bench_test.go`：HTTP chat ~5.1μs/op；SSE stream ~15.7μs/op
- **状态**：✅ **已完成（骨架）**

#### 17. `examples/voice/main.go` — Voice Agent 端到端示例
- **目标**：演示 STT → Chat → TTS 完整语音对话 pipeline
- **关键交付物**：
  - `examples/voice/main.go`：OpenAI Chat + OpenAITTS（Whisper + TTS-1）
  - 支持音频文件输入转录、Agent 推理、回复音频合成保存
  - 文档注释说明如何扩展为实时语音（pion/webrtc / oto）
  - 编译通过
- **状态**：✅ **已完成**

---

*完成一项请勾选或更新对应条目状态，保持本文件实时更新。*
