# 多 Agent 编排

AgentScope.Go 提供丰富的多 Agent 编排原语，超越 Python 版的 Msg 列表模式。

## 编排模式

### 1. Pipeline（顺序执行）

```go
import "github.com/linkerlin/agentscope.go/pipeline"

pipe := pipeline.New("ResearchPipe", plannerAgent, writerAgent)
resp, _ := pipe.Call(ctx, message.NewMsg().
    Role(message.RoleUser).
    TextContent("Write a report on Go concurrency").
    Build())
```

### 2. MsgHub（广播调度）

```go
import "github.com/linkerlin/agentscope.go/msghub"

hub := msghub.New()
hub.Register("coder", coderAgent)
hub.Register("reviewer", reviewerAgent)

results := hub.Broadcast(ctx, msg) // map[string]*message.Msg
```

### 3. Parallel（并发执行）

```go
import "github.com/linkerlin/agentscope.go/workflow"

par := workflow.NewParallel("DualCheck", nil, agentA, agentB)
resp, _ := par.Call(ctx, msg)
```

### 4. MapReduce（分片汇总）

```go
mr := workflow.NewMapReduce(
    "DocSummary",
    func(m *message.Msg) []string {
        return splitIntoParagraphs(m.GetTextContent())
    },
    summarizerAgent, // mapper
    synthesizerAgent, // reducer
    4, // parallelism
)
```

### 5. Condition（条件分支）

```go
cond := workflow.NewCondition("Router",
    func(m *message.Msg) bool {
        return strings.Contains(m.GetTextContent(), "urgent")
    },
    urgentAgent, normalAgent)
```

### 6. Loop（迭代优化）

```go
loop := workflow.NewLoop("Refiner", editorAgent,
    func(m *message.Msg) bool {
        return !strings.Contains(m.GetTextContent(), "FINAL")
    },
    5) // max iterations
```

### 7. Reflection（自省迭代）

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
```

## 与 Python 版对比

| 编排模式 | Python 2.0 | Go 2.0.0 |
|---------|-----------|----------|
| 顺序执行 | ⚠️ Msg 列表 | ✅ Pipeline |
| 广播 | ⚠️ MsgHub 基础 | ✅ MsgHub |
| 并发 | ⚠️ 手动 | ✅ Parallel/Workflow |
| MapReduce | ❌ 无 | ✅ 完整实现 |
| 条件分支 | ❌ 无 | ✅ Condition |
| 循环迭代 | ❌ 无 | ✅ Loop |
| 自省反思 | ❌ 无 | ✅ Reflection |
| Subagent 递归 | ❌ 无 | ✅ SubagentTool |
