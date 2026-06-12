务必使用中文进行思考、推理和输出！
======

# AgentScope Go 开发备忘录

## 项目概述

本项目是 [AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现，采用地道的 Go 惯用法构建生产级 AI Agent 框架。当前版本 **v2.0.0-rc.3**。

## V2 架构总览

```
网关层     gateway/       HTTP/SSE/WebSocket/流式 HTTP + AG-UI Protocol + 多租户认证 + Tool Offload
          a2a/           Agent-to-Agent 协议 (Go 领先 PyV2)
服务层     service/       Storage 抽象 + CRUD + AES-GCM 加密 + Cipher
          schedule/      Cron 调度器 + 定时任务引擎
          runcontext/    运行时上下文注入 (Session/Tools/Agent)
状态层     state/         AgentState 可序列化存储 (JSONFile + Redis + 加密)
          session/       会话管理 (SessionManager + 事件缓冲 + 重放)
          interruption/  中断处理
          shutdown/      优雅关闭
消息层     message/       Msg 类型 + 多模态 ContentBlock (Text/Image/Audio/File 等)
编排层     pipeline/      流水线编排 (顺序 + 并行)
          workflow/      MapReduce/Condition/Loop/Parallel
          msghub/        消息中心广播
          reflection/    反思机制 (Writer+Critic)
Agent 层   agent/         V1/V2 接口 + Base 基类 + ReActAgent (事件流 + 状态机 + 结构化输出)
          middleware/     Agent 生命周期中间件链 (洋葱模型: Reply/Reasoning/Acting/ModelCall/SystemPrompt)
          subagent/      元工具: Agent 作为 Tool 递归委托
事件系统   event/         20+ 事件类型 + Bus + MetricsHandler
          hook/          经典 Hook (9 HookPoint) + StreamHook + JSONL Trace Exporter
能力层     model/         10+ 后端 + OpenAI Responses API + ModelCard YAML (35 卡片) + TTS/Audio + Router
          tool/          内置工具集 (file/shell/web/json/multimodal + Task/Schedule/Subagent)
          toolkit/       工具注册/执行 + MCP 适配 + 中间件链
          formatter/     5 后端格式化器 + MultiAgent 变体
          workspace/     Local/Docker/E2B + MCP Gateway + Offloader
          permission/    规则引擎 + Bash AST 解析 + Shell 命令验证
          embedding/     独立 Embedding 包 (OpenAI/Ollama/Gemini/DashScope + FileCache)
记忆层     memory/        ReMe (文件/向量) + 5 向量后端 + Hybrid Search + Dream 演化引擎 + Compactor/Summarizer
可观测性   observability/ OpenTelemetry + LangSmith + TracingMiddlewareAdapter + otelbridge
演化层     evolver/       GEP Gene/Capsule 类型 + Evolver 客户端 + Run/Reflect/Solidify 流程 + Skill2GEP 蒸馏
辅助包     config/        配置管理
          credential/    凭证抽象/Factory/Schemas
          dist/          分发/打包
          loader/        文档加载器 (TextLoader/DirLoader)
          output/        结构化输出 (StructuredRunner + 校验重试)
          retry/         重试策略 (指数退避/永久错误分类)
          async/         异步任务执行池
          plan/          PlanNotebook 多步骤任务管理
          rag/           RAG 集成 (含 Apache Tika + Memory Adapter)
          skill/         SkillBox + SkillViewer + load_skill + 蒸馏到 Gene
          tests/         跨语言契约测试
```

## 核心模块与代码量（非测试行 / 测试行 / 测试文件数）

| 模块 | 非测试行 | 测试行 | 测试文件 | 说明 |
|------|----------|--------|----------|------|
| `memory/` | 9,780 | 4,470 | 51 | 最大模块: ReMe + 5 向量后端 + Dream + Summarizer/Compactor + Hybrid Search |
| `gateway/` | 5,168 | 4,134 | 29 | HTTP/SSE/WS/流式 HTTP + AG-UI + Tool Offload + 调度 CRUD |
| `tool/` | 4,628 | 2,848 | 27 | file/shell/web/json/multimodal + Task/Schedule/Subagent |
| `agent/` | 3,610 | 3,146 | 18 | V1/V2 接口 + Base + ReActAgent + ReplyStream + StructuredOutput |
| `model/` | 2,642 | 2,467 | 20 | 10+ 后端 + Responses API + 35 ModelCard + TTS/Audio + Router |
| `workspace/` | 1,911 | 1,223 | 9 | Local/Docker/E2B + MCP Gateway |
| `toolkit/` | 1,492 | 1,187 | 12 | 工具注册/执行 + MCP 适配 |
| `service/` | 1,336 | 1,177 | 6 | Storage/Auth/Cipher |
| `permission/` | 1,205 | 661 | 8 | 规则引擎 + Bash AST |
| `formatter/` | 946 | 1,203 | 7 | 5 后端 Formatter |
| `skill/` | 1,038 | 393 | 4 | SkillBox + SkillViewer + load_skill |
| `evolver/` | 928 | 94 | 1 | 🆕 GEP Gene/Capsule + Mock/Recording 客户端 |
| `event/` | 1,007 | 434 | 4 | 20+ 事件类型 + Bus |
| `a2a/` | 826 | 878 | 6 | Agent 间协议 (Go 独有) |
| `message/` | 734 | 641 | 3 | Msg + 多模态 ContentBlock |
| `plan/` | 612 | 459 | 3 | PlanNotebook 多步骤管理 |
| `hook/` | 524 | 364 | 4 | 经典 Hook + StreamHook + Trace Exporter |
| `credential/` | 510 | 94 | 1 | 凭证抽象/Factory |
| `observability/` | 477 | 514 | 4 | OTel + LangSmith + TracingMiddlewareAdapter |
| `state/` | 384 | 305 | 3 | AgentState 持久化 (JSONFile/Redis) |
| `middleware/` | 318 | 126 | 2 | 洋葱模型中间件链 |
| `embedding/` | 316 | 85 | 1 | 4 后端 + FileCache |
| `rag/` | 301 | 259 | 4 | RAG + Tika 集成 |
| `session/` | 250 | 131 | 2 | 会话管理 |
| `workflow/` | 245 | 245 | 2 | MapReduce/Condition/Loop/Parallel |
| `dist/` | 244 | 167 | 1 | 分发/打包 |
| `async/` | 191 | 176 | 1 | 异步执行池 |
| `pipeline/` | 130 | 239 | 3 | 顺序/并行流水线 |
| `schedule/` | 121 | 160 | 1 | Cron 调度器 |
| `output/` | 104 | 129 | 1 | 结构化输出 |
| `loader/` | 99 | 71 | 1 | 文档加载器 |
| `tests/` | 95 | 69 | 1 | 跨语言契约测试 |
| `config/` | 91 | 53 | 1 | 配置管理 |
| `msghub/` | 87 | 105 | 1 | 消息中心广播 |
| `reflection/` | 66 | 143 | 1 | 反思机制 (Writer+Critic) |
| `retry/` | 61 | 95 | 1 | 重试策略 |
| `shutdown/` | 42 | 35 | 1 | 优雅关闭 |
| `interruption/` | 51 | 52 | 1 | 中断处理 |
| `runcontext/` | 39 | 37 | 2 | 运行时上下文 |
| **总计** | **~39,430** | **~25,968** | **~239** | 持续增长 |

## 测试

```bash
go test ./... -race -count=1   # 全量通过（提交前强制）
```

推荐使用 `make test`（见根目录 Makefile）或 `make ci` 进行本地模拟。

**注意**: `memory/reme/` 和 `rag/` 子包当前存在编译错误（未定义符号），需优先修复。

## 编码规范（更新于 P0/P1 工程化）

- **所有包必须通过** `go test ./... -race -count=1`
- **提交前必须** `gofmt -l .` 返回空（或使用 `make fmt` / `make fmt-check`）
- 优先使用 `golang.org/x/sync/errgroup` 进行并发控制
- 中断检查优先使用原子操作，配合 `sync.RWMutex` 保护复杂状态
- 多模态结果使用 `message.ContentBlock` 子类型封装
- 工具返回值使用 `tool.Response` 规范类型
- 事件流使用 `<-chan event.AgentEvent` channel 模式
- Agent 状态挂起/恢复通过 `InjectEvent()` + `pendingExternalEvents` 实现
- 推荐安装 golangci-lint 并通过 `make lint` / `golangci-lint run ./...`
- 新代码优先使用顶级 `embedding/` 包（NewOpenAI / NewOllama / NewGemini / NewDashScope + WithFileCache）。`memory/embedding` 仅为向后兼容的 adapter（已标记 Deprecated）。
- 中间件使用洋葱模型（`OnXxx(ctx, agent, input, next XxxNext) -> (*Msg, error)`），支持 Reply/Reasoning/Acting/ModelCall/SystemPrompt 五个拦截点

本地推荐流程（使用 Makefile）：
```bash
make fmt
make vet
make lint   # 如已安装 golangci-lint
make build
make test   # 或 make ci
```

## 关键设计决策

1. **事件驱动范式**：从"消息为中心"转向"事件为中心"，Agent 输出为 `ReplyStream() -> <-chan event.AgentEvent`
2. **状态机模型**：Agent 可挂起/恢复，`AgentState` 可序列化到 Redis/JSONFile，支持跨请求恢复
3. **Channel vs Iterator**：使用 Go channel 替代 Python AsyncGenerator，背压自然处理
4. **struct embedding 复用**：`agent.Base` 通过 embedding 提供统一生命周期（钩子、中断、关闭）
5. **Formatter 解耦**：消息格式化与模型实现分离，通过 interface 注入（5 个后端分别实现）
6. **Workspace 沙箱**：工具执行隔离在 Local/Docker/E2B 环境中，通过 `workspace.Workspace` 注入
7. **可观测性对齐**：TracingMiddlewareAdapter 实现完整 middleware 5 接口，支持 agent 生命周期 tracing（on_reply/on_reasoning/on_acting/on_model_call/on_system_prompt），结合 TracedAgent + OTel/LangSmith + otelbridge 自动桥接
8. **GEP 自演化对齐 (Phase 6)**：通过 evolver/ 包引入 Gene/Capsule 类型、Evolver 接口（21 方法）、高层 GEPFlow（RunAndSolidify 闭环）、skill2gep 蒸馏、Mock + Recording 客户端。利用现有 gateway MCP 网关 + ReMe + a2a 实现"轻量桥接"
9. **泛型工具构造**：`NewFunctionToolAuto[T]` 通过 Go 泛型自动从 handler 签名提取 JSON Schema 并解码输入
10. **洋葱模型中间件**：`middleware.Chain` 按类型自动分类并构建拦截链，支持 Reply/Reasoning/Acting/ModelCall/SystemPrompt 五点
11. **OpenAI Responses API 独立后端**：`model/openai_response/` 直接使用 `net/http`（不依赖 SDK），支持推理模型的链式思考事件流
12. **多模态路由**：`model.MultimodalRouter` 根据输入媒体内容自动在文本/视觉模型间切换
13. **流式 HTTP 传输**：受 MCP 2025-03-26 启发，`gateway/streamable_http.go` 单一端点支持 POST/GET/DELETE，含 SSE 流式、AG-UI 转换和会话回放
14. **Tool Offload 机制**：长时间运行工具从同步 ReAct 循环中卸载，完成后通过提示注入方式通知 Agent
