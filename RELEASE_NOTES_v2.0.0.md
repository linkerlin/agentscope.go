# AgentScope.Go v2.0.0 Release Notes

> 🎉 **AgentScope.Go 正式发布 v2.0.0** —— 从"Python 版的高性能替代实现"升级为"面向云原生、高并发、长记忆、自演化 Agent 服务的首选 Go 框架"。

---

## 核心亮点

### 1. 生产级服务化（对齐 Python 2.0）
- **`gateway.NewApp(AppConfig{})`** 一键装配：Storage + SessionManager + BackgroundTaskManager + WorkspaceManager + ToolOffload + StandardTools 自动注入
- **多租户认证**：JWT + API Key 双轨，AES-256-GCM 加密凭证
- **Cron 调度器**：基于 `robfig/cron/v3`，支持 stateful/stateless 执行，自动持久化恢复
- **Redis 存储**：完整 CRUD + 索引 + TTL，支持会话重放

### 2. V2 事件驱动范式
- **真事件流**：`ReplyStream() -> <-chan event.AgentEvent`，Channel 驱动替代 AsyncGenerator
- **20+ 事件类型**：TextBlockDelta / ThinkingBlockDelta / ToolCallDelta / DataBlockDelta 等，与 Python PyV2 JSON 完全对齐
- **AG-UI Protocol**：`gateway/agui.go` 完整映射，Python Studio UI 即插即用
- **AgentState 状态机**：可序列化到 Redis/JSONFile，支持 HITL 挂起/恢复

### 3. ReMe 长期记忆系统（Go 独有领先）
- **文件型 + 向量型**：ReMeLight + ReMeVectorMemory
- **5 向量后端**：Local / Chroma / Qdrant / Milvus / Pgvector / Elasticsearch
- **Hybrid Search**：向量 + 全文（BM25/FTS5）混合检索
- **Orchestrator**：自动提取个人/任务/工具/画像记忆
- **Rerank**：Cohere / Jina / Local cosine 二阶段精排

### 4. A2A 协议完整实现（Go 独有领先）
- **AgentCard / Task / SSE / Registry** 全栈实现
- **Redis 分布式注册中心**：一致哈希分片 + Watch 故障转移
- **ShardRouter**：健康感知路由，自动故障切换

### 5. GEP 自演化（Go 独有领先）
- **Gene / Capsule 类型**：对齐 Evolver 论文，支持 signals_match + strategy + constraints
- **Run / Reflect / Solidify 闭环**：完整自演化流程
- **Skill → Gene 蒸馏**：将 ad-hoc skill 转化为可演化资产
- **ReMe 承载**：MemoryTypeGene / Capsule / EvoEvent 支持 narrativeMemory / memoryGraph

### 6. 多 Agent 编排（Go 独有领先）
- **Pipeline**：顺序执行
- **Parallel / MsgHub**：并发 + 广播
- **Workflow**：MapReduce / Condition / Loop
- **Reflection**：Writer + Critic 迭代优化
- **SubagentTool**：递归 Agent 调用（Python 无对等实现）

### 7. 模型支持
- **10 个后端**：OpenAI Chat / OpenAI Response / Anthropic / Gemini / DashScope / DeepSeek / Moonshot / xAI / vLLM / Ollama
- **ModelCard YAML**：35 个声明式配置 + `/api/v1/models` API
- **AudioModel**：TTS 接口预留 + OpenAITTS 实现
- **Embedding**：OpenAI / Ollama / Gemini / DashScope（多模态）+ FileCache

### 8. 可观测性
- **OpenTelemetry**：完整追踪
- **LangSmith**：事件流自动上报
- **TracingMiddlewareAdapter**：5 个拦截点（on_reply / on_reasoning / on_acting / on_model_call / on_system_prompt）

---

## 与 Python AgentScope 2.0 功能对照

| 特性 | Python 2.0 | Go 2.0.0 | 状态 |
|------|-----------|----------|------|
| ReAct Agent | ✅ | ✅ | 对齐 |
| 事件流 | ✅ AsyncGenerator | ✅ Channel | Go 更地道 |
| 工具系统 | ✅ | ✅ | 对齐 |
| MCP 集成 | ✅ | ✅ | 对齐 |
| 上下文压缩 | ✅ | ✅ | 对齐 |
| 结构化输出 | ✅ | ✅ | 对齐 |
| 权限引擎 | ✅ | ✅ | 对齐 |
| Workspace 沙箱 | ✅ Local/Docker/E2B | ✅ + Offloader | Go 更完整 |
| 长期记忆 | ❌ 临时移除 | ✅ ReMe 5+后端 | **Go 领先** |
| A2A 协议 | ❌ Roadmap | ✅ 完整实现 | **Go 领先** |
| 多 Agent 编排 | ❌ 无独立模块 | ✅ Pipeline/Workflow/MapReduce | **Go 领先** |
| GEP 自演化 | ❌ 无 | ✅ Gene/Capsule/Skill2GEP | **Go 领先** |
| 语音/TTS | ❌ Roadmap | ✅ AudioModel + voice 示例 | **Go 领先** |
| Subagent 递归 | ❌ 无 | ✅ SubagentTool | **Go 领先** |
| Web UI / Studio | ✅ React 完整应用 | ⚠️ HTMX 轻量实现 | Python 领先 |
| 文档站点 | ✅ docs.agentscope.io | ⚠️ 仓库内 docs | Python 领先 |
| 模型示例脚本 | ✅ 36 个 | ⚠️ 14 个 | Python 领先 |
| 社区规模 | ✅ ~50 贡献者 | ⚠️ 早期 | Python 领先 |

---

## 安装

```bash
go get github.com/linkerlin/agentscope.go@v2.0.0
```

快速开始：

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    chatModel, _ := openai.Builder().
        APIKey(os.Getenv("OPENAI_API_KEY")).
        ModelName("gpt-4o-mini").
        Build()

    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful AI assistant.").
        Model(chatModel).
        Build()

    response, _ := agent.Call(context.Background(), message.NewMsg().
        Role(message.RoleUser).
        TextContent("Hello!").
        Build())

    fmt.Println(response.GetTextContent())
}
```

---

## 生产级服务一键启动

```go
srv := gateway.NewApp(gateway.AppConfig{
    Storage:          service.NewRedisStorage("localhost:6379"),
    WorkspaceBaseDir: "./workspaces",
    AutoStandardTools: true,
    AutoToolOffload:   true,
    EmbeddingCacheDir: "./.embed_cache",
})
srv.RegisterAppRoutes(jwtAuth)
srv.Start()
defer srv.Close()
```

---

## 质量指标

| 指标 | 数值 |
|------|------|
| 测试通过率 | ✅ 100% (`go test ./... -race -count=1`) |
| 测试覆盖率 | 60.5% |
| CI 平台 | Ubuntu / Windows / macOS |
| 代码规范 | gofmt 0 漂移 / golangci-lint 通过 |
| 安全扫描 | govulncheck 通过 |
| 基准测试 | 17 个 benchmark 覆盖 formatter |

---

## 迁移指南

- 从 Python AgentScope 2.0 迁移：[MIGRATION.md](https://github.com/linkerlin/agentscope.go/blob/main/MIGRATION.md)
- 从 v1.x / v2-rc 迁移：[MIGRATION.md#从-v2.0.0-rc.x-迁移到-v2.0.0](https://github.com/linkerlin/agentscope.go/blob/main/MIGRATION.md)

---

## 文档

- [README.md](https://github.com/linkerlin/agentscope.go/blob/main/README.md) — 项目概览与快速开始
- [docs/tutorial.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/tutorial.md) — 5 步教程
- [docs/deployment.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/deployment.md) — 生产部署指南
- [docs/api-reference.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/api-reference.md) — API 参考（761 行）
- [docs/A2A.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/A2A.md) — A2A 协议指南
- [docs/EVOLVER.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/EVOLVER.md) — GEP 自演化教程
- [docs/STUDIO.md](https://github.com/linkerlin/agentscope.go/blob/main/docs/STUDIO.md) — Studio UI 指南

---

## 示例

| 示例 | 路径 | 说明 |
|------|------|------|
| Hello Agent | `examples/hello` | 基础用法 |
| 工具调用 | `examples/tools` | 带计算工具的 Agent |
| V2 事件流 | `examples/v2_event_stream` | 完整事件生命周期 |
| 生产服务 | `examples/full_service` | 一键装配生产级服务 |
| Studio UI | `examples/studio` | 纯 Go HTMX 轻量 Studio |
| ReMe 记忆 | `examples/reme/orchestrator` | 端到端记忆提取与检索 |
| A2A 分布式 | `examples/a2a_redis_registry` | Redis 注册中心 + 分片路由 |
| GEP 自演化 | `examples/evolver` | Gene/Capsule/Skill 蒸馏 |
| 语音对话 | `examples/voice` | STT → Chat → TTS |
| MapReduce | `examples/mapreduce` | 长文档摘要 |

---

## 贡献与社区

- [CONTRIBUTING.md](https://github.com/linkerlin/agentscope.go/blob/main/CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](https://github.com/linkerlin/agentscope.go/blob/main/CODE_OF_CONDUCT.md)
- [SECURITY.md](https://github.com/linkerlin/agentscope.go/blob/main/SECURITY.md)

---

## 许可证

Apache License 2.0

---

**Full Changelog**: [v2.0.0-rc.1...v2.0.0](https://github.com/linkerlin/agentscope.go/compare/v2.0.0-rc.1...v2.0.0)
