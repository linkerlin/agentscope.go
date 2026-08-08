# agentscope.go

AgentScope.Go — a production-grade AI Agent development framework that lets you build LLM-powered applications in idiomatic Go.

> **Simplified Chinese**: [README.md](README.md)

## Overview

AgentScope.Go provides everything needed to build intelligent agents using the ReAct (reason + act) paradigm: tool calling, memory management, multi-agent collaboration, **multi-platform chatbots (Webhook / Discord / Feishu)**, and an **ecosystem marketplace (MCP / Skill)** — all in idiomatic Go.

## What's New (v2.5.0)

<!-- BEGIN NEWS -->
- **`Channel` multi-platform integration**: **3 platform adapters out of the box** — Webhook (zero-dependency HTTP), Discord (discordgo), Feishu (pure HTTP, incl. `send_message`/`list_chats` agent tools); chat→agent routing + async runs + reply delivery.
- **`Hub` marketplace**: browse + install MCP/Skill cards (FSHub — a directory is a marketplace, zip-slip hardened).
- **`Plugin` ecosystem example**: three-phase lifecycle + YAML config + tool registration (`examples/plugin_demo`).
- **`RAG` managed knowledge bases**: document→parser(Text/PDF/PPTX/Image)→chunker→blob→kb→index pipeline + RAGMiddleware + KB HTTP API. Aligned with Python rag/ hosting.
- **`Message Bus CoordBus`**: Lock/Registry/Queue/Log primitives (Local+Redis backends) + cross-session projection.
- **`Web UI Console`**: zero-build SPA (Chat/KB/System, `go:embed` single binary).
- **`Agentic Memory`**: the agent manages Markdown memory files itself (file-based, unlike passive ReMe retrieval).
- **`Tracing` semantic attributes**: 5-hook span extraction (model/tool/iteration/usage) + otelSpan bridging.
- **`MCP` declarative config**: ServerSpec YAML + 6-server catalog + resilient connections.
- **`Langfuse`** integration + **`RBAC`** tests + **audit wiring** + **`slog`** structured logging.
<!-- END NEWS -->

## Quickstart

**Requirements:** Go 1.25 or newer

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

## Supported Models

| Provider | Package | Notes |
|----------|---------|-------|
| OpenAI Chat | `github.com/linkerlin/agentscope.go/model/openai` | GPT-4o / GPT-4o-mini / o1 / o3 etc. (Chat Completions API) |
| OpenAI Response | `github.com/linkerlin/agentscope.go/model/openai_response` | o3 / o4-mini etc. (Responses API, reasoning streaming) |
| Anthropic | `github.com/linkerlin/agentscope.go/model/anthropic` | Claude 3.5 Sonnet / Opus / Haiku, native HTTP + SSE |
| Gemini | `github.com/linkerlin/agentscope.go/model/gemini` | Gemini 1.5 Flash / Pro, native HTTP + SSE |
| DashScope (Alibaba) | `github.com/linkerlin/agentscope.go/model/dashscope` | Qwen series (OpenAI-compatible) |
| DeepSeek | `github.com/linkerlin/agentscope.go/model/deepseek` | DeepSeek-V3 / Coder / Reasoner (OpenAI-compatible) |
| Moonshot | `github.com/linkerlin/agentscope.go/model/moonshot` | Kimi series (OpenAI-compatible) |
| xAI | `github.com/linkerlin/agentscope.go/model/xai` | Grok series (OpenAI-compatible) |
| vLLM | `github.com/linkerlin/agentscope.go/model/vllm` | Self-hosted (OpenAI-compatible) |
| Ollama | `github.com/linkerlin/agentscope.go/model/ollama` | Local open models (OpenAI-compatible) |

Any OpenAI-compatible service can be reached via `BaseURL`.

### Per-provider quickstart scripts

[`scripts/model_examples/`](scripts/model_examples/) provides minimal runnable scripts for every provider, plus multiagent / stream / multimodal scenarios:

```bash
cd scripts/model_examples/openai_chat_call
export OPENAI_API_KEY=sk-...
go run .
```

Covered: OpenAI Chat / OpenAI Response / Anthropic / Gemini / DashScope / DeepSeek / Moonshot / xAI / Ollama / vLLM, plus multiagent / stream / multimodal.

### Cookbook

[`cookbook/`](cookbook/) provides reusable solution recipes:

- Long-document summarization (MapReduce)
- Multi-agent paper review (Reflection + Pipeline)
- RAG Q&A (Loader + Embedding + ReMe)
- Scheduled report agent (Schedule)
- Self-healing agent (GEP / Evolver)

## Core Packages

| Package | Description |
|---------|-------------|
| `message` | `Msg` type with multimodal content blocks (text, image, audio, video, tool calls/results, thinking) |
| `model` | `ChatModel` interface, streaming responses |
| `agent` | `Agent` base interface + `Base` unified lifecycle (hooks, streaming events, usage stats) |
| `agent/react` | ReAct agent implementation, embeds `agent.Base` |
| `memory` | `Memory` interface + 5 impls (InMemory/Window/ReMeInMemory/ReMeFile/ReMeVector) + 7 vector backends + Hybrid Search (BM25+Reranker) + Dream evolution + knowledge graph |
| `tool` | `Tool` interface + built-ins (file: Read/Write/Edit/Glob/Grep, shell, web, json, multimodal + Task/Schedule/Subagent) |
| `formatter` | Provider-agnostic request/response formatting (OpenAI / Anthropic / Gemini / DashScope / Ollama) |
| `pipeline` | Multi-agent orchestration: Pipeline (sequential) + Parallel (concurrent) |
| `msghub` | Broadcast multi-agent messaging (Hub) |
| `workflow` | Advanced orchestration: Condition / Loop / MapReduce |
| `reflection` | Self-reflection: Writer + Critic loop |
| `a2a` | A2A protocol: AgentCard, Task, SSE, Registry, ShardRouter, security (auth/rate-limit/WebSocket) |
| `gateway` | HTTP + SSE + WebSocket + AG-UI Gateway, multi-tenant auth + session persistence + tool offload + KB API + audit + RBAC |
| `service` | Multi-tenant service layer: Storage + Auth + Credential encryption + RBAC roles/permissions + audit log |
| `rag` | **Managed knowledge bases**: document/parser(Text/PDF/PPTX/Image)/chunker/blob/kb/index pipeline + RAGMiddleware + KB HTTP API |
| `messagebus` | **Distributed message bus**: LocalBus + RedisBus + CoordBus primitives (Lock/Registry/Queue/Log) + TeamBus + cross-session projection |
| `middleware` | Agent lifecycle middleware (onion model) + Budget/TTS/LongTermMemory/**RAG**/**AgenticMemory** |
| `logging` | **Structured logging**: stdlib slog wrapper + LOG_LEVEL/LOG_FORMAT env config + request-scoped FromContext |
| `schedule` | Cron scheduler |
| `async` | Async task pool |
| `loader` | Document loaders (TextLoader / DirLoader) |
| `observability` | OpenTelemetry + LangSmith + **Langfuse** + tracing semantic attributes + otelbridge |
| `channel` | **Multi-platform integration**: ChannelEvent/Channel interface/Gateway/Dispatcher/Routing + **Webhook/Discord/Feishu adapters** + Feishu agent tools |
| `hub` | **Hub marketplace**: browse + install MCP/Skill cards (FSHub + zip-slip protection) |
| `session` | Session management |
| `hook` | Hook system with human-in-the-loop support |
| `plan` | PlanNotebook for structured multi-step tasks |
| `embedding` | Standalone embedding package: OpenAI / Ollama / Gemini / DashScope / DashScope-multimodal + FileCache, usable directly from gateway / memory / RAG |
| `evolver` | GEP Gene/Capsule types + Evolver client + Run/Reflect/Solidify flow + Skill→Gene distillation |
| `embedding/onnx` | ONNX local inference: CLIP image embeddings + Whisper audio embeddings + model manager (HTTP proxy, zero CGO) |

## Channel Multi-Platform Integration

Connect agents to Webhook / Discord / Feishu through one pipeline:

```go
// 1. Build the channel subsystem
wh := channel.NewWebhookChannel("webhook-1")
router := channel.NewChatRouter(channel.RouteTable{
    ChannelID: "webhook-1",
    Bindings:  []channel.Binding{{ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"}},
})
gateway := channel.NewGateway(router, runner)   // runner adapts SessionManager
dispatcher := channel.NewDispatcher(gateway, channel.NewRegistry())

// 2. Platform adapter (pick one)
// dc := discord.New("discord-1", os.Getenv("DISCORD_TOKEN"))          // Discord
// fc := feishu.New("feishu-1", appID, appSecret)                      // Feishu (pure HTTP)

// 3. Start + serve the webhook endpoint
dispatcher.StartAll(ctx)
mux.Handle("/webhook", wh)   // or fc (Feishu event-subscription URL)
```

Feishu agent tools: `feishu_send_message` / `feishu_list_chats` (the agent can actively operate Feishu).
See [`docs/CHANNEL.md`](docs/CHANNEL.md).

## Hub Marketplace

Browse and install MCP/Skill cards from registries:

```go
h, _ := builtin.NewFSHub("./catalog", "demo", "Demo Hub", "")
mcps, next, _ := h.ListMCPCards(ctx, "", 0, 20)   // browse (cursor paging)
hub.InstallSkill(ctx, skillCard, "./skills")      // download + extract (zip-slip hardened)
mgr, _ := hub.InstallMCPs(ctx, mcps)              // resilient connect (missing binaries skipped)
```

Gateway wiring: `srv.WithHubs(h)` + `srv.RegisterHubRoutes()` (5 browse/install routes).
See [`docs/HUB.md`](docs/HUB.md).

## ONNX Production Inference (Local Multimodal)

Pure-Go image/audio preprocessing, connected to an ONNX Runtime service over HTTP — no Python needed, zero CGO:

```go
import "github.com/linkerlin/agentscope.go/embedding/onnx"

// Image preprocessing (CLIP) → NCHW [1,3,224,224]
preprocessor := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())
vec, _ := preprocessor.Preprocess(imageReader)

// Audio preprocessing (Whisper) → Mel spectrogram [1,80,3000]
audioProc := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())
mel, _ := audioProc.Preprocess(pcmSamples, 16000)

// CLIP image embedder (HTTP proxy)
clip := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
embedding, _ := clip.EmbedImage(vec)

// Cross-modal similarity (image-text alignment)
sim, _ := onnx.CrossModalSimilarity(imageEmbedding, textEmbedding)

// Model manager: auto download/cache/versioning
manager, _ := onnx.NewModelManager(onnx.DefaultModelManagerConfig())
manager.RegisterModel(onnx.PredefinedModels()[0]) // CLIP ViT-B/32
```

## A2A Enhanced (Auth + Rate Limit + WebSocket)

Full A2A protocol implementation with production-grade security and realtime delivery:

```go
import "github.com/linkerlin/agentscope.go/a2a"

// Secure server: auth + rate limit + CORS + logging
server := a2a.NewSecureServer(card, runner, store)
server.auth.AddAPIKey("sk-xxx", "production-client")
server.WithRateLimit(a2a.NewRateLimiter(100, 200)) // 100 req/s, burst 200

// WebSocket realtime task push
wsServer := a2a.NewWebSocketEnabledServer(card, runner, store)
// Clients subscribe to task status over WebSocket, receiving task_update events in real time
```

## Benchmarks

| Benchmark | Performance | Notes |
|-----------|-------------|-------|
| Embedding cache hit | 550 ns/op | in-memory LRU, lock-free reads |
| Cross-modal similarity | 741 ns/op | 512-dim cosine |
| Vector search (1000 nodes) | 229 μs/op | brute force + HNSW auto-switch |
| FTS full-text (1000 docs) | 97 μs/op | FTS5 trigram + CJK fallback |
| ReMe file memory add | 463 μs/op | incl. persistence write |
| ReAct memory injection | 132 μs/op | vector retrieval + format + inject |
| ONNX image preprocess | 3.5 ms/op | 1024×768 → 224×224 + normalize |
| ONNX audio preprocess | ~9.7 s/op | 30s audio → Mel spectrogram (optimizable) |

Run: `go test ./memory/... -run=^$ -bench=. -benchtime=1s`

## High-Level Production Bootstrap (Recommended)

`gateway.AppConfig` + `NewApp` provides a "create_app"-like one-shot experience with extensive auto-wiring:

```go
appCfg := gateway.AppConfig{
    Agent:                 myAgent,
    Storage:               service.NewMemoryStorage(),
    JWTAuth:               jwtAuth,
    WorkspaceBaseDir:      "./workspaces",
    AutoStandardTools:     true,           // auto-inject file+task+web+json+schedule tools for sessions
    AutoToolOffload:       true,
    DefaultPermissionMode: permission.ModeExplore,
    EmbeddingModel:        embedding.NewOpenAI(apiKey, "text-embedding-3-small"),
    EmbeddingCacheDir:     "./.embed_cache", // auto WithFileCache
}
srv := gateway.NewApp(appCfg)
srv.RegisterAppRoutes(jwtAuth)
srv.Start()   // auto-restores persisted schedules
defer srv.Close()
```

See `examples/full_service` and `examples/production`.

## Embedding Package

Standalone usage:

```go
import "github.com/linkerlin/agentscope.go/embedding"

emb := embedding.NewOpenAI(os.Getenv("OPENAI_API_KEY"), "text-embedding-3-small")
emb = embedding.WithFileCache(emb, ".cache/embeddings") // optional

vecs, _ := emb.Embed(ctx, []string{"hello world"})
```

Gemini / DashScope (incl. multimodal) supported.

## Using Tools

```go
import "github.com/linkerlin/agentscope.go/tool"

myTool := tool.NewFunctionTool(
    "weather",
    "Get the current weather for a city",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "city": map[string]any{"type": "string"},
        },
        "required": []string{"city"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        city := input["city"].(string)
        return fmt.Sprintf("%s: sunny, 22°C", city), nil
    },
)

agent, _ := react.Builder().
    Name("WeatherBot").
    Model(chatModel).
    Tools(myTool).
    Build()
```

## Memory Management

### Basic Memory

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

### ReMe Long-Term Memory (File + Vector)

```go
import "github.com/linkerlin/agentscope.go/memory"
import "github.com/linkerlin/agentscope.go/memory/handler"

// Create vector memory
v, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embedModel)

// Inject an orchestrator for automatic extraction and retrieval
orch := handler.NewMemoryOrchestrator(personalSum, proceduralSum, toolSum, memTool, profileTool, historyTool, dedup)
v.SetOrchestrator(orch)

// End-to-end automatic extraction of personal/task memories into the vector store
res, _ := v.SummarizeMemory(ctx, msgs, "alice", "coding_task", "")

// Unified retrieval
nodes, _ := v.RetrieveMemoryUnified(ctx, "Go best practices", "alice", "coding_task", "", memory.RetrieveOptions{TopK: 5})
```

### ReAct Memory-Injection Orchestration

```go
import "github.com/linkerlin/agentscope.go/memory"

// Step-level recorder
recorder := memory.NewReactStepRecorder(memory.NewInMemoryStepStore())

// Memory-injection orchestrator (4 strategies: recent/targeted/personal/hybrid)
orchestrator := memory.NewReactOrchestrator(recorder, store, memory.DefaultReactOrchestratorConfig())

// Inject relevant memories inside the ReAct loop
memNodes, sysMsg, _ := orchestrator.InjectMemory(ctx, query, history, "alice", "coding_task")

// Retrospective extraction: success paths / failure lessons / new knowledge
replay := memory.NewReactReplayExtractor(memory.DefaultReactReplayConfig())
result, _ := replay.Replay(ctx, steps)
```

## Hook System (Human-in-the-Loop)

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

## Plan Notebook

```go
import "github.com/linkerlin/agentscope.go/plan"

notebook := plan.NewPlanNotebook()
p := notebook.CreatePlan("Research task")
notebook.AddStep(p.ID, "Search for information")
notebook.AddStep(p.ID, "Summarize findings")

// Use as a tool inside an agent
agent, _ := react.Builder().
    Name("Planner").
    Model(chatModel).
    Tools(notebook.AsTool()).
    Build()
```

## Model Backend Examples

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

### DashScope (Alibaba)

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

## Multi-Agent Orchestration

### Sequential (Pipeline)

```go
import "github.com/linkerlin/agentscope.go/pipeline"

pipe := pipeline.New("ResearchPipe", plannerAgent, writerAgent)
resp, _ := pipe.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent("Go concurrency patterns").Build())
```

### Broadcast (MsgHub)

```go
import "github.com/linkerlin/agentscope.go/msghub"

hub := msghub.New()
hub.Register("coder", coderAgent)
hub.Register("reviewer", reviewerAgent)
results := hub.Broadcast(ctx, msg) // map[string]*message.Msg
```

### Parallel / Conditional / Loop (Workflow)

```go
import "github.com/linkerlin/agentscope.go/workflow"

// Parallel: two agents process simultaneously, results merged
par := workflow.NewParallel("DualCheck", nil, agentA, agentB)

// Conditional: branch on input content
cond := workflow.NewCondition("Router",
    func(m *message.Msg) bool { return strings.Contains(m.GetTextContent(), "urgent") },
    urgentAgent, normalAgent)

// Loop: iterate until a quality condition is met
loop := workflow.NewLoop("Refiner", editorAgent,
    func(m *message.Msg) bool { return !strings.Contains(m.GetTextContent(), "FINAL") },
    5)
```

## Realtime Chat Gateway

```go
import "github.com/linkerlin/agentscope.go/gateway"

srv := gateway.NewServer(agent)
http.ListenAndServe(":8080", srv)
```

- `POST /chat` — non-streaming chat, body `{"text":"..."}`, returns JSON.
- `POST /chat/stream` — SSE streaming chat, consume increments with `EventSource`.
- `GET /chat/ws` — WebSocket streaming chat, bidirectional realtime.

## MapReduce Workflow

```go
import "github.com/linkerlin/agentscope.go/workflow"

mr := workflow.NewMapReduce(
    "DocSummary",
    func(m *message.Msg) []string { return splitIntoParagraphs(m.GetTextContent()) },
    summarizerAgent,  // mapper
    synthesizerAgent, // reducer
    4, // parallelism
)
```

Input is split into chunks; each chunk is processed by the `mapper` in parallel; the `reducer` aggregates into a single result.

## Self-Reflection Mode

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

## Examples

- [`examples/a2a`](examples/a2a/main.go) — A2A protocol basics: AgentCard, Task, SSE
- [`examples/a2a_redis_registry`](examples/a2a_redis_registry/main.go) — A2A Redis distributed registry
- [`examples/embedding`](examples/embedding/main.go) — Standalone Embedding (OpenAI/Ollama + FileCache)
- [`examples/schedule`](examples/schedule/main.go) — Cron scheduler
- [`examples/rag`](examples/rag/main.go) — Full RAG Q&A pipeline (Loader + Embedding + ReMe)
- [`examples/rag_kb`](examples/rag_kb/main.go) — **Managed KB pipeline** (upload→parse→chunk→index→search, runs offline)
- [`examples/agentic_memory`](examples/agentic_memory/main.go) — **Agentic Memory** (agent-managed Markdown memory)
- [`examples/mcp_servers`](examples/mcp_servers/main.go) — **Declarative MCP config** (YAML + resilient multi-server connect)
- [`examples/web_ui`](examples/web_ui/main.go) — **Web UI Console** (Chat/KB/System, zero-build SPA)
- [`examples/channel_webhook`](examples/channel_webhook/main.go) — **Channel multi-platform** (webhook receive/send + routing + reply)
- [`examples/channel_discord`](examples/channel_discord/main.go) — **Discord bot** (WebSocket Gateway + REST replies)
- [`examples/channel_feishu`](examples/channel_feishu/main.go) — **Feishu bot** (event webhook + send, pure HTTP)
- [`examples/hub_demo`](examples/hub_demo/main.go) — **Hub marketplace** (browse MCP/Skill cards + install)
- [`examples/plugin_demo`](examples/plugin_demo/main.go) — **Plugin system** (3-phase lifecycle + YAML + tool registration)
- [`examples/observability`](examples/observability/main.go) — OpenTelemetry + LangSmith tracing
- [`examples/state`](examples/state/main.go) — AgentState persistence (JSONFile/Redis)
- [`examples/a2a_secure`](examples/a2a_secure/main.go) — A2A auth + rate limit + WebSocket
- [`examples/memory_benchmark`](examples/memory_benchmark/main.go) — Memory benchmarks
- [`examples/onnx`](examples/onnx/main.go) — ONNX image/audio preprocessing + embeddings
- [`examples/react_orchestrator`](examples/react_orchestrator/main.go) — ReAct memory-injection orchestration
- [`examples/cross_modal`](examples/cross_modal/main.go) — Cross-modal retrieval (text→image/audio)
- [`examples/multimodal`](examples/multimodal/main.go) — Multimodal agent (image/audio input)
- [`examples/multimodal_router`](examples/multimodal_router/main.go) — Automatic multimodal routing
- [`examples/middleware`](examples/middleware/main.go) — Agent lifecycle middleware chain
- [`examples/interrupt`](examples/interrupt/main.go) — Interruption and suspend/resume
- [`examples/trace`](examples/trace/main.go) — Event tracing + hook system
- [`examples/hello`](examples/hello/main.go) — Agent basics
- [`examples/tools`](examples/tools/main.go) — Agent with calculation tools
- [`examples/v2_event_stream`](examples/v2_event_stream/main.go) — V2 event-stream full lifecycle
- [`examples/anthropic`](examples/anthropic/main.go) — Agent with Claude backend
- [`examples/gemini`](examples/gemini/main.go) — Agent with Gemini backend
- [`examples/pipeline`](examples/pipeline/main.go) — Sequential multi-agent orchestration
- [`examples/msghub`](examples/msghub/main.go) — Broadcast multi-agent messaging
- [`examples/workflow`](examples/workflow/main.go) — Parallel + conditional + loop workflows
- [`examples/gateway`](examples/gateway/main.go) — HTTP + SSE realtime gateway
- [`examples/reflection`](examples/reflection/main.go) — Writer + Critic self-reflection
- [`examples/mapreduce`](examples/mapreduce/main.go) — MapReduce long-doc summarization
- [`examples/reme/file`](examples/reme/file/main.go) — ReMe file memory (ReMeLight)
- [`examples/reme/vector`](examples/reme/vector/main.go) — ReMe vector memory retrieval
- [`examples/reme/orchestrator`](examples/reme/orchestrator/main.go) — ReMe Orchestrator end-to-end
- [`examples/voice`](examples/voice/main.go) — STT → Chat → TTS voice pipeline
- [`examples/multi_tenant_workspace`](examples/multi_tenant_workspace/main.go) — Multi-tenant auth + workspace + permission engine
- [`examples/production`](examples/production/main.go) — Full-featured production service (Auth + tools + permissions + Gateway)
- [`examples/full_service`](examples/full_service/main.go) — Minimal auto-assembled production service (recommended)
- [`examples/studio`](examples/studio/main.go) — Pure-Go lightweight Studio (HTMX) — Auth/Agents/Credentials/Schedules/Chat + live SSE
- [`examples/evolver`](examples/evolver/main.go) — GEP self-evolution demo (Gene/Capsule, Run/Reflect/Solidify, distillation)
- [`examples/langsmith`](examples/langsmith/main.go) — Forward agent events to LangSmith

## Observability

### Tracing Middleware

Use `TracingMiddlewareAdapter` to inject tracing spans across the agent lifecycle (on_reply, on_reasoning, on_acting, on_model_call, on_system_prompt).

```go
import (
    "github.com/linkerlin/agentscope.go/observability"
    "github.com/linkerlin/agentscope.go/agent/react"
)

tracer := observability.NewOTelTracer(...) // or a LangSmith tracer, etc.
tracingMW := &observability.TracingMiddlewareAdapter{
    Tracer: tracer,
    Name:   "my-agent",
}

agent, _ := react.Builder().
    Name("TracedAgent").
    Model(chatModel).
    Middlewares(tracingMW).  // usable directly in the middleware chain
    Build()
```

Or wrap with `TracedAgent`:

```go
traced := observability.NewTracedAgent("my-agent", baseAgent).WithTracer(tracer)
```

`RecordingTracer` is available for debugging (see examples/full_service).

### LangSmith Tracing

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

All events emitted during a run (`ReplyStart`, `TextBlockDelta`, `ToolCallStart`, `ToolCallEnd`, `ReplyEnd`, …) are forwarded to LangSmith, producing full call-chain traces.

### OpenTelemetry

```go
import "github.com/linkerlin/agentscope.go/observability"

tp, _ := observability.InitTracerProvider("agent-service")
defer tp.Shutdown(context.Background())
```

The Gateway integrates an OTel HTTP middleware automatically — every request is traced. The toolkit layer also has a tracing middleware.

## GEP Self-Evolution & Evolver Alignment

agentscope.go aligns with the core strengths of [Evolver](https://github.com/EvoMap/evolver) — **GEP (Gene Evolution Protocol) based self-evolution**.

### Why align with Evolver?
Compact **Genes (strategy genes with signals_match + strategy + constraints + validation)** are better evolution assets than loose ad-hoc prompts/skill documents. They provide:
- Auditable, reusable, solidifiable evolution loops (run → reflect → solidify)
- Capsules (success snapshots with blast_radius, execution_trace, outcome)
- Typed memory (remember/recall gene/capsule/event, narrativeMemory / memoryGraph)
- Structured meetings (meeting_start / proceed / human_input / finalize)
- ATP task marketplace (claim / complete + hub reuse)
- Skill ↔ Gene distillation (skill2gep / distiller / publisher)
- Safety & rollback (safety_status, policy, gitOps rollback)

Since agentscope.go already has ReMe (world-class memory), a2a (leading protocol), gateway (perfect MCP bridging), and event+tracing infrastructure, we **bridge lightly** rather than reinvent:
- Native Go types + high-level flow APIs
- Your agents call all evolver tools through the existing MCP gateway
- ReMe + MemoryTypeGene/Capsule naturally carry evolution assets
- Skill distillation + recording-style visibility

### Quick start (Mock first, MCP in production)

```go
import "github.com/linkerlin/agentscope.go/evolver"

flow := evolver.NewGEPFlow(evolver.NewMockEvolver()) // production: real backed client
runCfg := evolver.RunConfig{Context: "recurring gateway timeout on large payload", Strategy: "repair-only"}
runRes, solRes, _ := flow.RunAndSolidify(ctx, runCfg, false /* true = dry run */)

fmt.Println("Selected gene:", runRes.SelectedGene.ID)
fmt.Println("Solidified capsule:", solRes.CapsuleID)

// Distill a Skill into a Gene
sk := &skill.AgentSkill{Name: "timeout_recovery", Description: "...", SkillContent: "..."}
gene := sk.DistillToGene(evolver.CategoryRepair)
flow.Client.UpsertGene(ctx, gene)

// Evolution memory recall (narrative style)
_ = flow.Client.Remember(ctx, evolver.RememberRequest{Text: "...", Type: "capsule", Category: evolver.CategoryRepair})
hits, _ := flow.Client.Recall(ctx, evolver.RecallRequest{Query: "timeout", Category: "capsule"})
```

**Production integration**: with `gateway.AppConfig{EvolverEnabled: true}`, session agents can call `evolver__evolver_run` / `evolver_solidify` etc. through MCP tools (see gateway/session_mcp_gateway). Combined with ReMe-persisted gene/capsule assets, you get "auto-trigger GEP fix on error + solidify + audit".

See also:
- `evolver/` package (types, client, gep flow, tests)
- `examples/evolver/main.go` (complete runnable demo)
- `DEV_PLAN_CATCHUP.md` Phase 6 section
- Evolver paper: arXiv:2604.15097

## Deployment & Migration

- Production deployment guide: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- Migrating from Python AgentScope or older versions: [MIGRATION.md](MIGRATION.md)
- Release process: [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md)

## Contributing & Community

We welcome all forms of contribution!

- Contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Security vulnerability reporting: [SECURITY.md](SECURITY.md)
- Current tasks & roadmap: [TODO.md](TODO.md)

If you run into issues, check [docs/](docs/) and [examples/](examples/) first, then file an Issue.

## License

Apache License 2.0 — see the [LICENSE](LICENSE) file.
