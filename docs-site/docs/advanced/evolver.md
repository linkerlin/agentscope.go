# GEP 自演化指南

AgentScope.Go 对齐了 [Evolver](https://github.com/EvoMap/evolver) 的核心优势——基于 GEP（Gene Evolution Protocol）的自演化能力。

## 核心概念

### Gene（策略基因）

Gene 是可审计、可复用、可固化的演化资产，远优于松散的 ad-hoc prompt：

```go
type Gene struct {
    ID            string
    Category      string // repair / optimize / innovate / explore
    SignalsMatch  []string
    Strategy      string
    Constraints   []string
    Validation    string
    BlastRadius   float64
    ToolPolicy    map[string]string
    Epigenetic    map[string]any
}
```

### Capsule（成功快照）

Capsule 记录成功执行的上下文，用于未来复用：

```go
type Capsule struct {
    ID             string
    GeneID         string
    ExecutionTrace []Step
    Outcome        Outcome
    BlastRadius    float64
}
```

## 快速使用

### 1. 创建 GEP Flow

```go
import "github.com/linkerlin/agentscope.go/evolver"

// Mock 模式（开发测试）
flow := evolver.NewGEPFlow(evolver.NewMockEvolver())

// 生产模式：通过 MCP 连接真实 Evolver 后端
// flow := evolver.NewGEPFlow(evolver.NewMCPEvolver(mcpClient))
```

### 2. 运行并固化

```go
runCfg := evolver.RunConfig{
    Context:  "recurring gateway timeout on large payload",
    Strategy: "repair-only",
}

runRes, solRes, _ := flow.RunAndSolidify(ctx, runCfg, false)

fmt.Println("Selected gene:", runRes.SelectedGene.ID)
fmt.Println("Solidified capsule:", solRes.CapsuleID)
```

### 3. Skill 蒸馏为 Gene

```go
sk := &skill.AgentSkill{
    Name:        "timeout_recovery",
    Description: "Handle gateway timeouts",
    SkillContent: "...",
}

gene := sk.DistillToGene(evolver.CategoryRepair)
flow.Client.UpsertGene(ctx, gene)
```

### 4. 演化记忆

```go
// 记录
_ = flow.Client.Remember(ctx, evolver.RememberRequest{
    Text:     "Fixed timeout by increasing buffer size",
    Type:     "capsule",
    Category: evolver.CategoryRepair,
})

// 召回
hits, _ := flow.Client.Recall(ctx, evolver.RecallRequest{
    Query:    "timeout",
    Category: "capsule",
})
```

## 生产集成

在 Gateway 中启用 Evolver：

```go
srv := gateway.NewApp(gateway.AppConfig{
    // ...
    EvolverEnabled: true, // 通过 MCP 网关暴露 evolver 工具
})
```

Agent 即可通过工具调用 `evolver__evolver_run` / `evolver_solidify` 等触发 GEP 流程。

## 与 ReMe 结合

```go
// ReMe 承载 Gene/Capsule 资产
memoryType := memory.MemoryTypeGene
geneMemory := &memory.EvoMemory{
    Type: memoryType,
    Gene: gene,
}
reme.Remember(ctx, geneMemory)
```

## 与 Python 版对比

| 能力 | Python 2.0 | Go 2.0.0 |
|------|-----------|----------|
| GEP 实现 | ❌ 无 | ✅ 完整 |
| Gene/Capsule | ❌ 无 | ✅ 类型化 |
| Skill 蒸馏 | ❌ 无 | ✅ Skill2GEP |
| 演化记忆 | ❌ 无 | ✅ ReMe 承载 |
| MCP 桥接 | ❌ 无 | ✅ 轻量集成 |
