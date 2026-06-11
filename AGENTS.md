务必使用中文进行思考、推理和输出！
======

# AgentScope Go 开发备忘录

## 项目概述

本项目是 [AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现，采用地道的 Go 惯用法构建生产级 AI Agent 框架。当前版本 **v2.0.0-rc.1**。

## V2 架构总览

```
网关层     gateway/       HTTP/SSE/WebSocket + AG-UI Protocol + 多租户认证
服务层     service/       Storage 抽象 + CRUD + AES-GCM 加密
          schedule/      Cron 调度器
          a2a/           Agent-to-Agent 协议 (Go 领先 PyV2)
编排层     pipeline/      流水线编排 / workflow/ MapReduce/Condition/Loop
          msghub/        消息中心 / reflection/ 反思机制
Agent 层   agent/         Base 基类 + ReActAgent (事件流 + 状态机)
          middleware/     Agent 生命周期中间件链
事件系统   event/         20+ 事件类型 + Bus + MetricsHandler
          hook/          经典 Hook + StreamHook
能力层     model/         10 个后端 + ModelCard YAML + TTS/Audio
          tool/          内置工具集 + 多模态 + Task/Schedule/SkillViewer
          toolkit/       工具注册/执行 + MCP 适配 + 中间件链
          formatter/     5 后端格式化器 + MultiAgent 变体
          workspace/     Local/Docker/E2B + MCP Gateway + Offloader
          permission/    规则引擎 + Bash AST 解析
          embedding/     独立 Embedding 包 (OpenAI/Ollama/Gemini/DashScope + FileCache，多模态支持)
记忆层     memory/        ReMe (文件/向量) + 5 向量后端 + Hybrid Search
可观测性   observability/ OpenTelemetry + LangSmith + TracingMiddlewareAdapter (agent 生命周期 hook 追踪)
演化层     evolver/       GEP Gene/Capsule 类型 + Evolver 客户端 + Run/Reflect/Solidify 流程 + Skill2GEP 蒸馏（对齐 ./evolver 优势）
```

## 核心模块与代码量

| 模块 | 代码行数 | 测试文件 | 说明 |
|------|----------|----------|------|
| `memory/` | ~6,025 | 37 | 🏆 ReMe + Orchestrator + Hybrid Search |
| `tool/` | ~3,500 | 18 | 内置工具 + Task/Schedule/SkillViewer + Web/JSON + 多模态 |
| `agent/` (含 react) | ~4,548 | 21 | Base + ReActAgent + ReplyStream + State |
| `model/` | ~2,294 | 17 | 10 后端 + ModelCard YAML |
| `gateway/` | ~1,986 | 13 | SSE/WS/AG-UI/Tool Offload/Model API |
| `toolkit/` | ~1,412 | 12 | 工具注册 + MCP + 中间件链 |
| `event/` | ~1,004 | 4 | 22 事件类型 |
| `service/` | ~1,047 | 5 | Storage/Auth/Cipher |
| `a2a/` | ~826 | 6 | 🏆 Go 独有，Agent 间协议 |
| `workspace/` | ~908 | 4 | Local/Docker/E2B + MCP Gateway |
| `skill/` | ~849 | 2 | SkillBox + SkillViewer |
| `permission/` | ~698 | 2 | 规则引擎 + Bash AST |
| `formatter/` | ~652 | 5 | 多后端 Formatter |
| `embedding/` | 新 | 5+ | 独立 Embedding (providers + cache) |
| `observability/` | ~404 | 4 | OTel + LangSmith + TracingMiddlewareAdapter (支持完整 middleware 接口) |
| `evolver/` | 新 | 3 | GEP Gene/Capsule + Mock/Recording 客户端 + 高层 Flow + Distill（Phase 6 对齐 evolver） |
| **总计** | **~31,300** | **233** | 持续增长 |


## 测试

```bash
go test ./... -race -count=1   # 全量通过（提交前强制）
```

推荐使用 `make test`（见根目录 Makefile）或 `make ci` 进行本地模拟。

## 编码规范（更新于 P0/P1 工程化）

- **所有包必须通过** `go test ./... -race -count=1`
- **提交前必须** `gofmt -l .` 返回空（或使用 `make fmt` / `make fmt-check`）。CI 会严格阻断未格式化代码。
- 优先使用 `golang.org/x/sync/errgroup` 进行并发控制
- 中断检查优先使用原子操作，配合 `sync.RWMutex` 保护复杂状态
- 多模态结果使用 `message.ContentBlock` 子类型封装
- 工具返回值使用 `tool.Response` 规范类型
- 事件流使用 `<-chan event.AgentEvent` channel 模式
- Agent 状态挂起/恢复通过 `InjectEvent()` + `pendingExternalEvents` 实现
- 推荐安装 golangci-lint 并通过 `make lint` / `golangci-lint run ./...`（CI 中有独立 golangci job，使用项目根 .golangci.yml 配置）
- 新代码优先使用顶级 `embedding/` 包（NewOpenAI / NewOllama / ... + WithFileCache）。`memory/embedding` 仅为向后兼容的 adapter（已标记 Deprecated）。

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
2. **状态机模型**：Agent 可挂起/恢复，`AgentState` 可序列化到 Redis，支持跨请求恢复
3. **Channel vs Iterator**：使用 Go channel 替代 Python AsyncGenerator，背压自然处理
4. **struct embedding 复用**：`agent.Base` 通过 embedding 提供统一生命周期
5. **Formatter 解耦**：消息格式化与模型实现分离，通过 interface 注入
6. **Workspace 沙箱**：工具执行隔离在 Local/Docker/E2B 环境中
7. **可观测性对齐**：TracingMiddlewareAdapter 实现完整 middleware 接口，支持 agent 生命周期 tracing（on_reply/on_reasoning/on_acting/on_model_call/on_system_prompt），与 Python _tracing/ 对齐；结合 TracedAgent + OTel/LangSmith。
8. **GEP 自演化对齐 (Phase 6)**：通过 evolver/ 包引入 Gene（紧凑策略模板，优于 ad-hoc skill）/ Capsule（演化快照）/ 演化闭环（run/reflect/solidify）、typed remember/recall（narrative + memoryGraph 风格）、skill2gep 蒸馏、meeting/ATP 任务。利用现有 gateway MCP 网关 + ReMe + a2a 实现“轻量桥接”，不重造 evolver 引擎。Mock + Recording 便于测试/可见性。
