# 教程：从入门到生产

> 本文档通过 5 个循序渐进的示例，带你掌握 AgentScope.Go 的核心用法。

---

## 目录

1. [Hello Agent](#1-hello-agent)
2. [使用工具](#2-使用工具)
3. [事件流与 HITL](#3-事件流与-hith)
4. [多 Agent 编排](#4-多-agent-编排)
5. [生产部署](#5-生产部署)

---

## 1. Hello Agent

### 最小可用示例

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/memory"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    // 1. 创建模型
    model, err := openai.Builder().
        APIKey("YOUR_OPENAI_API_KEY").
        ModelName("gpt-4o-mini").
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 2. 创建 Agent
    agent, err := react.Builder().
        Name("assistant").
        Model(model).
        Memory(memory.NewInMemoryMemory()).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 3. 对话
    ctx := context.Background()
    msg := message.NewMsg().Role(message.RoleUser).TextContent("你好！").Build()
    resp, err := agent.Call(ctx, msg)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.GetTextContent())
}
```

### 使用其他模型后端

```go
// DeepSeek
import "github.com/linkerlin/agentscope.go/model/deepseek"
model, _ := deepseek.Builder().APIKey("key").Build()

// Moonshot (Kimi)
import "github.com/linkerlin/agentscope.go/model/moonshot"
model, _ := moonshot.Builder().APIKey("key").Build()

// vLLM（私有化部署）
import "github.com/linkerlin/agentscope.go/model/vllm"
model, _ := vllm.Builder().APIKey("key").BaseURL("http://localhost:8000/v1").Build()
```

---

## 2. 使用工具

### 给 Agent 配备工具

```go
import (
    "github.com/linkerlin/agentscope.go/tool/file"
    "github.com/linkerlin/agentscope.go/tool/shell"
    "github.com/linkerlin/agentscope.go/toolkit"
)

ws := workspace.NewLocalWorkspace("default", "./workspace")

// 一键注册所有文件工具
fileTools := file.RegisterAll("./workspace", false) // false = 含写入工具

// Shell 工具
shellTool := shell.NewShellCommandTool("./workspace", []string{"ls", "cat", "pwd"}, nil)

tk := toolkit.NewToolkit()
for _, t := range fileTools {
    tk.Register(t)
}
tk.Register(shellTool)

agent, _ := react.Builder().
    Name("coder").
    Model(model).
    Toolkit(tk).
    Build()
```

导入必要包即可一键配备所有常用工具。`RegisterAll` 的 `readOnly=true` 模式下只注册只读工具。`false` 模式包含写入和编辑工具。

### 工具使用示例

Agent 现在可以：
- 用 `view_text_file` 查看文件内容
- 用 `write_text_file` 创建或覆盖文件
- 用 `edit_text_file` 精确替换文本
- 用 `glob` 查找文件
- 用 `grep` 搜索代码
- 用 `shell_command` 执行命令

### 其他内置工具

```go
import (
    "time"
    "github.com/linkerlin/agentscope.go/tool/web"
    "github.com/linkerlin/agentscope.go/tool/json"
    "github.com/linkerlin/agentscope.go/tool/schedule"
    "github.com/linkerlin/agentscope.go/schedule"
)

// Web Fetch — 抓取 URL 内容
webTool := web.NewFetchTool(30 * time.Second)

// JSON — 解析和查询
parseTool := json.NewParseTool()
queryTool := json.NewQueryTool()

// Schedule — Agent 自管理 Cron 调度（独立使用，无需 Gateway）
sched := schedule.NewScheduler(handler)
sched.Start()
mgr := scheduletool.NewStandardManager(sched)
scheduleTools := scheduletool.RegisterTools(mgr)
```

---

## 3. 事件流与 HITL

### 消费事件流

```go
ch, err := agent.ReplyStream(ctx, msg)
if err != nil {
    log.Fatal(err)
}

for ev := range ch {
    switch e := ev.(type) {
    case *event.TextBlockDeltaEvent:
        fmt.Print(e.Delta) // 实时输出 token
    case *event.ToolCallStartEvent:
        fmt.Printf("\n[Tool] %s\n", e.Name)
    case *event.RequireUserConfirmEvent:
        fmt.Printf("\n[Confirm] 执行 %s? (y/n)\n", e.ToolCalls[0].Name)
        // 等待用户输入后注入确认结果
    case *event.ReplyEndEvent:
        fmt.Println("\n[Done]")
    }
}
```

### HITL（Human-in-the-Loop）

当权限规则为 `ASK` 时，Agent 会挂起并发出 `RequireUserConfirmEvent`：

```go
engine := permission.NewEngine(permission.ModeExplore, []permission.Rule{
    {Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
    {Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
})

agent, _ := react.Builder().
    Model(model).
    PermissionEngine(engine).
    Build()
```

恢复挂起的 Agent：

```go
// 在 Gateway 层自动处理
// 或手动注入确认事件
agent.InjectEvent(ctx, event.NewUserConfirmResult(replyID, confirmID, []event.ConfirmDecision{
    {ToolCallID: "tc_xxx", Allowed: true},
}))
```

---

## 4. 多 Agent 编排

### Pipeline 顺序执行

```go
import "github.com/linkerlin/agentscope.go/pipeline"

p := pipeline.NewPipeline("research", planner, researcher, writer)
resp, _ := p.Call(ctx, msg)
```

### Parallel 并发执行

```go
import "github.com/linkerlin/agentscope.go/pipeline"

parallel := pipeline.NewParallel("review", reviewerA, reviewerB, reviewerC).
    WithAggregator(func(msgs []*message.Msg) *message.Msg {
        // 合并三个评审员的反馈
        var sb strings.Builder
        for _, m := range msgs {
            sb.WriteString(m.GetTextContent())
            sb.WriteString("\n---\n")
        }
        return message.NewMsg().Role(message.RoleAssistant).TextContent(sb.String()).Build()
    })

resp, _ := parallel.Call(ctx, msg)
```

### Workflow 工作流

```go
import "github.com/linkerlin/agentscope.go/workflow"

// 条件分支
cond := workflow.NewCondition("check", decider,
    workflow.Case("yes", positiveBranch),
    workflow.Case("no", negativeBranch),
)

// MapReduce
mr := workflow.NewMapReduce("process", splitter, mapper, reducer)
```

### MsgHub 消息中心

```go
import "github.com/linkerlin/agentscope.go/msghub"

hub := msghub.NewHub()
hub.AddAgent(agentA)
hub.AddAgent(agentB)
hub.Broadcast(ctx, msg)
```

---

## 5. 生产部署

### 单机部署（零依赖）

```go
package main

import (
    "log"
    "net/http"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/gateway"
    "github.com/linkerlin/agentscope.go/memory"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    model, _ := openai.Builder().APIKey("key").Build()
    agent, _ := react.Builder().
        Name("bot").
        Model(model).
        Memory(memory.NewInMemoryMemory()).
        Build()

    srv := gateway.NewServer(agent)
    srv.RegisterV2Routes()
    log.Println("Listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", srv))
}
```

### 生产部署（Redis + 多租户）

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/linkerlin/agentscope.go/gateway"
    "github.com/linkerlin/agentscope.go/service"
)

redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
storage := service.NewRedisStorage(redisClient)

srv := gateway.NewServer(agent)
srv.WithStorage(storage) // 启用 Session 持久化 + AgentState 快照
srv.RegisterV2Routes()
```

### 调用 SSE 事件流

```bash
curl -X POST http://localhost:8080/v2/chat/stream \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"text": "Hello", "session_id": "sess_123"}'
```

### 恢复挂起的 Agent

```bash
curl -X POST http://localhost:8080/v2/resume \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "session_id": "sess_123",
    "reply_id": "reply_456",
    "confirm_id": "confirm_789",
    "decisions": [{"tool_call_id": "tc_xxx", "allowed": true}]
  }'
```

---

## 下一步

- [核心概念](concepts.md) — 深入理解事件系统、AgentState、Workspace
- [API 参考](api-reference.md) — 完整接口速查
- [部署指南](deployment.md) — Docker、K8s 部署
