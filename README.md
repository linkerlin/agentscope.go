# agentscope.go

[AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现 —— 一个生产级的 AI Agent 开发框架，助你使用 Go 构建基于大语言模型的智能应用。

## 概述

AgentScope Go 提供了构建智能 Agent 所需的一切，采用 ReAct（推理 + 行动）范式：工具调用、记忆管理、多 Agent 协作等功能一应俱全，并且全部使用地道的 Go 语言惯用法实现。

## 快速开始

**环境要求：** Go 1.25 或更高版本

```bash
go get github.com/linkerlin/agentscope.go
```

```go
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
        TextContent("Hello! What can you help me with?").
        Build())

    fmt.Println(response.GetTextContent())
}
```

## 支持的模型

| 提供商 | 包路径 | 说明 |
|--------|--------|------|
| OpenAI Chat | `github.com/linkerlin/agentscope.go/model/openai` | GPT-4o / GPT-4o-mini / o1 / o3 等（Chat Completions API） |
| OpenAI Response | `github.com/linkerlin/agentscope.go/model/openai_response` | o3 / o4-mini 等（Responses API，支持 reasoning 流） |
| Anthropic | `github.com/linkerlin/agentscope.go/model/anthropic` | Claude 3.5 Sonnet / Opus / Haiku 原生 HTTP + SSE |
| Gemini | `github.com/linkerlin/agentscope.go/model/gemini` | Gemini 1.5 Flash / Pro 原生 HTTP + SSE |
| DashScope (阿里云) | `github.com/linkerlin/agentscope.go/model/dashscope` | 通义千问系列（OpenAI 兼容） |
| DeepSeek | `github.com/linkerlin/agentscope.go/model/deepseek` | DeepSeek-V3 / Coder / Reasoner（OpenAI 兼容） |
| Moonshot | `github.com/linkerlin/agentscope.go/model/moonshot` | Kimi 系列（OpenAI 兼容） |
| xAI | `github.com/linkerlin/agentscope.go/model/xai` | Grok 系列（OpenAI 兼容） |
| vLLM | `github.com/linkerlin/agentscope.go/model/vllm` | 私有化部署（OpenAI 兼容） |
| Ollama | `github.com/linkerlin/agentscope.go/model/ollama` | 本地开源模型（OpenAI 兼容） |

任何兼容 OpenAI API 格式的服务都可以通过 `BaseURL` 配置使用。

### 按模型提供商快速上手

[`scripts/model_examples/`](scripts/model_examples/) 提供每家模型提供商的最小可运行脚本，以及多 Agent、事件流、多模态等场景示例：

```bash
cd scripts/model_examples/openai_chat_call
export OPENAI_API_KEY=sk-...
go run .
```

目前已覆盖：OpenAI Chat / OpenAI Response / Anthropic / Gemini / DashScope / DeepSeek / Moonshot / xAI / Ollama / vLLM，以及 multiagent / stream / multimodal。

### Cookbook

[`cookbook/`](cookbook/) 提供可复用的解决方案 recipes：

- 长文档摘要（MapReduce）
- 多 Agent 审稿（Reflection + Pipeline）
- RAG 问答（Loader + Embedding + ReMe）
- 定时报告 Agent（Schedule）
- 自愈 Agent（GEP / Evolver）

## 核心包

| 包名 | 说明 |
|------|------|
| `message` | `Msg` 类型，支持多模态内容块（文本、图片、音频、视频、工具调用/结果、思考过程） |
| `model` | `ChatModel` 接口，支持流式响应 |
| `agent` | `Agent` 基础接口与 `Base` 统一生命周期（Hook、流式事件、Usage 统计） |
| `agent/react` | ReAct Agent 实现，内嵌 `agent.Base` |
| `memory` | `Memory` 接口 + 5 实现（InMemory/Window/ReMeInMemory/ReMeFile/ReMeVector） + 7 向量后端 + Hybrid Search(BM25+Reranker) + Dream 演化 + 知识图谱 |
| `tool` | `Tool` 接口 + 内置工具（file: Read/Write/Edit/Glob/Grep, shell, web, json, multimodal + Task/Schedule/Subagent） |
| `formatter` | 独立的模型请求/响应格式化抽象层（OpenAI / Anthropic / Gemini / DashScope / Ollama） |
| `pipeline` | 多 Agent 编排：Pipeline（顺序）+ Parallel（并发） |
| `msghub` | 广播式多 Agent 消息调度（Hub） |
| `workflow` | 高级多 Agent 编排：条件（Condition）、循环（Loop）、MapReduce |
| `reflection` | Agent 自省/反思模式：Writer + Critic 循环迭代优化 |
| `a2a` | A2A 协议实现：AgentCard、Task、SSE、Registry |
| `gateway` | HTTP + SSE + WebSocket Gateway，支持多租户认证 + Session 持久化 |
| `service` | 多租户 Service 层：Storage + Auth + Credential 加密 |
| `schedule` | Cron 定时任务调度器 |
| `async` | 异步任务执行池 |
| `loader` | 文档加载器（TextLoader / DirLoader） |
| `observability` | OpenTelemetry + LangSmith 可观测性 |
| `session` | 会话管理 |
| `hook` | 钩子系统，支持人机协作 |
| `plan` | PlanNotebook，用于结构化多步骤任务管理 |
| `embedding` | 独立 Embedding 包：OpenAI / Ollama / Gemini / DashScope / DashScope多模态 + FileCache，可直接用于 gateway / memory / RAG |
| `evolver` | GEP Gene/Capsule 类型 + Evolver 客户端 + Run/Reflect/Solidify 流程 + Skill→Gene 蒸馏（Phase 6 对齐 evolver 优势） |
| `embedding/onnx` | ONNX 本地推理：CLIP 图像嵌入 + Whisper 音频嵌入 + 模型管理器（HTTP 代理方案，零 CGO 依赖） |

## ONNX 生产化（多模态本地推理）

无需 Python 环境，纯 Go 实现图像/音频预处理管道，通过 HTTP 代理连接 ONNX Runtime 服务：

```go
import "github.com/linkerlin/agentscope.go/embedding/onnx"

// 图像预处理（CLIP）→ 输出 NCHW [1,3,224,224]
preprocessor := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())
vec, _ := preprocessor.Preprocess(imageReader)

// 音频预处理（Whisper）→ 输出 Mel 频谱图 [1,80,3000]
audioProc := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())
mel, _ := audioProc.Preprocess(pcmSamples, 16000)

// CLIP 图像嵌入器（HTTP 代理）
clip := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
embedding, _ := clip.EmbedImage(vec)

// 跨模态相似度（图像-文本对齐）
sim, _ := onnx.CrossModalSimilarity(imageEmbedding, textEmbedding)

// 模型管理器：自动下载/缓存/版本管理
manager, _ := onnx.NewModelManager(onnx.DefaultModelManagerConfig())
manager.RegisterModel(onnx.PredefinedModels()[0]) // CLIP ViT-B/32
```

## A2A 增强（认证 + 限流 + WebSocket）

A2A 协议完整实现，新增生产级安全与实时通信能力：

```go
import "github.com/linkerlin/agentscope.go/a2a"

// 安全服务器：认证 + 限流 + CORS + 日志
server := a2a.NewSecureServer(card, runner, store)
server.auth.AddAPIKey("sk-xxx", "production-client")
server.WithRateLimit(a2a.NewRateLimiter(100, 200)) // 100 req/s, burst 200

// WebSocket 实时任务推送
wsServer := a2a.NewWebSocketEnabledServer(card, runner, store)
// 客户端通过 WebSocket 订阅任务状态，实时接收 task_update 事件
```

## 性能基准

| 测试项 | 性能 | 说明 |
|--------|------|------|
| 嵌入缓存命中 | 550 ns/op | 内存 LRU，无锁读 |
| 跨模态相似度 | 741 ns/op | 512 维余弦相似度 |
| 向量存储搜索（1000 节点） | 229 μs/op | 暴力搜索 + HNSW 自动切换 |
| FTS 全文搜索（1000 文档） | 97 μs/op | FTS5 trigram + CJK 回退 |
| ReMe 文件记忆添加 | 463 μs/op | 含持久化写入 |
| ReAct 记忆注入 | 132 μs/op | 向量检索 + 格式化 + 注入 |
| ONNX 图像预处理 | 3.5 ms/op | 1024×768 → 224×224 + 归一化 |
| ONNX 音频预处理 | ~9.7 s/op | 30s 音频 → Mel 频谱图（可优化） |

运行基准：`go test ./memory/... -run=^$ -bench=. -benchtime=1s`

## 高层生产服务 Bootstrap（强烈推荐）

`gateway.AppConfig` + `NewApp` 提供接近 Python `create_app` + lifespan 的“一键”体验，支持大量自动装配：

```go
appCfg := gateway.AppConfig{
    Agent:                 myAgent,
    Storage:               service.NewMemoryStorage(),
    JWTAuth:               jwtAuth,
    WorkspaceBaseDir:      "./workspaces",
    AutoStandardTools:     true,           // 自动为 sessions 注入 file+task+web+json+schedule 等
    AutoToolOffload:       true,
    DefaultPermissionMode: permission.ModeExplore,
    EmbeddingModel:        embedding.NewOpenAI(apiKey, "text-embedding-3-small"),
    EmbeddingCacheDir:     "./.embed_cache", // 自动 WithFileCache
}
srv := gateway.NewApp(appCfg)
srv.RegisterAppRoutes(jwtAuth)
srv.Start()   // 自动恢复 persisted schedules
defer srv.Close()
```

详见 `examples/full_service` 和 `examples/production`。

## Embedding 包

独立使用：

```go
import "github.com/linkerlin/agentscope.go/embedding"

emb := embedding.NewOpenAI(os.Getenv("OPENAI_API_KEY"), "text-embedding-3-small")
emb = embedding.WithFileCache(emb, ".cache/embeddings") // 可选

vecs, _ := emb.Embed(ctx, []string{"hello world"})
```

支持 Gemini / DashScope (含多模态提示)。

## 使用工具

```go
import "github.com/linkerlin/agentscope.go/tool"

myTool := tool.NewFunctionTool(
    "weather",
    "获取指定城市的当前天气",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "city": map[string]any{"type": "string"},
        },
        "required": []string{"city"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        city := input["city"].(string)
        return fmt.Sprintf("%s 天气晴朗，22°C", city), nil
    },
)

agent, _ := react.Builder().
    Name("WeatherBot").
    Model(chatModel).
    Tools(myTool).
    Build()
```

## 记忆管理

### 基础 Memory

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

### ReMe 长期记忆（文件 + 向量）

```go
import "github.com/linkerlin/agentscope.go/memory"
import "github.com/linkerlin/agentscope.go/memory/handler"

// 创建向量记忆
v, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embedModel)

// 注入编排器，实现自动提取与检索
orch := handler.NewMemoryOrchestrator(personalSum, proceduralSum, toolSum, memTool, profileTool, historyTool, dedup)
v.SetOrchestrator(orch)

// 端到端自动提取个人/任务记忆并写入向量库
res, _ := v.SummarizeMemory(ctx, msgs, "alice", "coding_task", "")

// 统一检索
nodes, _ := v.RetrieveMemoryUnified(ctx, "Go 最佳实践", "alice", "coding_task", "", memory.RetrieveOptions{TopK: 5})
```

### ReAct 记忆注入编排

```go
import "github.com/linkerlin/agentscope.go/memory"

// 创建 ReAct 步级记录器
recorder := memory.NewReactStepRecorder(memory.NewInMemoryStepStore())

// 创建记忆注入编排器（4 种策略：recent/targeted/personal/hybrid）
orchestrator := memory.NewReactOrchestrator(recorder, store, memory.DefaultReactOrchestratorConfig())

// 在 ReAct 循环中注入相关记忆
memNodes, sysMsg, _ := orchestrator.InjectMemory(ctx, query, history, "alice", "coding_task")

// 复盘提取：成功路径 / 失败教训 / 新知识
replay := memory.NewReactReplayExtractor(memory.DefaultReactReplayConfig())
result, _ := replay.Replay(ctx, steps)
```

## 钩子系统（人机协作）

```go
import "github.com/linkerlin/agentscope.go/hook"

loggingHook := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
    fmt.Printf("[%s] Agent: %s\n", hCtx.Point, hCtx.AgentName)
    return nil, nil
})

agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Hooks(loggingHook).
    Build()
```

## 计划笔记本

```go
import "github.com/linkerlin/agentscope.go/plan"

notebook := plan.NewPlanNotebook()
p := notebook.CreatePlan("研究任务")
notebook.AddStep(p.ID, "搜索信息")
notebook.AddStep(p.ID, "总结发现")

// 作为工具在 Agent 中使用
agent, _ := react.Builder().
    Name("Planner").
    Model(chatModel).
    Tools(notebook.AsTool()).
    Build()
```

## 多模型后端示例

### Anthropic

```go
import "github.com/linkerlin/agentscope.go/model/anthropic"

chatModel, _ := anthropic.NewBuilder().
    APIKey(os.Getenv("ANTHROPIC_API_KEY")).
    ModelName("claude-3-5-sonnet-20241022").
    Build()
```

### Gemini

```go
import "github.com/linkerlin/agentscope.go/model/gemini"

chatModel, _ := gemini.NewBuilder().
    APIKey(os.Getenv("GEMINI_API_KEY")).
    ModelName("gemini-1.5-flash").
    Build()
```

### DashScope（阿里云）

```go
import "github.com/linkerlin/agentscope.go/model/dashscope"

chatModel, _ := dashscope.Builder().
    APIKey(os.Getenv("DASHSCOPE_API_KEY")).
    ModelName("qwen-max").
    Build()
```

### Ollama

```go
import "github.com/linkerlin/agentscope.go/model/ollama"

chatModel, _ := ollama.NewBuilder().
    BaseURL("http://127.0.0.1:11434/v1").
    ModelName("llama3.2").
    Build()
```

### DeepSeek

```go
import "github.com/linkerlin/agentscope.go/model/deepseek"

chatModel, _ := deepseek.Builder("sk-...").
    ModelName(deepseek.ModelChat).
    Build()
```

### Moonshot (Kimi)

```go
import "github.com/linkerlin/agentscope.go/model/moonshot"

chatModel, _ := moonshot.Builder("sk-...").
    ModelName(moonshot.Model8K).
    Build()
```

### xAI (Grok)

```go
import "github.com/linkerlin/agentscope.go/model/xai"

chatModel, _ := xai.Builder("xai-...").
    ModelName(xai.ModelGrok2).
    Build()
```

### vLLM

```go
import "github.com/linkerlin/agentscope.go/model/vllm"

chatModel, _ := vllm.Builder("http://localhost:8000/v1", "sk-...").
    ModelName("meta-llama/Meta-Llama-3-8B-Instruct").
    Build()
```

### OpenAI Response API

```go
import "github.com/linkerlin/agentscope.go/model/openai_response"

chatModel, _ := openai_response.Builder().
    APIKey(os.Getenv("OPENAI_API_KEY")).
    ModelName("o3").
    ThinkingEnable(true).
    ReasoningEffort("medium").
    Build()
```

## 多 Agent 编排

### 顺序执行（Pipeline）

```go
import "github.com/linkerlin/agentscope.go/pipeline"

pipe := pipeline.New("ResearchPipe", plannerAgent, writerAgent)
resp, _ := pipe.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent("Go 并发模式").Build())
```

### 广播调度（MsgHub）

```go
import "github.com/linkerlin/agentscope.go/msghub"

hub := msghub.New()
hub.Register("coder", coderAgent)
hub.Register("reviewer", reviewerAgent)
results := hub.Broadcast(ctx, msg) // map[string]*message.Msg
```

### 并行 / 条件 / 循环（Workflow）

```go
import "github.com/linkerlin/agentscope.go/workflow"

// 并行：让两个 Agent 同时处理，合并结果
par := workflow.NewParallel("DualCheck", nil, agentA, agentB)

// 条件：根据输入内容决定走哪个分支
cond := workflow.NewCondition("Router",
    func(m *message.Msg) bool { return strings.Contains(m.GetTextContent(), "urgent") },
    urgentAgent, normalAgent)

// 循环：反复优化直到满足质量条件
loop := workflow.NewLoop("Refiner", editorAgent,
    func(m *message.Msg) bool { return !strings.Contains(m.GetTextContent(), "FINAL") },
    5)
```

## 实时对话 Gateway

```go
import "github.com/linkerlin/agentscope.go/gateway"

srv := gateway.NewServer(agent)
http.ListenAndServe(":8080", srv)
```

- `POST /chat` —— 非流式对话，请求体 `{"text":"..."}`，返回 JSON。
- `POST /chat/stream` —— SSE 流式对话，浏览器可用 `EventSource` 接收增量回复。
- `GET /chat/ws` —— WebSocket 流式对话，支持双向实时交互。

## MapReduce 工作流

```go
import "github.com/linkerlin/agentscope.go/workflow"

mr := workflow.NewMapReduce(
    "DocSummary",
    func(m *message.Msg) []string { return splitIntoParagraphs(m.GetTextContent()) },
    summarizerAgent, // mapper
    synthesizerAgent, // reducer
    4, // parallelism
)
```

输入被 `split` 成多个 chunk，每个 chunk 由 `mapper` 并行处理，最后由 `reducer` 汇总为单一结果。

## Agent 自省/反思模式

```go
import "github.com/linkerlin/agentscope.go/reflection"

agent := reflection.NewSelfReflectingAgent(
    "RefiningWriter",
    writerAgent,
    criticAgent,
    func(_, critique *message.Msg) bool {
        return strings.Contains(critique.GetTextContent(), "PASS")
    },
    3, // max iterations
)

resp, _ := agent.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
```

## 示例

- [`examples/a2a`](examples/a2a/main.go) —— A2A 协议基础：AgentCard、Task、SSE
- [`examples/a2a_redis_registry`](examples/a2a_redis_registry/main.go) —— A2A Redis 分布式注册中心
- [`examples/embedding`](examples/embedding/main.go) —— 独立 Embedding 包（OpenAI/Ollama + FileCache）
- [`examples/schedule`](examples/schedule/main.go) —— Cron 定时任务调度器
- [`examples/rag`](examples/rag/main.go) —— RAG 问答完整流程（Loader + Embedding + ReMe）
- [`examples/observability`](examples/observability/main.go) —— OpenTelemetry + LangSmith 追踪
- [`examples/state`](examples/state/main.go) —— AgentState 持久化（JSONFile/Redis）
- [`examples/a2a_secure`](examples/a2a_secure/main.go) —— A2A 认证 + 限流 + WebSocket
- [`examples/memory_benchmark`](examples/memory_benchmark/main.go) —— 记忆系统基准测试运行
- [`examples/onnx`](examples/onnx/main.go) —— ONNX 图像/音频预处理与嵌入
- [`examples/react_orchestrator`](examples/react_orchestrator/main.go) —— ReAct 记忆注入编排
- [`examples/cross_modal`](examples/cross_modal/main.go) —— 跨模态检索（文本→图像/音频）
- [`examples/multimodal`](examples/multimodal/main.go) —— 多模态 Agent（图像/音频输入）
- [`examples/multimodal_router`](examples/multimodal_router/main.go) —— 多模态路由自动切换
- [`examples/middleware`](examples/middleware/main.go) —— Agent 生命周期中间件链
- [`examples/interrupt`](examples/interrupt/main.go) —— 中断处理与暂停恢复
- [`examples/trace`](examples/trace/main.go) —— 事件追踪与 Hook 系统
- [`examples/web_ui`](examples/web_ui/main.go) —— Web UI 实时对话
- [`examples/hello`](examples/hello/main.go) —— Agent 基础用法
- [`examples/tools`](examples/tools/main.go) —— 带计算工具的 Agent
- [`examples/v2_event_stream`](examples/v2_event_stream/main.go) —— V2 事件流完整生命周期演示
- [`examples/anthropic`](examples/anthropic/main.go) —— 使用 Claude 后端的 Agent
- [`examples/gemini`](examples/gemini/main.go) —— 使用 Gemini 后端的 Agent
- [`examples/pipeline`](examples/pipeline/main.go) —— 多 Agent 顺序编排（Pipeline）
- [`examples/msghub`](examples/msghub/main.go) —— 广播式多 Agent 消息调度
- [`examples/workflow`](examples/workflow/main.go) —— 并行 + 条件 + 循环工作流
- [`examples/gateway`](examples/gateway/main.go) —— HTTP + SSE 实时对话 Gateway
- [`examples/reflection`](examples/reflection/main.go) —— Writer + Critic 自我反思迭代
- [`examples/mapreduce`](examples/mapreduce/main.go) —— MapReduce 长文档摘要
- [`examples/reme/file`](examples/reme/file/main.go) —— ReMe 文件型记忆（ReMeLight）
- [`examples/reme/vector`](examples/reme/vector/main.go) —— ReMe 向量记忆检索
- [`examples/reme/orchestrator`](examples/reme/orchestrator/main.go) —— ReMe Orchestrator 端到端（提取 + 检索 + Profile）
- [`examples/voice`](examples/voice/main.go) —— STT → Chat → TTS 语音对话 Pipeline
- [`examples/multi_tenant_workspace`](examples/multi_tenant_workspace/main.go) —— 多租户认证 + Workspace + 权限引擎端到端
- [`examples/production`](examples/production/main.go) —— 全功能生产级服务（Auth + 工具 + 权限 + Gateway）
- [`examples/full_service`](examples/full_service/main.go) —— 极简重度自动装配生产服务（推荐）
- [`examples/studio`](examples/studio/main.go) —— 纯 Go 轻量 Studio (HTMX) —— 完整 Auth/Agents/Credentials/Schedules/Chat + 实时 SSE + auto tools 结果展示
- [`examples/evolver`](examples/evolver/main.go) —— GEP 自演化 demo（Gene/Capsule、Run/Reflect/Solidify 闭环、Skill 蒸馏、Recording 调用、演化记忆 recall）
- [`examples/langsmith`](examples/langsmith/main.go) —— Agent 事件流转发到 LangSmith

## 可观测性

### 追踪中间件（Phase 5 新增，对齐 Python）

使用 `TracingMiddlewareAdapter` 可在 Agent 生命周期（on_reply、on_reasoning、on_acting、on_model_call、on_system_prompt）注入 tracing spans。

```go
import (
    "github.com/linkerlin/agentscope.go/observability"
    "github.com/linkerlin/agentscope.go/agent/react"
)

tracer := observability.NewOTelTracer(...) // 或 LangSmith tracer 等
tracingMW := &observability.TracingMiddlewareAdapter{
    Tracer: tracer,
    Name:   "my-agent",
}

agent, _ := react.Builder().
    Name("TracedAgent").
    Model(chatModel).
    Middlewares(tracingMW).  // 直接用于 middleware 链
    Build()
```

也可用 `TracedAgent` 包装：

```go
traced := observability.NewTracedAgent("my-agent", baseAgent).WithTracer(tracer)
```

支持 RecordingTracer 用于调试（见 examples/full_service）。

### LangSmith 追踪

```go
import (
    "github.com/linkerlin/agentscope.go/event"
    "github.com/linkerlin/agentscope.go/observability"
)

client := observability.NewLangSmithClient(os.Getenv("LANGSMITH_API_KEY"))
observer := observability.NewLangSmithObserver(client, "my-project", "session-1")

bus := event.NewBus(100)
go observer.Observe(ctx, bus)

agent, _ := react.Builder().
    Name("TracedAgent").
    Model(chatModel).
    WithEventBus(bus).
    Build()
```

Agent 运行期间的所有事件（`ReplyStart`、`TextBlockDelta`、`ToolCallStart`、`ToolCallEnd`、`ReplyEnd` 等）将自动上报到 LangSmith，生成完整的调用链追踪。

### OpenTelemetry

```go
import "github.com/linkerlin/agentscope.go/observability"

tp, _ := observability.InitTracerProvider("agent-service")
defer tp.Shutdown(context.Background())
```

Gateway 自动集成 OTel HTTP 中间件，所有请求都会被追踪。Toolkit 层也有 TracingMiddleware。

## GEP 自演化与 Evolver 对齐（Phase 6 新阶段）

agentscope.go 现在对齐了 [Evolver](https://github.com/EvoMap/evolver)（及 evolver.py）的核心优势——**基于 GEP（Gene Evolution Protocol）的自演化能力**。

### 为什么对齐 Evolver？
Evolver 的论文与实践表明：**紧凑的 Gene（策略基因，带 signals_match + strategy + constraints + validation）** 作为演化资产，远优于松散的 ad-hoc prompt / skill 文档。它提供：
- 可审计、可复用、可固化的演化闭环（run → reflect → solidify）
- Capsule（成功快照，含 blast_radius、execution_trace、outcome）
- 带类型的记忆（remember/recall gene/capsule/event，支持 narrativeMemory / memoryGraph）
- 结构化会议（meeting_start / proceed / human_input / finalize）
- ATP 任务市场（claim / complete + hub 复用）
- Skill ↔ Gene 蒸馏（skill2gep / distiller / publisher）
- 安全与回滚（safety_status、policy、gitOps rollback）

agentscope.go 已有 ReMe（世界级记忆）、a2a（领先协议）、gateway（MCP 完美桥接）、事件+tracing 基础设施，因此我们**轻量桥接**而非重造：
- 提供原生 Go 类型 + 高层流程 API
- 通过现有 MCP 网关即可让你的 Agent 直接调用 evolver 全部工具
- ReMe + 新 MemoryTypeGene/Capsule 天然承载演化资产
- Skill 蒸馏 + Recording 风格可见性（类似 Phase 5 tracing）

### 快速使用（Mock 先行，生产接 MCP）
```go
import "github.com/linkerlin/agentscope.go/evolver"

flow := evolver.NewGEPFlow(evolver.NewMockEvolver()) // 生产：换成真实 backed client
runCfg := evolver.RunConfig{Context: "recurring gateway timeout on large payload", Strategy: "repair-only"}
runRes, solRes, _ := flow.RunAndSolidify(ctx, runCfg, false /* 设 true 干运行 */)

fmt.Println("Selected gene:", runRes.SelectedGene.ID)
fmt.Println("Solidified capsule:", solRes.CapsuleID)

// Skill 蒸馏为 Gene（对齐 skillDistiller）
sk := &skill.AgentSkill{Name: "timeout_recovery", Description: "...", SkillContent: "..."}
gene := sk.DistillToGene(evolver.CategoryRepair)
flow.Client.UpsertGene(ctx, gene)

// 演化记忆 recall（narrative 风格）
_ = flow.Client.Remember(ctx, evolver.RememberRequest{Text: "...", Type: "capsule", Category: evolver.CategoryRepair})
hits, _ := flow.Client.Recall(ctx, evolver.RecallRequest{Query: "timeout", Category: "capsule"})
```

**生产集成**：在 `gateway.AppConfig{EvolverEnabled: true}` 启动后，session agent 即可通过 MCP 工具调用 `evolver__evolver_run` / `evolver_solidify` 等（见 gateway/session_mcp_gateway）。配合 ReMe 持久化 gene/capsule 资产，即可实现“遇到错误自动触发 GEP 修复 + 固化 + 审计”。

详见：
- `evolver/` 包（types, client, gep flow, tests）
- `examples/evolver/main.go`（完整可运行 demo，含 recording calls、distill、recall）
- `DEV_PLAN_CATCHUP.md` Phase 6 章节（含优势对比、实施策略）
- evolver 官方：基因优于 skill 的论文 arXiv:2604.15097

未来将持续增强：真实 MCP 客户端包装、Studio 演化资产 UI、a2a ATP 任务扩展、ReMe 深度 memoryGraph 实现。

## 部署与迁移

- 生产部署指南：[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- 从 Python AgentScope 或旧版本迁移：[MIGRATION.md](MIGRATION.md)
- 版本发布流程：[RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md)

## 贡献与社区

我们欢迎所有形式的贡献！

- 贡献指南：[CONTRIBUTING.md](CONTRIBUTING.md)
- 行为准则：[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 安全漏洞报告：[SECURITY.md](SECURITY.md)
- 当前任务与路线图：[TODO.md](TODO.md)
- 详细演进方案：[演进方案.md](演进方案.md)

如果你在使用过程中遇到问题，请先查看 [docs/](docs/) 和 [examples/](examples/)，然后提交 Issue。

## 许可证

Apache License 2.0 —— 详见 [LICENSE](LICENSE) 文件。
