# GEP 自演化指南

AgentScope.Go 的 `evolver/` 包对齐 [Evolver](https://github.com/EvoMap/evolver) 的 GEP（Gene Evolution Protocol），让 Agent 能够将成功经验固化为可复用、可审计的“策略基因”。

---

## 1. 核心概念

| 概念 | 说明 |
|------|------|
| **Gene** | 策略基因：包含 signals_match、strategy、constraints、validation、routing_hint |
| **Capsule** | 成功演化快照：包含 blast_radius、outcome、execution_trace、derivation_tokens |
| **Run** | 针对问题上下文运行 GEP，提取信号并选择最佳 Gene |
| **Reflect** | 风险检测与影响面分析 |
| **Solidify** | 验证、记录事件、更新 Gene、保存 Capsule |
| **Skill2GEP** | 将 ad-hoc Skill 蒸馏为 Gene |

---

## 2. 快速开始

```go
package main

import (
    "context"
    "fmt"

    "github.com/linkerlin/agentscope.go/evolver"
)

func main() {
    ctx := context.Background()

    // MockEvolver 用于本地演示；生产环境接入真实 MCP 后端
    client := evolver.NewMockEvolver()
    flow := evolver.NewGEPFlow(client)

    runCfg := evolver.RunConfig{
        Context:  "recurring gateway timeout on large payload",
        Strategy: "repair-only",
    }

    runRes, solRes, err := flow.RunAndSolidify(ctx, runCfg, false)
    if err != nil {
        panic(err)
    }

    fmt.Println("Selected gene:", runRes.SelectedGene.ID)
    fmt.Println("Solidified capsule:", solRes.CapsuleID)
}
```

---

## 3. 将 Skill 蒸馏为 Gene

```go
import "github.com/linkerlin/agentscope.go/skill"

sk := &skill.AgentSkill{
    Name:         "timeout_recovery",
    Description:  "Recover from gateway timeout",
    SkillContent: "When timeout occurs on large payload, retry with chunked upload...",
}

gene := sk.DistillToGene(evolver.CategoryRepair)
flow.Client.UpsertGene(ctx, gene)
```

---

## 4. 演化记忆 recall

```go
// 记录成功演化资产
_ = flow.Client.Remember(ctx, evolver.RememberRequest{
    Text:     "Chunked upload fixed large payload timeout",
    Type:     "capsule",
    Category: evolver.CategoryRepair,
})

// 后续遇到类似问题时检索
hits, _ := flow.Client.Recall(ctx, evolver.RecallRequest{
    Query:    "timeout large payload",
    Category: "capsule",
})
for _, h := range hits {
    fmt.Println(h.Text)
}
```

---

## 5. 生产集成

在生产环境中，Evolver 通过 Gateway 的 MCP 网关暴露为工具：

```go
appCfg := gateway.AppConfig{
    EvolverEnabled: true,
    // ...
}
```

启动后，session agent 可直接调用 MCP 工具：

- `evolver__evolver_run`
- `evolver__evolver_solidify`
- `evolver__remember`
- `evolver__recall`

结合 ReMe 记忆系统，可实现：遇到错误 → 触发 GEP → 选择 Gene → 应用修复 → 固化 Capsule → 审计记录。

---

## 6. 会议与协作

Evolver 支持结构化演化会议：

```go
flow.Client.MeetingStart(ctx, evolver.MeetingRequest{Type: "debug", Topic: "timeout issue"})
flow.Client.MeetingProceed(ctx, ...)
flow.Client.MeetingFinalize(ctx, ...)
```

---

## 7. 安全与审计

- 所有 GEP 操作返回 safety_status 与事件列表
- Solidify 支持 `dryRun` 模式先验证
- Capsule 记录 derivation_tokens 与 execution_trace，便于审计
- 建议结合 Workspace 与 PermissionEngine 限制演化过程中工具执行范围

---

## 8. 相关文件

- `evolver/types.go`
- `evolver/client.go`
- `evolver/gep.go`
- `skill/skill.go`
- `examples/evolver/main.go`
