# 核心概念

AgentScope.Go V2 已完成从"消息为中心"到"事件为中心"的架构范式转移。理解以下四个核心概念是掌握本框架的关键。

---

## 1. 事件系统（Event System）

### 从 Msg 到 AgentEvent

传统 Agent 框架以 `Msg`（消息）为核心通信原语：

```
用户 Msg → Agent → 助手 Msg
```

AgentScope.Go V2 以 `AgentEvent`（事件）为核心：

```
ReplyStartEvent
├── TextBlockStartEvent → TextBlockDeltaEvent* → TextBlockEndEvent
├── ThinkingBlockStartEvent → ThinkingBlockDeltaEvent* → ThinkingBlockEndEvent
├── ToolCallStartEvent → ToolCallDeltaEvent* → ToolCallEndEvent
├── RequireUserConfirmEvent  ← HITL 挂起点
├── RequireExternalExecutionEvent
├── ToolResultStartEvent → ToolResultTextDeltaEvent* → ToolResultEndEvent
└── ReplyEndEvent
```

### 为什么用事件？

- **实时性**：模型每输出一个 token，客户端立即收到 `TextBlockDeltaEvent`
- **可观测性**：UI、A2A、日志全部建立在同一事件流之上
- **HITL**：`RequireUserConfirmEvent` 使 Agent 在 `SUBMITTED` 与 `FINISHED` 之间显式挂起

### Go 中的事件流

```go
ch, _ := agent.ReplyStream(ctx, msg)
for ev := range ch {
    switch e := ev.(type) {
    case *event.TextBlockDeltaEvent:
        fmt.Print(e.Delta)
    case *event.RequireUserConfirmEvent:
        // 弹出确认面板
        fmt.Printf("确认执行以下工具？%v\n", e.ToolCalls)
    }
}
```

---

## 2. AgentState 状态机

### 可序列化的运行时快照

`AgentState` 是 Agent 在某一时刻的完整运行时快照：

```go
type AgentState struct {
    ReplyID     string        // 当前 reply 的唯一标识
    CurIter     int           // ReAct 当前迭代轮次
    Messages    []*message.Msg // 完整对话历史
    ToolContext ToolContext    // 工具上下文（EquippedGroups、PendingCalls、Results）
    PermissionContext PermissionContext // 权限上下文
    SuspendedAt    *time.Time // 挂起时间（如有）
    WaitConfirmID  string     // 等待确认的 ID（如有）
}
```

### 挂起-恢复协议

```
Agent 运行中
    │
    ▼
遇到权限规则 = ASK
    │
    ▼
发出 RequireUserConfirmEvent
AgentState.Save() → Store（Redis / 文件）
    │
    ▼
外部系统注入 UserConfirmResultEvent
AgentState.Load() ← Store
    │
    ▼
Agent 继续执行
```

### 存储后端

| 后端 | 适用场景 |
|------|----------|
| `JSONFileStore` | 单机开发测试 |
| `RedisStore` | 生产多副本共享 |

---

## 3. Workspace 沙箱

Workspace 是工具执行的隔离环境：

```go
// LocalWorkspace：本地文件系统
ws, _ := workspace.NewLocalWorkspace("/tmp/agent_workspace")

// DockerWorkspace：容器隔离
ws, _ := workspace.NewDockerWorkspace("my-image")

// 绑定到 Agent
agent.WithWorkspace(ws)
```

工具（如 `ShellCommandTool`、`FileTool`）不再直接操作 `os.ReadFile`，而是通过 `agent.workspace.ReadFile`，实现执行环境的可插拔。

---

## 4. 权限引擎（Permission Engine）

权限引擎在工具执行前进行规则校验：

```go
engine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
    {Name: "allow_ls", Target: "execute_shell_command", Pattern: "ls*", Decision: permission.DecisionAllow},
    {Name: "deny_rm", Target: "execute_shell_command", Pattern: "rm*", Decision: permission.DecisionDeny},
})
```

四种决策模式：

| 决策 | 行为 |
|------|------|
| `ALLOW` | 自动允许 |
| `DENY` | 自动拒绝 |
| `ASK` | 触发 `RequireUserConfirmEvent`，等待用户确认 |
| `PASSTHROUGH` | 透传给下一个规则 |

---

## 术语对照（Go vs Python）

| Python v2 | Go V2 | 说明 |
|-----------|-------|------|
| `reply_stream()` | `ReplyStream()` | 真事件流 |
| `AgentState` | `agent.AgentState` | 可序列化状态 |
| `PermissionEngine` | `permission.Engine` | 权限引擎 |
| `Workspace` | `workspace.Workspace` | 执行环境抽象 |
| `Toolkit` | `toolkit.Toolkit` | 工具集 |
| `StorageBase` | `service.Storage` / `state.Store` | 持久化存储抽象 |
