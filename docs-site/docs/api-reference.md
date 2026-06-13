# API 参考

> 本文档列出 AgentScope.Go 各核心包的公共接口，供开发者快速查阅。

---

## 目录

- [agent](#agent)
- [agent/react](#agentreact)
- [message](#message)
- [event](#event)
- [model](#model)
- [tool](#tool)
- [workspace](#workspace)
- [permission](#permission)
- [memory](#memory)
- [gateway](#gateway)
- [service](#service)

---

## agent

`github.com/linkerlin/agentscope.go/agent`

### Agent

Agent 的基础接口，所有 Agent 实现必须满足。

```go
type Agent interface {
    Name() string
    Call(ctx context.Context, msg *message.Msg) (*message.Msg, error)
}
```

### V2Agent

V2 事件驱动 Agent 接口，扩展了流式输出和状态管理能力。

```go
type V2Agent interface {
    Agent
    ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error)
    SaveState() (*state.AgentState, error)
    LoadState(st *state.AgentState) error
    InjectEvent(ctx context.Context, ev event.AgentEvent) error
}
```

### Base

Agent 基类，通过 struct embedding 复用通用生命周期。

```go
type Base struct {
    Name string
    // ...
}
```

---

## agent/react

`github.com/linkerlin/agentscope.go/agent/react`

### ReActAgent

基于 ReAct 范式的 V2 Agent 实现，支持事件流、工具调用和挂起恢复。

```go
agent, err := react.Builder().
    Name("assistant").
    Model(model).
    Memory(memory.NewInMemoryMemory()).
    Toolkit(tk).
    Build()
```

### Builder 方法

| 方法 | 说明 |
|------|------|
| `Name(string)` | Agent 名称 |
| `Model(model.ChatModel)` | 对话模型 |
| `Memory(memory.Memory)` | 记忆系统 |
| `Toolkit(*toolkit.Toolkit)` | 工具集 |
| `Workspace(workspace.Workspace)` | 执行环境 |
| `PermissionEngine(*permission.Engine)` | 权限引擎 |
| `SystemPrompt(string)` | 系统提示 |
| `Build() (*ReActAgent, error)` | 构建 |

---

## message

`github.com/linkerlin/agentscope.go/message`

### Msg

消息是 Agent 之间通信的基本单元，包含多个 ContentBlock。

```go
msg := message.NewMsg().
    Role(message.RoleUser).
    TextContent("Hello!").
    Build()
```

### ContentBlock 类型

| 类型 | 说明 | 创建方式 |
|------|------|----------|
| `TextBlock` | 文本块 | `message.NewTextBlock(text)` |
| `ImageBlock` | 图像块 | `message.NewImageBlock(url)` |
| `ToolUseBlock` | 工具调用块 | 模型自动生成 |
| `ToolResultBlock` | 工具结果块 | `tool.NewTextResponse(text)` |
| `ThinkingBlock` | 思考块 | 模型自动生成 |
| `HintBlock` | 提示块 | `message.NewHintBlock(text, kind)` |

### MsgRole

```go
const (
    RoleUser      MsgRole = "user"
    RoleAssistant MsgRole = "assistant"
    RoleSystem    MsgRole = "system"
)
```

---

## event

`github.com/linkerlin/agentscope.go/event`

### AgentEvent

所有事件的公共接口。

```go
type AgentEvent interface {
    EventType() string
    ReplyID() string
    Timestamp() time.Time
}
```

### 核心事件类型

| 事件 | 说明 |
|------|------|
| `ReplyStartEvent` | reply 开始 |
| `TextBlockStartEvent` / `TextBlockDeltaEvent` / `TextBlockEndEvent` | 文本块生命周期 |
| `ThinkingBlockStartEvent` / `ThinkingBlockDeltaEvent` / `ThinkingBlockEndEvent` | 思考块生命周期 |
| `ToolCallStartEvent` / `ToolCallDeltaEvent` / `ToolCallEndEvent` | 工具调用生命周期 |
| `ToolResultStartEvent` / `ToolResultTextDeltaEvent` / `ToolResultEndEvent` | 工具结果生命周期 |
| `RequireUserConfirmEvent` | 请求用户确认（HITL 挂起点） |
| `RequireExternalExecutionEvent` | 请求外部执行 |
| `ReplyEndEvent` | reply 结束 |
| `UserConfirmResultEvent` | 用户确认结果（外部注入恢复） |
| `ExternalExecutionResultEvent` | 外部执行结果（外部注入恢复） |

### Bus

事件总线，支持发布-订阅模式。

```go
bus := event.NewBus()
bus.Subscribe(func(ev event.AgentEvent) {
    // 处理事件
})
bus.Publish(ctx, ev)
```

---

## model

`github.com/linkerlin/agentscope.go/model`

### ChatModel

所有对话模型的公共接口。

```go
type ChatModel interface {
    ModelName() string
    Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error)
    ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error)
}
```

### 模型后端

| 后端 | 包路径 | Builder |
|------|--------|---------|
| OpenAI Chat | `model/openai` | `openai.Builder()` |
| OpenAI Response | `model/openai_response` | `openai_response.Builder()` |
| Anthropic | `model/anthropic` | `anthropic.Builder()` |
| Gemini | `model/gemini` | `gemini.Builder()` |
| DashScope | `model/dashscope` | `dashscope.Builder()` |
| Ollama | `model/ollama` | `ollama.Builder()` |
| DeepSeek | `model/deepseek` | `deepseek.Builder()` |
| Moonshot | `model/moonshot` | `moonshot.Builder()` |
| xAI | `model/xai` | `xai.Builder()` |
| vLLM | `model/vllm` | `vllm.Builder()` |

### Builder 通用方法

| 方法 | 说明 |
|------|------|
| `APIKey(string)` | API 密钥 |
| `ModelName(string)` | 模型名称 |
| `BaseURL(string)` | 自定义 Base URL |
| `Retry(maxAttempts int, backoff time.Duration)` | 重试策略 |
| `Build() (model.ChatModel, error)` | 构建 |

### ResponseFormat（结构化输出）

```go
rf := &model.ResponseFormat{
    Type:       model.ResponseFormatTypeJSONObject,
    JSONSchema: schema,
}
model.Chat(ctx, msgs, model.WithResponseFormat(rf))
```

---

## tool

`github.com/linkerlin/agentscope.go/tool`

### Tool

所有工具的公共接口。

```go
type Tool interface {
    Name() string
    Description() string
    Spec() model.ToolSpec
    Execute(ctx context.Context, input map[string]any) (*Response, error)
}
```

### Response

工具执行结果。

```go
resp := tool.NewTextResponse("result text")
resp := tool.NewErrorResponse(errors.New("failed"))
```

### 内置文件工具

| 工具 | 创建 | 说明 |
|------|------|------|
| `view_text_file` | `file.NewReadFileTool(baseDir)` | 查看文件内容（支持行范围） |
| `list_directory` | `file.NewListDirectoryTool(baseDir)` | 列出目录内容 |
| `write_text_file` | `file.NewWriteFileTool(baseDir)` | 写入/覆盖文件（支持行范围替换） |
| `insert_text_file` | `file.NewInsertTextFileTool(baseDir)` | 在指定行插入内容 |
| `edit_text_file` | `file.NewEditFileTool(baseDir)` | 精确字符串替换 |
| `glob` | `file.NewGlobTool(baseDir)` | 文件路径模式匹配 |
| `grep` | `file.NewGrepTool(baseDir)` | 文本正则搜索 |

### 其他工具

| 工具 | 创建 | 说明 |
|------|------|------|
| `shell_command` | `shell.NewShellCommandTool(...)` | 执行 shell 命令 |
| `subagent` | `subagent.NewSubagentTool(name, desc, agent)` | 递归调用子 Agent |
| `web_fetch` | `web.NewFetchTool(timeout)` | HTTP GET 抓取 URL 内容 |
| `json_parse` | `json.NewParseTool()` | 解析并格式化 JSON |
| `json_query` | `json.NewQueryTool()` | 按 dot-separated 路径查询 JSON |

---

## workspace

`github.com/linkerlin/agentscope.go/workspace`

### Workspace

执行环境抽象接口。

```go
type Workspace interface {
    ID() string
    Type() string
    ReadFile(ctx context.Context, path string) ([]byte, error)
    WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error
    ListDir(ctx context.Context, path string) ([]DirEntry, error)
    MkdirAll(ctx context.Context, path string, perm fs.FileMode) error
    Stat(ctx context.Context, path string) (FileInfo, error)
    Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error)
    Close() error
}
```

### 实现

| 实现 | 创建 | 说明 |
|------|------|------|
| `LocalWorkspace` | `workspace.NewLocalWorkspace(id, dir)` | 本地文件系统 |
| `DockerWorkspace` | `workspace.NewDockerWorkspace(id, image)` | Docker 容器 |
| `E2BWorkspace` | `workspace.CreateE2BWorkspace(...)` | E2B 云端沙箱（REST 生命周期） |

---

## permission

`github.com/linkerlin/agentscope.go/permission`

### Engine

权限规则引擎。

```go
engine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
    {Target: "tool_name", Pattern: "read_file", Decision: permission.DecisionAllow},
    {Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
})
```

### PermissionMode

| 模式 | 说明 |
|------|------|
| `ModeExplore` | 探索模式：读操作自动允许，写操作需确认 |
| `ModeStrict` | 严格模式：所有操作需匹配规则 |
| `ModeBypass` | 绕过模式：所有操作允许（仅测试用） |

### Decision

| 决策 | 行为 |
|------|------|
| `DecisionAllow` | 自动允许 |
| `DecisionDeny` | 自动拒绝 |
| `DecisionAsk` | 触发 HITL 挂起 |
| `DecisionPassthrough` | 透传给下一个规则 |

---

## memory

`github.com/linkerlin/agentscope.go/memory`

### Memory

记忆系统接口。

```go
type Memory interface {
    Add(ctx context.Context, msg *message.Msg) error
    Retrieve(ctx context.Context, query string, k int) ([]*message.Msg, error)
    GetHistory(ctx context.Context) ([]*message.Msg, error)
    Clear(ctx context.Context) error
}
```

### 实现

| 实现 | 创建 | 说明 |
|------|------|------|
| `InMemoryMemory` | `memory.NewInMemoryMemory()` | 内存历史 |
| `ReMeFileMemory` | `memory.NewReMeFileMemory(cfg)` | 文件长期记忆 |
| `ReMeVectorMemory` | `memory.NewReMeVectorMemory(cfg)` | 向量长期记忆 |
| `HybridSearchMemory` | `memory.NewHybridSearchMemory(...)` | 混合搜索 |

### ReMe 配置

```go
cfg := &memory.ReMeConfig{
    AgentName:      "bot",
    WorkingDir:     "./memory_data",
    MaxRecentTurns: 10,
    EnableFile:     true,
    EnableVector:   true,
}
```

---

## gateway

`github.com/linkerlin/agentscope.go/gateway`

### Server

HTTP/SSE/WebSocket 网关服务。

```go
srv := gateway.NewServer(agent)
srv.WithStorage(redisStorage) // 启用 Session 持久化
srv.RegisterV2Routes()
log.Fatal(http.ListenAndServe(":8080", srv))
```

### 核心端点

| 端点 | 方法 | Content-Type | 说明 |
|------|------|-------------|------|
| `/health` | GET | JSON | 健康检查（版本、存储、认证、活跃会话数） |
| `/v2/chat` | POST | SSE / JSON | Streamable HTTP：启动 Agent 运行 |
| `/v2/chat` | GET | SSE | Streamable HTTP：订阅已有运行 |
| `/v2/chat` | DELETE | — | Streamable HTTP：终止运行 |
| `/v2/chat/stream` | POST | SSE | SSE 事件流（legacy 兼容） |
| `/v2/chat/ws` | GET | WebSocket | WebSocket 双向流（支持挂起/恢复） |
| `/v2/resume` | POST | JSON | HTTP 恢复挂起的 Agent |
| `/api/v1/auth/register` | POST | JSON | 用户注册（返回 API Key） |
| `/api/v1/auth/login` | POST | JSON | 用户登录（返回 JWT） |
| `/api/v1/agents` | CRUD | JSON | Agent 管理 |
| `/api/v1/sessions` | CRUD | JSON | Session 管理 |
| `/api/v1/credentials` | CRUD | JSON | 凭证管理 |
| `/api/v1/models` | GET | JSON | 模型卡片列表 |
| `/api/v1/schedule` | CRUD | JSON | Cron 调度管理 |
| `/api/v1/background-tasks` | GET/DELETE | JSON | 后台任务管理 |

### AG-UI 协议

所有 SSE/WS 端点支持 `?protocol=agui` 查询参数或 `X-Protocol: agui` 请求头，将事件流自动转换为 AG-UI 协议格式。

### 认证

设置 `Authenticator` 后，所有 V2 和管理端点自动启用认证：

```go
// API Key 认证
srv.WithAuthenticator(apiKeyAuth)
// 请求头：X-API-Key: <key>

// JWT Bearer 认证
srv.WithAuthenticator(jwtAuth)
// 请求头：Authorization: Bearer <token>
```

### SessionStateManager

跨请求挂起-恢复管理器。

```go
mgr := gateway.NewSessionStateManager(storage)
mgr.SaveSnapshot(ctx, sessionID, v2Agent)
mgr.Resume(ctx, sessionID, v2Agent, confirmEvent)
```

---

## service

`github.com/linkerlin/agentscope.go/service`

### Storage

持久化存储接口。

```go
type Storage interface {
    SaveSnapshot(ctx context.Context, snap *AgentSnapshot) error
    GetSnapshot(ctx context.Context, sessionID string) (*AgentSnapshot, error)
    DeleteSnapshot(ctx context.Context, sessionID string) error
    // ... 管理端点方法
}
```

### 实现

| 实现 | 创建 | 说明 |
|------|------|------|
| `MemoryStorage` | `service.NewMemoryStorage()` | 内存存储（开发测试） |
| `RedisStorage` | `service.NewRedisStorage(client)` | Redis 存储（生产） |

### Cipher（凭证加密）

```go
cipher, err := service.NewCipherFromEnv() // 从 AGENTSCOPE_ENCRYPTION_KEY 读取
ciphertext, err := cipher.Encrypt(plaintext)
plaintext, err := cipher.Decrypt(ciphertext)
```

---

## formatter

`github.com/linkerlin/agentscope.go/formatter`

消息格式化层，将 Agent 级 `Msg` 和 `ToolSpec` 转换为各 LLM API 的原生请求格式。与模型实现解耦，便于添加新后端。

### Formatter

所有格式化器的通用接口：

```go
type Formatter interface {
    FormatMessages(msgs []*message.Msg) (any, error)
    FormatTools(specs []model.ToolSpec) (any, error)
    FormatToolChoice(tc *model.ToolChoice) (any, error)
    ParseResponse(resp any) (*message.Msg, error)
}
```

### 实现

| 格式化器 | 创建 | 说明 |
|----------|------|------|
| `OpenAIFormatter` | `formatter.NewOpenAIFormatter()` | ChatGPT / DeepSeek / Moonshot / xAI / vLLM |
| `AnthropicFormatter` | `formatter.NewAnthropicFormatter()` | Claude Messages API |
| `GeminiFormatter` | `formatter.NewGeminiFormatter()` | Gemini REST API |
| `DashScopeFormatter` | `formatter.NewDashScopeFormatter()` | 通义千问（OpenAI 兼容，别名） |
| `OllamaFormatter` | `formatter.NewOllamaFormatter()` | Ollama（OpenAI 兼容，别名） |

### ThinkingFormatter（可选扩展）

支持 Thinking/Reasoning 块的格式化器：

```go
type ThinkingFormatter interface {
    Formatter
    WrapThinkingBlock(content string) string
}
```

### MultiAgent Formatter

将多 Agent 对话历史压缩为单条 user message 的格式化器：

```go
// formatter/multi_agent.go
formatter.NewMultiAgentOpenAIFormatter()
formatter.NewMultiAgentAnthropicFormatter()
formatter.NewMultiAgentGeminiFormatter()
```

### 性能特征（i9-13900HX）

| 操作 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| OpenAI FormatMessages (1 msg) | 224 | 352 | 3 |
| OpenAI FormatMessages (50 msg) | 15,261 | 17,312 | 101 |
| Anthropic FormatMessages (10 msg) | 9,752 | 7,585 | 116 |
| Gemini FormatContents (10 msg) | 4,272 | 7,552 | 82 |
| ExtractThinkingBlocks (无标签) | 42 | 0 | 0 |
| ExtractThinkingBlocks (含标签) | 4,292 | 650 | 10 |

---

## gateway

`github.com/linkerlin/agentscope.go/gateway`

### AppConfig

生产级服务一键装配配置：

```go
type AppConfig struct {
    Agent                 agent.Agent
    Storage               service.Storage
    JWTAuth               *JWTAuthenticator
    WorkspaceBaseDir      string
    AutoStandardTools     bool
    AutoToolOffload       bool
    DefaultPermissionMode permission.Mode
    EmbeddingModel        embedding.EmbeddingModel
    EmbeddingCacheDir     string
    EvolverEnabled        bool
    EnableDemoRegister    bool
}
```

### NewApp

```go
srv := gateway.NewApp(appCfg)
srv.RegisterAppRoutes(jwtAuth)
srv.Start()  // 自动恢复 persisted schedules
defer srv.Close()
```

### 主要路由

| 端点 | 说明 |
|------|------|
| `POST /api/v1/auth/register` | 用户注册 |
| `POST /api/v1/auth/login` | 用户登录 |
| `GET /api/v1/me` | 当前用户信息 |
| `GET/POST/DELETE /api/v1/agents` | Agent CRUD |
| `GET/POST/DELETE /api/v1/sessions` | Session CRUD |
| `GET/POST/DELETE /api/v1/credentials` | Credential CRUD |
| `GET/POST/DELETE /api/v1/schedules` | Schedule CRUD |
| `GET /api/v1/models` | ModelCard 列表 |
| `POST /v2/chat/stream` | SSE 事件流对话 |
| `GET /v2/chat/ws` | WebSocket 对话 |
| `POST /v2/resume` | 恢复挂起 Agent |
| `GET /health` | 健康检查 |
| `GET /metrics/events` | 事件流指标 |

---

## service

`github.com/linkerlin/agentscope.go/service`

### Storage

```go
type Storage interface {
    CreateUser(ctx context.Context, user *User) error
    GetUserByUsername(ctx context.Context, username string) (*User, error)
    // Agent / Session / Credential / Schedule / Message CRUD
}
```

### 实现

- `service.NewMemoryStorage()` — 内存存储，适合开发测试
- `service.NewRedisStorage(rdb)` — Redis 存储，适合生产

### Cipher

AES-256-GCM 凭证加密：

```go
cipher, _ := service.NewCipherFromEnv() // AGENTSCOPE_CIPHER_KEY
encrypted, _ := cipher.Encrypt(plainText)
plain, _ := cipher.Decrypt(encrypted)
```

---

## a2a

`github.com/linkerlin/agentscope.go/a2a`

### AgentCard

```go
type AgentCard struct {
    Name         string
    Description  string
    URL          string
    Version      string
    Capabilities Capabilities
}
```

### Server

```go
server := a2a.NewServer(card, adapter)
http.ListenAndServe(":9000", server)
```

### Client

```go
client := a2a.NewClient("http://localhost:9000")
task, _ := client.SendTask(ctx, task)
ch, _ := client.SendTaskSubscribe(ctx, task)
```

### Registry

```go
registry := a2a.NewRegistry(30 * time.Second)
registry.Register(card)
healthy := registry.ListHealthy()
```

---

## evolver

`github.com/linkerlin/agentscope.go/evolver`

### Gene

```go
type Gene struct {
    ID          string
    Category    string // repair / optimize / innovate / explore
    Signals     []string
    Strategy    string
    Constraints []string
    Validation  []string
    // ...
}
```

### GEPFlow

```go
flow := evolver.NewGEPFlow(client)
runRes, solRes, err := flow.RunAndSolidify(ctx, evolver.RunConfig{
    Context:  "recurring timeout",
    Strategy: "repair-only",
}, false)
```

### Skill2GEP

```go
gene := skill.DistillToGene(evolver.CategoryRepair)
```

---

## observability

`github.com/linkerlin/agentscope.go/observability`

### OpenTelemetry

```go
tp, _ := observability.InitTracerProvider("agent-service")
defer tp.Shutdown(ctx)
```

### LangSmith

```go
client := observability.NewLangSmithClient(apiKey)
observer := observability.NewLangSmithObserver(client, project, session)
go observer.Observe(ctx, bus)
```

### TracingMiddlewareAdapter

```go
tracingMW := &observability.TracingMiddlewareAdapter{
    Tracer: tracer,
    Name:   "my-agent",
}
agent, _ := react.Builder().Middlewares(tracingMW).Build()
```

---

## 其他重要包

| 包 | 说明 |
|----|------|
| `formatter` | 模型提示格式化（OpenAI / Anthropic / Gemini / DashScope / Ollama） |
| `toolkit` | 工具集管理 + MCP Client/Server + 洋葱中间件 |
| `schedule` | Cron 调度器 |
| `async` | 异步任务执行池 |
| `pipeline` | 多 Agent 编排（Parallel 并发执行） |
| `workflow` | 工作流（Condition / Loop / MapReduce） |
| `msghub` | 消息中心（多 Agent 广播） |
| `rag` | RAG 管道（文档加载 + 向量检索） |
| `loader` | 文档加载器（TextLoader / DirLoader） |
| `state` | AgentState 存储（JSONFileStore / RedisStore） |
| `session` | Session 管理 |
| `plan` | 计划执行器 |
| `reflection` | 反射 Agent |
| `skill` | Skill 加载与管理 |
