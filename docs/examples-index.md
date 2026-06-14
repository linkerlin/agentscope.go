# 示例分类索引

AgentScope.Go 提供丰富的示例程序，覆盖从基础入门到生产部署的完整场景。每个示例均可独立运行。

---

## 基础示例

| 示例 | 路径 | 说明 |
|------|------|------|
| Hello Agent | [`examples/hello/main.go`](../examples/hello/main.go) | 最简 ReAct Agent：创建模型、构建 Agent、发送消息 |
| 工具调用 | [`examples/tools/main.go`](../examples/tools/main.go) | 使用 `tool.NewFunctionToolAuto` 创建计算器工具并绑定到 Agent |
| 状态持久化 | [`examples/state/main.go`](../examples/state/main.go) | JSONFileStore 的 Save/Get/ListKeys/Exists 完整演示 |
| 中断机制 | [`examples/interrupt/main.go`](../examples/interrupt/main.go) | 通过 `agent.Interrupt()` 在慢模型推理中触发优雅中断 |

---

## 模型示例

| 示例 | 路径 | 说明 |
|------|------|------|
| OpenAI | [`examples/hello/main.go`](../examples/hello/main.go) | 使用 OpenAI GPT-4o-mini 构建 Agent |
| Anthropic | [`examples/anthropic/main.go`](../examples/anthropic/main.go) | 使用 Claude 3.5 Sonnet 构建 Agent |
| Gemini | [`examples/gemini/main.go`](../examples/gemini/main.go) | 使用 Gemini 1.5 Flash 构建 Agent |
| 多模态路由 | [`examples/multimodal_router/main.go`](../examples/multimodal_router/main.go) | MultimodalRouter 自动在文本模型和视觉模型间切换 |
| 语音 Agent | [`examples/voice/main.go`](../examples/voice/main.go) | TTS + STT 完整语音对话流程 |
| 实时语音 | [`examples/voice/realtime/main.go`](../examples/voice/realtime/main.go) | 端到端实时语音：VAD → STT → LLM → TTS，支持打断 |

---

## 记忆示例

| 示例 | 路径 | 说明 |
|------|------|------|
| ReMe 文件记忆 | [`examples/reme/file/main.go`](../examples/reme/file/main.go) | 纯文件型 ReMe：Add/GetMemoryForPrompt/SaveTo/LoadFrom |
| ReMe 向量记忆 | [`examples/reme/vector/main.go`](../examples/reme/vector/main.go) | 固定维度嵌入 + LocalVectorStore + 混合检索（VectorWeight 0.5） |
| ReMe 摘要器 | [`examples/reme/summarizers/main.go`](../examples/reme/summarizers/main.go) | Personal/Procedural/Tool Summarizer + 去重演示 |
| ReMe 编排器 | [`examples/reme/orchestrator/main.go`](../examples/reme/orchestrator/main.go) | MemoryOrchestrator 端到端：SummarizeMemory + RetrieveMemoryUnified |
| RAG 管道 | [`examples/rag/main.go`](../examples/rag/main.go) | TextLoader → 嵌入 → VectorMemory → 语义检索 |
| 跨模态检索 | [`examples/cross_modal/main.go`](../examples/cross_modal/main.go) | CrossModalSearcher：文本查询返回混合结果（文本+图像） |
| 记忆基准 | [`examples/memory_benchmark/main.go`](../examples/memory_benchmark/main.go) | 运行 LoCoMo 长对话基准并输出评分 |
| ReAct 编排 | [`examples/react_orchestrator/main.go`](../examples/react_orchestrator/main.go) | ReactStepRecorder + ReactOrchestrator + 记忆注入 |

---

## 编排示例

| 示例 | 路径 | 说明 |
|------|------|------|
| 流水线 | [`examples/pipeline/main.go`](../examples/pipeline/main.go) | 两个 Agent 顺序串联：Planner → Writer |
| 工作流 | [`examples/workflow/main.go`](../examples/workflow/main.go) | Parallel + Pipeline + Condition 组合编排 |
| MapReduce | [`examples/mapreduce/main.go`](../examples/mapreduce/main.go) | 长文本拆分 → 并行摘要 → 合并总结 |
| 反射 Agent | [`examples/reflection/main.go`](../examples/reflection/main.go) | SelfReflectingAgent：Writer + Critic 迭代优化 |
| 消息中心 | [`examples/msghub/main.go`](../examples/msghub/main.go) | MsgHub.Broadcast 向多个 Agent 并行发送消息 |
| 调度器 | [`examples/schedule/main.go`](../examples/schedule/main.go) | Cron 表达式定时任务调度与列表管理 |

---

## A2A 示例

| 示例 | 路径 | 说明 |
|------|------|------|
| A2A 基础 | [`examples/a2a/main.go`](../examples/a2a/main.go) | 完整 A2A 流程：Server → Client.Send → WaitForTask |
| A2A 安全 | [`examples/a2a_secure/main.go`](../examples/a2a_secure/main.go) | SecureServer + API Key + RateLimiter + WebSocket |
| A2A Redis 注册中心 | [`examples/a2a_redis_registry/main.go`](../examples/a2a_redis_registry/main.go) | Redis 分布式注册 + 一致哈希分片 + 健康检查 |

---

## 生产示例

| 示例 | 路径 | 说明 |
|------|------|------|
| 生产服务 | [`examples/production/main.go`](../examples/production/main.go) | NewApp 一键装配：JWT + 记忆 + 工具 + 权限 + Schedule |
| 全功能服务 | [`examples/full_service/main.go`](../examples/full_service/main.go) | 最大自动装配：tracing + embedding cache + auto restore |
| 多租户工作区 | [`examples/multi_tenant_workspace/main.go`](../examples/multi_tenant_workspace/main.go) | 多租户网关 + MCP Sidecar + 工作区隔离 |
| 网关服务 | [`examples/gateway/main.go`](../examples/gateway/main.go) | gateway.NewServer 提供 SSE + WebSocket 端点 |
| Web UI | [`examples/web_ui/main.go`](../examples/web_ui/main.go) | AG-UI Streamable HTTP 协议前端对接 |
| Studio | [`examples/studio/main.go`](../examples/studio/main.go) | 纯 Go + HTMX 管理后台：凭证/Agent/记忆/A2A/Evolver |

---

## ONNX 示例

| 示例 | 路径 | 说明 |
|------|------|------|
| ONNX 预处理 | [`examples/onnx/main.go`](../examples/onnx/main.go) | ImagePreprocessor + AudioPreprocessor + CLIPImageEmbedder 演示 |
| 跨模态 | [`examples/cross_modal/main.go`](../examples/cross_modal/main.go) | 文本查询图像结果的跨模态搜索 |

---

## 可观测性示例

| 示例 | 路径 | 说明 |
|------|------|------|
| 事件总线 | [`examples/observability/main.go`](../examples/observability/main.go) | event.Bus + LangSmith Observer 转发 |
| 链路追踪 | [`examples/trace/main.go`](../examples/trace/main.go) | recorder.Builder 记录推理过程到 JSONL |
| LangSmith | [`examples/langsmith/main.go`](../examples/langsmith/main.go) | 完整 LangSmith 集成：Bus + Observer + TracedAgent |
| 中间件 | [`examples/middleware/main.go`](../examples/middleware/main.go) | LoggingMiddleware + HookOnError 组合 |

---

## 其他示例

| 示例 | 路径 | 说明 |
|------|------|------|
| 嵌入缓存 | [`examples/embedding/main.go`](../examples/embedding/main.go) | OpenAI 嵌入 + FileCache + 批量嵌入 |
| 多模态工具 | [`examples/multimodal/main.go`](../examples/multimodal/main.go) | OpenAI 文生图 / 图生文工具绑定到 Toolkit |
| GEP 自演化 | [`examples/evolver/main.go`](../examples/evolver/main.go) | GEPFlow：run → reflect → solidify 闭环 + Skill2Gene |
| V2 事件流 | [`examples/v2_event_stream/main.go`](../examples/v2_event_stream/main.go) | 20+ 细粒度事件类型的流式消费演示 |
| 慢工具卸载 | [`examples/shared/slowtool/slow_demo.go`](../examples/shared/slowtool/slow_demo.go) | 模拟长时运行工具，用于 Tool Offload 测试 |

---

## 快速导航

按学习目标选择：

- **刚入门** → `examples/hello` + `examples/tools`
- **学记忆** → `examples/reme/file` → `examples/reme/vector` → `examples/rag`
- **学编排** → `examples/pipeline` → `examples/workflow` → `examples/mapreduce`
- **学 A2A** → `examples/a2a` → `examples/a2a_secure` → `examples/a2a_redis_registry`
- **学部署** → `examples/gateway` → `examples/production` → `examples/full_service`
- **学 ONNX** → `examples/onnx` → `examples/cross_modal`
- **学可观测** → `examples/trace` → `examples/observability` → `examples/langsmith`
