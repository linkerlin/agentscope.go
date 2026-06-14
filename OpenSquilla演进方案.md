# OpenSquilla 演进方案 — agentscope.go 差距补齐与架构跃迁

> **版本**：2026-06-14 v1
> **基准**：OpenSquilla (Python, v0.3.1) 架构优势 vs. agentscope.go (Go, v2.0.0) 现状
> **核心命题**：OpenSquilla 在"模型路由智能化"、"Turn 状态机"、"安全纵深防御"、"Token 效率"四维度领先显著，而 agentscope.go 在"事件驱动 V2"、"ReMe 记忆"、"A2A 协议"上自有优势。本方案非照搬 Python 模式，而是以 Go 地道方式吸纳 OpenSquilla 之设计精髓，使 agentscope.go 从"功能完备"跃迁至"智能高效"。
> **与既有方案之关系**：V2 演进方案（P0-P3，已完成）、ReMe 记忆演进（五阶段完成）、DEV_PLAN_CATCHUP（Phase 1-4 完成，Phase 5 进行中）——本方案不重复上述内容，专注 OpenSquilla 差距。

---

## 一、两项目架构总览

### 1.1 OpenSquilla 架构

```
opensquilla/
├── agents/          # Agent 注册表 + 工厂
├── application/     # 应用引导 + 生命周期
├── channels/        # 多通道合约适配器 (CLI/WebUI/Chat)
├── chat/            # 对话管理
├── cli/             # 命令行界面
├── compat/          # 兼容层
├── contracts/       # 稳定契约接口 (跨边界共享)
├── contrib/         # 第三方贡献
├── dist/            # 打包分发
├── engine/          # 🔥 核心引擎: Agent 状态机 + TurnRunner + Subagent
├── gateway/         # HTTP 网关
├── health/          # 健康检查
├── identity/        # 🔥 三文档合并 (SOUL+IDENTITY+AGENTS)
├── mcp/             # MCP Client (SSE/Stdio)
├── mcp_server/      # MCP Server 桥接
├── memory/          # 🔥 混合检索 (FTS5+向量+MMR+时间衰减)
├── migration/       # 数据迁移
├── observability/   # 🔥 决策日志 + SavingsTelemetry + PrivacyGuardSink
├── onboarding/      # 首次配置引导
├── persistence/     # 持久化层
├── plugins/         # 插件系统
├── provider/        # 🔥 多 LLM Provider (20+) + Plugin Hooks
├── safety/          # 🔥 五层纵深防御
├── sandbox/         # 进程级沙箱
├── scheduler/       # 任务调度
├── search/          # Web 搜索集成
├── session/         # 会话管理
├── skills/          # 🔥 DAG 编排 + MetaOrchestrator
├── squilla_router/  # 🔥 四层 ML 模型路由 (ONNX+LightGBM)
└── tools/           # 21+ 内置工具
```

### 1.2 agentscope.go 架构

```
agentscope.go/
├── a2a/             # A2A 协议 (Go 领先 Py)
├── agent/           # Agent 接口 (V1/V2) + Base 生命周期
├── async/           # 异步任务池
├── config/          # 配置管理 (仅 JSON)
├── credential/      # 凭据管理
├── embedding/       # 独立嵌入包 (多 Provider + ONNX)
├── event/           # 🔥 20+ 事件类型 + Bus
├── evolver/         # GEP 自演化 (Gene/Capsule)
├── formatter/       # 独立格式化器 (5 Provider)
├── gateway/         # HTTP/SSE/WS Gateway + AG-UI
├── hook/            # 钩子系统
├── interruption/    # 中断管理
├── loader/          # 文档加载器
├── memory/          # 🔥 ReMe 5 实现 + 7 向量后端 + BM25 + 知识图谱
├── message/         # 多模态 Msg
├── middleware/       # 洋葱中间件 (5 类)
├── model/           # 10+ ChatModel 后端 + Router
├── observability/   # OTel + LangSmith
├── permission/      # 🔥 权限引擎 (599 行, 5 Mode)
├── pipeline/        # Pipeline + Parallel
├── plan/            # PlanNotebook
├── reflection/      # Writer-Critic 反思
├── schedule/        # Cron 调度器
├── service/         # 多租户 Storage + Auth
├── session/         # 会话管理
├── shutdown/        # 优雅关闭
├── skill/           # SkillBox + SkillViewer
├── state/           # AgentState 序列化
├── tool/            # file/shell/web/json/task/schedule/subagent/multimodal
├── toolkit/         # MCP Client/Server 适配
├── workflow/        # Condition + Loop + MapReduce
└── workspace/       # Local + Docker + E2B
```

### 1.3 核心差异对照

| 维度 | OpenSquilla (Python) | agentscope.go (Go) | 差距 |
|------|---------------------|---------------------|------|
| 语言生态 | Python 3.12+, 依赖 ONNX/LightGBM/NumPy | Go 1.25+, 零 CGO, 单二进制 | Go 部署优势 |
| 核心范式 | 微内核 + 显式状态机 + 异步生成器 | 事件驱动 V2 + ReAct 循环 | 范式接近，Go 粒度更粗 |
| 模型路由 | 🔥 四层 ML 路由 (89% 成本降低) | 静态 Router(主+备) + CircuitBreaker | **严重差距** |
| Turn 结构 | 🔥 7 阶段 TurnRunner | replyInternal ~340 行 for 循环 | **严重差距** |
| 安全防御 | 🔥 五层纵深 (注入→分级→沙箱→过滤→审计) | Permission Engine 成熟，缺注入防御/输出过滤/审计 | **显著差距** |
| 工具效率 | 🔥 Tokenjuice 动态压缩 + 子集选择 | 有 context compression + tool result truncation | **中高差距** |
| 记忆系统 | 混合检索 + MMR + 时间衰减 + 嵌入 Provider | 🔥 ReMe 极成熟 (BM25+图谱+Dream+ReMe) | Go 已领先，仅缺 MMR/TimeDecay |
| A2A 协议 | 无 | 🔥 完整实现 + 分布式注册 + 分片路由 | Go 已领先 |
| 事件范式 | AgentEvent 流式生成器 | 🔥 20+ 事件类型 + Bus + AG-UI 映射 | Go 已领先 |
| 工具生态 | 21+ 内置工具 | file/shell/web/json/task/schedule/subagent/multimodal | 基本对齐 |

---

## 二、差距总览与 ROI 优先级

| # | 差距项 | OpenSquilla 有 | agentscope.go 现状 | 影响力(1-5) | 可行性(1-5) | ROI | 优先级 |
|---|--------|---------------|---------------------|-----------|-----------|-----|--------|
| 1 | 智能模型路由 | SquillaRouter 4层ML路由 + V4Phase3Strategy + T0-T3/P0-P2 | 静态 Router(主+备) + MultimodalRouter(媒体检测) | 5 | 4 | 20 | **P0** |
| 2 | Turn 状态机 | AgentState enum + 7阶段 TurnRunner async generator | replyInternal() ~340行 for 循环，无阶段分解 | 4 | 4 | 16 | **P0** |
| 3 | 安全纵深防御 | 5层: InjectionGuard→RiskTier→Sandbox→OutputFilter→AuditLog | Permission Engine 成熟(599行)，但缺注入防御/输出过滤/审计日志 | 5 | 3 | 15 | **P1** |
| 4 | 工具 Token 压缩 | Tokenjuice 动态压缩工具描述 + 上下文相关子集 | 有 context compression + tool result truncation，但无工具描述压缩/动态子集 | 4 | 3 | 12 | **P1** |
| 5 | 决策日志与可观测 | DecisionEntry + TraceContext + SavingsTelemetry + PrivacyGuardSink | OTel + LangSmith + JsonlTraceExporter，但缺结构化决策审计 | 3 | 4 | 12 | **P1** |
| 6 | Skills DAG 编排 | MetaOrchestrator + DAG + 6 step kinds + SOP compilation | SkillBox + SkillViewer + load_skill，但无 DAG/条件激活/IO Schema | 3 | 3 | 9 | **P2** |
| 7 | Provider Plugin Hooks | failover_hook + quota_hook + pre/post processing hooks | Router + CircuitBreaker，但缺 pre/post hooks / 多级 failover / quota | 3 | 3 | 9 | **P2** |
| 8 | Channel 合约系统 | ChannelCapabilityProfile + GREEN/YELLOW/RED + Contract adapter | Gateway + AG-UI + A2A，但无统一 Channel 抽象 | 2 | 3 | 6 | **P3** |
| 9 | Identity 三文档合并 | SOUL.md + IDENTITY.md + AGENTS.md + Jinja2 rendering | SysPrompt + PersonalSummarizer，但无 Persona 对象 | 1 | 4 | 4 | **P3** |
| 10 | 记忆: MMR + Time Decay | MMR diversity + time decay scoring + embedding provider | ReMe 极其成熟，仅缺 MMR/TimeDecay/Reranker | 2 | 4 | 8 | **P2** |

**排序逻辑**：ROI = 影响力 x 可行性，P0 最高优先。

---

## 三、总体架构设计

```
                        ┌─────────────────────────────────────┐
                        │          Intelligent Router         │
                        │  SquillaRouter → RouteTable → Model │
                        │  (规则 + ML分数 + 成本优化)          │
                        └───────────┬─────────────────────────┘
                                    │
                        ┌───────────▼─────────────────────────┐
                        │          Turn State Machine          │
                        │  Receive→Classify→Route→Assemble     │
                        │  →Dispatch→Observe→Respond           │
                        │  (每阶段可挂 hook / 可 suspend)       │
                        └───────────┬─────────────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────────┐
              │                     │                         │
  ┌───────────▼─────────┐ ┌────────▼──────────┐ ┌────────────▼──────────┐
  │  Safety Defense     │ │ Decision Audit    │ │  Token Compressor    │
  │  InjectionGuard     │ │ Trail             │ │  (Tool Description   │
  │  RiskTier→Sandbox   │ │ DecisionEntry     │ │   Subset Selection)  │
  │  OutputFilter       │ │ TraceContext      │ │  + Dynamic Subset    │
  │  AuditLog           │ │ SavingsTelemetry │ │                     │
  └─────────────────────┘ └───────────────────┘ └─────────────────────┘
```

---

## 四、阶段一：智能模型路由 (P0) — SquillaRouter Go 版

### 4.1 目标
将 agentscope.go 的静态 `Router`(primary+fallback) 升级为多层智能路由，每轮对话自动选择"最便宜且胜任"的模型。

### 4.2 OpenSquilla 参考
- SquillaRouter: ONNX + LightGBM 4层路由
- V4Phase3Strategy: ThinkingMode(T0-T3) + PromptPolicy(P0-P2)
- PinchBench 验证: 89% 成本降低，质量持平

### 4.3 Go 实现方案

**不照搬 ONNX/LightGBM**（违背零 CGO 原则），而是用 Go 地道方式实现等价智能路由：

#### 4.3.1 核心接口 — `model/squilla/router.go`

```go
package squilla

// RouteDecision 记录路由决策
type RouteDecision struct {
    ModelName    string    `json:"model_name"`
    ThinkingMode ThinkMode `json:"thinking_mode"`
    PromptPolicy PromptPol `json:"prompt_policy"`
    Reason       string    `json:"reason"`
    CostSaved    float64   `json:"cost_saved,omitempty"`
    RoutedAt     time.Time `json:"routed_at"`
}

// ThinkMode 对齐 V4Phase3Strategy ThinkingMode
type ThinkMode int
const (
    T0 ThinkMode = iota // 无需推理，直接回答
    T1                   // 简短推理 (<100 tokens thinking)
    T2                   // 标准推理 (100-500 tokens thinking)
    T3                   // 深度推理 (>500 tokens thinking)
)

// PromptPol 对齐 V4Phase3Strategy PromptPolicy
type PromptPol int
const (
    P0 PromptPol = iota // 全量工具描述
    P1                   // 压缩工具描述 (子集)
    P2                   // 极简提示 (仅系统提示)
)

// SquillaRouter 智能模型路由器
type SquillaRouter struct {
    models     []ModelEntry       // 候选模型列表(按能力分级)
    classifier *TurnClassifier    // 意图分类器
    scorer     *CostQualityScorer // 成本-质量评分器
    history    *RouteHistory       // 历史路由记录
    rules      []RouteRule         // 静态规则(最高优先)
}

// ModelEntry 描述一个候选模型
type ModelEntry struct {
    Model            model.ChatModel
    Tier             int     // 1=旗舰, 2=标准, 3=经济
    CostPer1kIn      float64 // 输入每千token成本
    CostPer1kOut     float64 // 输出每千token成本
    MaxTokens        int
    SupportsThinking bool
    SupportsVision   bool
    SupportsTools    bool
}

// Route 执行路由决策
func (r *SquillaRouter) Route(ctx context.Context, messages []*message.Msg, opts []model.ChatOption) (*RouteDecision, error)
```

#### 4.3.2 四层路由架构

| 层 | 名称 | 输入 | 输出 | Go 实现 |
|----|------|------|------|---------|
| L1 | **静态规则** | 工具调用存在？媒体内容？ | 跳过 L2-L4 直接选模型 | 规则引擎 (现有 MultimodalRouter 逻辑升级) |
| L2 | **意图分类** | 用户消息语义 | ThinkingMode + PromptPolicy | 本地小型分类器 (ONNX-free，用 token 关键词 + embedding 余弦) |
| L3 | **成本优化** | L2 输出 + 历史质量数据 | 最便宜胜任模型 | CostQualityScorer (延迟/质量/成本三维优化) |
| L4 | **动态降级** | CircuitBreaker 状态 + quota | 备选模型 | 复用现有 circuitBreaker + 新增 QuotaTracker |

#### 4.3.3 TurnClassifier — 意图分类 (替代 ONNX)

```go
// TurnClassifier 用关键词 + embedding 余弦分类，无需 ONNX
type TurnClassifier struct {
    embedder   embedding.Provider       // 复用 embedding/ 包
    prototypes map[ThinkMode][]float64  // 预计算原型向量
    kwRules    []KeywordRule             // 关键词规则
}

type KeywordRule struct {
    Keywords []string
    Mode     ThinkMode
    Policy   PromptPol
}

// Classify 分析当前 turn，输出 ThinkingMode + PromptPolicy
func (c *TurnClassifier) Classify(ctx context.Context, messages []*message.Msg) (ThinkMode, PromptPol, error)
```

**关键词规则举例**：
- T0: "hi", "hello", "thanks", "ok" → 无工具、短对话
- T1: 包含工具调用，且工具 ≤2 个
- T2: 包含多步推理关键词 ("analyze", "compare", "为什么")
- T3: 包含深度推理关键词 ("prove", "formal", "mathematical proof")

#### 4.3.4 CostQualityScorer — 成本质量评分

```go
type CostQualityScorer struct {
    history *RouteHistory
}

// Score 返回给定模型处理给定请求的综合评分 (越高越好)
// score = qualityScore * qualityWeight - costScore * costWeight + latencyBonus
func (s *CostQualityScorer) Score(entry ModelEntry, mode ThinkMode, messages []*message.Msg) float64
```

#### 4.3.5 RouteHistory — 路由历史与反馈

```go
// RouteHistory 跟踪路由决策与结果，用于反馈优化
type RouteHistory struct {
    entries []RouteFeedback // 环形缓冲，最近 N 条
    mu      sync.RWMutex
}

type RouteFeedback struct {
    Decision     RouteDecision
    ActualModel  string
    QualityScore float64 // 1-5 分 (基于用户反馈或自动化评估)
    ActualCost   float64
    LatencyMs    int64
}
```

#### 4.3.6 集成点 — 替换 `chatModel` 字段

```go
// ReActAgentBuilder 新增
func (b *ReActAgentBuilder) SquillaRouter(r *squilla.SquillaRouter) *ReActAgentBuilder
```

当 SquillaRouter 已配置时，ReActAgent 每轮推理前调用 `router.Route()` 获取 `RouteDecision`，用 `Decision.ModelName` 对应的 ModelEntry.Model 替代 `a.chatModel` 发起调用。

### 4.4 涉及文件

| 文件 | 变更 |
|------|------|
| `model/squilla/router.go` | **新建** — SquillaRouter 核心 |
| `model/squilla/classifier.go` | **新建** — TurnClassifier |
| `model/squilla/scorer.go` | **新建** — CostQualityScorer |
| `model/squilla/history.go` | **新建** — RouteHistory |
| `model/squilla/rules.go` | **新建** — 内置规则 |
| `model/squilla/router_test.go` | **新建** |
| `model/router.go` | **微调** — Router 改为 SquillaRouter 的降级模式 |
| `model/multimodal_router.go` | **微调** — 整合进 SquillaRouter L1 层 |
| `agent/react/react_agent.go` | **微调** — Builder 新增 SquillaRouter 选项 |
| `agent/react/reply_stream.go` | **微调** — runModelStream 前 route |

### 4.5 工作量估算

| 子项 | 估算 |
|------|------|
| SquillaRouter 核心结构 + Route() | 3 天 |
| TurnClassifier (关键词 + embedding) | 3 天 |
| CostQualityScorer + RouteHistory | 2 天 |
| 集成 ReActAgent + Builder | 2 天 |
| 测试 + 基准 | 2 天 |
| **总计** | **~12 天** |

### 4.6 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 分类精度不够，路由效果差 | 中 | 高 | L1 静态规则兜底；L2 分类失败时降级到主模型 |
| embedding 查询增加延迟 | 低 | 中 | 缓存 embedding + 异步预计算 |
| 与现有 Router/CircuitBreaker 冲突 | 低 | 低 | SquillaRouter 内嵌 CircuitBreaker 逻辑 |

---

## 五、阶段二：Turn 状态机 (P0) — 7阶段 TurnRunner

### 5.1 目标
将 `replyInternal()` 的 340 行 for 循环重构为显式 7 阶段状态机，每阶段可独立 hook、可 suspend、可观测。

### 5.2 OpenSquilla 参考
- AgentState enum + async generator
- 7阶段 TurnRunner: receive → classify → route → assemble → dispatch → observe → respond

### 5.3 Go 实现方案

#### 5.3.1 核心类型 — `agent/turn/turn.go`

```go
package turn

// Phase 表示 Turn 的一个阶段
type Phase int
const (
    PhaseReceive   Phase = iota // 0: 接收用户消息
    PhaseClassify               // 1: 意图分类 (对接 SquillaRouter L2)
    PhaseRoute                  // 2: 模型路由 (对接 SquillaRouter)
    PhaseAssemble               // 3: 组装提示 (工具子集 + 记忆注入)
    PhaseDispatch               // 4: 调用模型
    PhaseObserve                // 5: 观察结果 (工具执行 + 安全检查)
    PhaseRespond                // 6: 生成最终响应
)

// PhaseFunc 是单个阶段的处理函数
type PhaseFunc func(ctx context.Context, s *TurnState) (*TurnState, PhaseAction, error)

// PhaseAction 控制阶段转换
type PhaseAction int
const (
    ActionNext    PhaseAction = iota // 进入下一阶段
    ActionGoto                        // 跳转到指定阶段
    ActionSuspend                     // 挂起 (HITL)
    ActionReturn                      // 直接返回结果
    ActionError                       // 返回错误
)

// TurnState 是跨阶段共享的状态
type TurnState struct {
    Phase          Phase
    Iteration      int
    InputMsg       *message.Msg
    History        []*message.Msg
    RouteDecision  *squilla.RouteDecision  // PhaseRoute 产出
    ModelResponse  *message.Msg            // PhaseDispatch 产出
    ToolResults    []*message.Msg          // PhaseObserve 产出
    FinalResponse  *message.Msg            // PhaseRespond 产出
    Metadata       map[string]any          // 跨阶段元数据
}

// TurnRunner 驱动 Turn 的阶段流转
type TurnRunner struct {
    phases   map[Phase]PhaseFunc
    hooks    []TurnHook
    maxIters int
}

// Run 执行一个完整 Turn (可能包含多轮 PhaseDispatch→PhaseObserve 循环)
func (r *TurnRunner) Run(ctx context.Context, input *TurnState) (*TurnState, error)
```

#### 5.3.2 与现有 ReActAgent 的关系

**不重写 ReActAgent**，而是将 TurnRunner 作为 `replyInternal()` 的内部引擎：

```go
func (a *ReActAgent) replyInternal(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
    runner := a.buildTurnRunner()
    state := &TurnState{InputMsg: msg, Phase: PhaseReceive}
    // ... 初始化 state
    finalState, err := runner.Run(ctx, state)
    // ... 提取 finalState.FinalResponse
}
```

#### 5.3.3 TurnHook — 阶段级钩子

```go
type TurnHook interface {
    OnPhase(ctx context.Context, phase Phase, state *TurnState) (*TurnState, PhaseAction, error)
}
```

这使得 SquillaRouter 可作为 `PhaseRoute` 的 hook 注入，安全检查可作为 `PhaseObserve` 的 hook 注入。

#### 5.3.4 可观测性 — 每阶段 emit 事件

在 `TurnRunner.Run()` 中，每进入/离开一个 Phase 都 emit 对应的 `TurnPhaseEvent`，复用现有 `event.Bus`：

```go
type TurnPhaseEvent struct {
    baseEvent
    Phase     Phase        `json:"phase"`
    Iteration int          `json:"iteration"`
    Action    PhaseAction  `json:"action"`
}
```

### 5.4 涉及文件

| 文件 | 变更 |
|------|------|
| `agent/turn/turn.go` | **新建** — TurnRunner 核心 |
| `agent/turn/state.go` | **新建** — TurnState |
| `agent/turn/hook.go` | **新建** — TurnHook 接口 |
| `agent/turn/runner.go` | **新建** — Run() 主逻辑 |
| `agent/react/react_agent.go` | **重构** — replyInternal 委托给 TurnRunner |
| `agent/react/reply_stream.go` | **重构** — replyStreamInternal 委托给 TurnRunner |
| `event/turn_events.go` | **新建** — TurnPhaseEvent |

### 5.5 工作量估算

| 子项 | 估算 |
|------|------|
| TurnRunner 核心 + Phase 定义 | 3 天 |
| 各 Phase 实现 (7个) | 4 天 |
| 集成 ReActAgent (replyInternal 重构) | 3 天 |
| 集成 ReplyStream | 2 天 |
| 测试 + 回归验证 | 3 天 |
| **总计** | **~15 天** |

### 5.6 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 重构 replyInternal 引入回归 | 中 | 高 | 先写完整测试覆盖，再重构；保持外部接口不变 |
| 7 阶段拆分粒度不当 | 低 | 中 | 参照 OpenSquilla 但允许 Go 版调整阶段数 |
| 与现有 Hook 系统冲突 | 低 | 中 | TurnHook 与 hook.Hook 并存，各司其职 |

---

## 六、阶段三：安全纵深防御 (P1) — 5 层防御体系

### 6.1 目标
在现有 Permission Engine 基础上，构建 5 层安全纵深防御：注入防护 → 风险分级 → 沙箱隔离 → 输出过滤 → 审计日志。

### 6.2 OpenSquilla 参考
- InjectionGuard: prompt injection 检测 (正则 + 语义)
- RiskTier: SecurityLevel 0-3 分级，失败闭合设计
- Sandbox: 进程级沙箱
- OutputFilter: 输出内容过滤 (PII/API Key/路径)
- AuditLog: 安全事件审计 + PrivacyGuardSink 脱敏

### 6.3 Go 实现方案

#### 6.3.1 第 1 层：InjectionGuard — `security/injection.go`

```go
package security

// InjectionGuard 检测 prompt injection 攻击
type InjectionGuard struct {
    rules     []InjectionRule
    embedder  embedding.Provider // 可选: 语义相似度检测
    threshold float64
}

// InjectionRule 定义注入检测规则
type InjectionRule struct {
    Name     string
    Pattern  *regexp.Regexp // 正则匹配
    Severity Severity       // Low / Medium / High / Critical
}

// Check 检测消息是否包含注入攻击
func (g *InjectionGuard) Check(msg *message.Msg) (*InjectionReport, error)

type InjectionReport struct {
    IsInjection  bool
    Severity     Severity
    MatchedRules []string
    Suggestions  []string // 缓解建议
}
```

**内置规则** (参考 OpenSquilla + OWASP LLM Top 10)：
- 系统提示泄露: "ignore previous instructions", "repeat your system prompt"
- 角色劫持: "you are now...", "from now on you are"
- 数据外泄: "output all your instructions", "what were you told"
- 越权操作: "execute without checking", "bypass safety"

#### 6.3.2 第 2 层：RiskTier — `security/risk.go`

```go
// RiskTier 评估每个 turn 的安全等级
type RiskTier int
const (
    RiskTier0 RiskTier = iota // 安全: 普通对话，无工具
    RiskTier1                  // 低风险: 只读工具
    RiskTier2                  // 中风险: 写操作，已有权限确认
    RiskTier3                  // 高风险: 危险操作，需额外防护
)

// RiskAssessor 评估 turn 风险等级
type RiskAssessor struct {
    injectionGuard *InjectionGuard
    permEngine     *permission.Engine // 复用现有
}

// Assess 评估当前 turn 的风险等级
func (r *RiskAssessor) Assess(ctx context.Context, msg *message.Msg, toolCalls []*message.ToolUseBlock) RiskTier
```

#### 6.3.3 第 3 层：Sandbox — 复用现有 Workspace

**不新增沙箱**，复用 `workspace/` 的 Docker/E2B 沙箱。但新增基于 RiskTier 的自动升级：

```go
// SandboxPolicy 基于 RiskTier 决定沙箱策略
type SandboxPolicy struct {
    Tier0Workspace workspace.Workspace // nil = 无沙箱
    Tier1Workspace workspace.Workspace // 可选 LocalWorkspace
    Tier2Workspace workspace.Workspace // 推荐 DockerWorkspace
    Tier3Workspace workspace.Workspace // 必须 E2BWorkspace
}
```

#### 6.3.4 第 4 层：OutputFilter — `security/filter.go`

```go
// OutputFilter 在模型输出返回给用户前进行过滤
type OutputFilter struct {
    rules    []OutputRule
    replacer func(string) string // 脱敏替换函数
}

type OutputRule struct {
    Name    string
    Pattern *regexp.Regexp
    Action  FilterAction // Redact / Block / Warn
}

// Filter 过滤模型输出
func (f *OutputFilter) Filter(msg *message.Msg) (*message.Msg, []FilterMatch, error)
```

**内置规则**：
- PII 检测: 邮箱/电话/信用卡号/SSN
- API Key 泄露: `sk-...`, `ghp_...`, `AKIA...`
- 内部路径泄露: `/home/...`, `/etc/...`

#### 6.3.5 第 5 层：AuditLog — `security/audit.go`

```go
// AuditEntry 记录一个安全事件
type AuditEntry struct {
    ID         string    `json:"id"`
    Timestamp  time.Time `json:"timestamp"`
    AgentName  string    `json:"agent_name"`
    TurnID     string    `json:"turn_id"`
    Phase      string    `json:"phase"`
    RiskTier   RiskTier  `json:"risk_tier"`
    EventType  string    `json:"event_type"` // "injection_blocked" / "output_filtered" / "permission_denied" / "sandbox_escalation"
    Detail     string    `json:"detail"`
    Blocked    bool      `json:"blocked"`
    SchemaVer  int       `json:"schema_ver"` // 版本化，对齐 OpenSquilla
}

// AuditSink 写入审计日志
type AuditSink interface {
    Write(ctx context.Context, entry AuditEntry) error
}

// PrivacyGuardSink 脱敏后写入 (对齐 OpenSquilla PrivacyGuardSink)
type PrivacyGuardSink struct {
    inner       AuditSink
    piiReplacer func(string) string
}
```

### 6.4 集成点 — 作为 TurnRunner Hook 注入

```go
// SecurityHook 实现 TurnHook，在 PhaseObserve 阶段执行安全检查
type SecurityHook struct {
    guard         *InjectionGuard
    assessor      *RiskAssessor
    outputFilter  *OutputFilter
    auditSink     AuditSink
    sandboxPolicy *SandboxPolicy
}

func (h *SecurityHook) OnPhase(ctx context.Context, phase turn.Phase, state *turn.TurnState) (*turn.TurnState, turn.PhaseAction, error) {
    switch phase {
    case turn.PhaseReceive:
        // 第 1 层: InjectionGuard
        report, err := h.guard.Check(state.InputMsg)
        if err != nil { return state, turn.ActionError, err }
        if report.IsInjection && report.Severity >= SeverityHigh {
            h.auditSink.Write(ctx, AuditEntry{...})
            return state, turn.ActionReturn, fmt.Errorf("injection blocked: %v", report.MatchedRules)
        }
    case turn.PhaseObserve:
        // 第 4 层: OutputFilter
        filtered, matches, err := h.outputFilter.Filter(state.ModelResponse)
        // ...
    }
    return state, turn.ActionNext, nil
}
```

### 6.5 涉及文件

| 文件 | 变更 |
|------|------|
| `security/injection.go` | **新建** — InjectionGuard |
| `security/risk.go` | **新建** — RiskTier + RiskAssessor |
| `security/filter.go` | **新建** — OutputFilter |
| `security/audit.go` | **新建** — AuditLog + AuditSink |
| `security/sandbox_policy.go` | **新建** — SandboxPolicy |
| `security/injection_test.go` | **新建** |
| `security/filter_test.go` | **新建** |
| `security/audit_test.go` | **新建** |

### 6.6 工作量估算

| 子项 | 估算 |
|------|------|
| InjectionGuard (规则 + 检测) | 3 天 |
| RiskTier + RiskAssessor | 2 天 |
| SandboxPolicy (复用 workspace) | 1 天 |
| OutputFilter (PII + API Key + 路径) | 2 天 |
| AuditLog + PrivacyGuardSink | 2 天 |
| 集成 TurnRunner SecurityHook | 2 天 |
| 测试 | 3 天 |
| **总计** | **~15 天** |

### 6.7 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| InjectionGuard 误杀率高 | 中 | 高 | 可配置阈值 + 白名单 + audit 审查 |
| OutputFilter 漏检 PII | 中 | 中 | 多层规则 + 可扩展 |
| 性能开销 | 低 | 低 | OutputFilter 可选，审计异步写入 |

---

## 七、阶段四：工具 Token 压缩 (P1) — TokenJuice Go 版

### 7.1 目标
动态压缩工具描述 + 上下文相关子集选择，减少 prompt token 消耗。

### 7.2 OpenSquilla 参考
- Tokenjuice: 压缩工具描述，动态选择子集
- 基于上下文相关性的工具子集选择

### 7.3 Go 实现方案

#### 7.3.1 核心接口 — `agent/react/tool_compress.go`

```go
// ToolCompressor 工具描述压缩器
type ToolCompressor struct {
    embedder      embedding.Provider
    cache         map[string]CompressedTool // 缓存压缩结果
    maxToolTokens int // 单个工具描述最大 token 数
}

// CompressedTool 压缩后的工具描述
type CompressedTool struct {
    Original         model.ToolSpec
    Compressed       model.ToolSpec
    CompressionRatio float64
}

// Compress 压缩工具描述
func (c *ToolCompressor) Compress(ctx context.Context, tool model.ToolSpec) (model.ToolSpec, error)
```

#### 7.3.2 工具子集选择 — `agent/react/tool_selector.go`

```go
// ToolSubsetSelector 基于上下文相关性选择工具子集
type ToolSubsetSelector struct {
    embedder embedding.Provider
    cache    map[string][]float64 // tool name → embedding
}

// Select 选择与当前对话最相关的工具子集
func (s *ToolSubsetSelector) Select(ctx context.Context, history []*message.Msg, allTools []model.ToolSpec, maxTools int) ([]model.ToolSpec, error)
```

**策略**：
1. 预计算每个工具描述的 embedding
2. 将最近 N 轮对话 embedding 平均作为上下文向量
3. 计算上下文向量与各工具 embedding 的余弦相似度
4. 取 Top-K 最相关工具 + 保留所有 "always_on" 工具

#### 7.3.3 集成点

在 TurnRunner 的 `PhaseAssemble` 阶段，替换现有 `a.toolSpecs(ctx)` 调用：

```go
// PhaseAssemble 实现
func (a *ReActAgent) assemblePhase(ctx context.Context, state *turn.TurnState) (*turn.TurnState, turn.PhaseAction, error) {
    allTools := a.toolSpecs(ctx)

    // 工具子集选择 (基于 PromptPolicy)
    switch state.RouteDecision.PromptPolicy {
    case squilla.P0:
        // 全量工具
        state.SelectedTools = allTools
    case squilla.P1:
        // 压缩工具描述 + 子集选择
        subset := a.toolSelector.Select(ctx, state.History, allTools, 10)
        state.SelectedTools = a.toolCompressor.CompressAll(ctx, subset)
    case squilla.P2:
        // 极简: 无工具
        state.SelectedTools = nil
    }
    return state, turn.ActionNext, nil
}
```

### 7.4 涉及文件

| 文件 | 变更 |
|------|------|
| `agent/react/tool_compress.go` | **新建** — ToolCompressor |
| `agent/react/tool_selector.go` | **新建** — ToolSubsetSelector |
| `agent/react/tool_compress_test.go` | **新建** |
| `agent/react/tool_selector_test.go` | **新建** |

### 7.5 工作量估算

| 子项 | 估算 |
|------|------|
| ToolCompressor | 2 天 |
| ToolSubsetSelector | 3 天 |
| 集成 PhaseAssemble | 1 天 |
| 测试 | 2 天 |
| **总计** | **~8 天** |

### 7.6 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 子集选择遗漏必要工具 | 中 | 高 | 保留 "essential" 标记工具始终在子集中 |
| 压缩后描述信息不足 | 中 | 中 | 可配置压缩比 + 保留关键参数 |
| embedding 计算开销 | 低 | 中 | 缓存 embedding + 异步预计算 |

---

## 八、阶段五：决策日志与可观测 (P1) — 结构化决策审计

### 8.1 目标
在现有 OTel + LangSmith 基础上，增加结构化决策审计轨迹 (Decision Audit Trail)。

### 8.2 OpenSquilla 参考
- DecisionEntry + TraceContext
- SavingsTelemetry (成本节省统计)
- PrivacyGuardSink (脱敏写入)
- Schema 版本化

### 8.3 Go 实现方案

#### 8.3.1 核心类型 — `observability/decision.go`

```go
package observability

// DecisionEntry 记录一个路由/安全/工具选择决策
type DecisionEntry struct {
    SchemaVer   int       `json:"schema_ver"`   // 版本号，对齐 OpenSquilla
    ID          string    `json:"id"`
    Timestamp   time.Time `json:"timestamp"`
    AgentName   string    `json:"agent_name"`
    TurnID      string    `json:"turn_id"`
    Phase       string    `json:"phase"`         // "route" / "safety" / "tool_select" / "compress"

    // 路由决策
    RouteFrom   string    `json:"route_from,omitempty"` // 候选模型
    RouteTo     string    `json:"route_to,omitempty"`   // 选定模型
    RouteReason string    `json:"route_reason,omitempty"`

    // 成本节省 (SavingsTelemetry)
    CostSaved   float64   `json:"cost_saved,omitempty"`
    TokensSaved int       `json:"tokens_saved,omitempty"`

    // 安全决策
    RiskTier    int       `json:"risk_tier,omitempty"`
    Blocked     bool      `json:"blocked,omitempty"`
    BlockReason string    `json:"block_reason,omitempty"`

    // 工具选择
    ToolsConsidered int    `json:"tools_considered,omitempty"`
    ToolsSelected   int    `json:"tools_selected,omitempty"`
    ToolsCompressed int    `json:"tools_compressed,omitempty"`
}

// DecisionTrail 写入决策审计轨迹
type DecisionTrail struct {
    sink   DecisionSink
    tracer trace.Tracer // 复用 OTel tracer
}

// Record 记录一个决策 (同时写入 sink 和 OTel span)
func (t *DecisionTrail) Record(ctx context.Context, entry DecisionEntry) error

// DecisionSink 决策持久化接口
type DecisionSink interface {
    Write(ctx context.Context, entry DecisionEntry) error
    Query(ctx context.Context, filter DecisionFilter) ([]DecisionEntry, error)
}

// JsonlDecisionSink JSONL 文件持久化 (复用现有 JsonlTraceExporter 模式)
type JsonlDecisionSink struct { ... }

// SavingsTelemetry 成本节省统计
type SavingsTelemetry struct {
    totalSaved  float64
    tokensSaved int64
}

// Report 生成成本节省报告
func (s *SavingsTelemetry) Report() SavingsReport
```

### 8.4 涉及文件

| 文件 | 变更 |
|------|------|
| `observability/decision.go` | **新建** — DecisionEntry + DecisionTrail |
| `observability/decision_sink.go` | **新建** — JsonlDecisionSink |
| `observability/savings.go` | **新建** — SavingsTelemetry |
| `observability/decision_test.go` | **新建** |

### 8.5 工作量估算：~6 天

---

## 九、阶段六：Skills DAG 编排 (P2) + Provider Plugin Hooks (P2) + 记忆补齐 (P2)

### 9.1 Skills DAG 编排

#### 9.1.1 目标
在现有 SkillBox + SkillViewer 基础上，增加 DAG 编排能力。

#### 9.1.2 Go 实现方案

```go
// skill/dag.go

// StepKind 步骤类型 (对齐 OpenSquilla 6 kinds)
type StepKind int
const (
    StepPrompt   StepKind = iota // 系统提示注入
    StepTool                      // 工具调用
    StepCondition                 // 条件分支
    StepParallel                 // 并行执行
    StepLoop                     // 循环
    StepSubAgent                 // 子 Agent 调用
)

// Step DAG 中的一个步骤
type Step struct {
    ID       string
    Kind     StepKind
    Config   map[string]any
    Depends  []string // 依赖的步骤 ID
    IOSchema *IOSchema // 输入输出 Schema
}

// IOSchema 步骤的输入输出类型声明
type IOSchema struct {
    Input  map[string]string // name → type
    Output map[string]string // name → type
}

// DAG 技能的有向无环图
type DAG struct {
    Steps    []*Step
    entryIDs []string // 入口步骤 ID
}

// Compile 编译 DAG 为可执行 SOP (Standard Operating Procedure)
func (d *DAG) Compile() (*SOP, error)

// SOP 编译后的标准操作流程
type SOP struct {
    dag    *DAG
    sorted []string // 拓扑排序后的步骤 ID
}

// Execute 执行 SOP
func (s *SOP) Execute(ctx context.Context, input map[string]any) (map[string]any, error)
```

#### 9.1.3 工作量估算：~10 天

### 9.2 Provider Plugin Hooks

#### 9.2.1 目标
在现有 Router + CircuitBreaker 基础上，增加 pre/post processing hooks + quota 管理。

#### 9.2.2 Go 实现方案

```go
// model/hooks.go

// ProviderHook 模型提供者的钩子
type ProviderHook struct {
    Name       string
    Before     func(ctx context.Context, req *HookRequest) (*HookResponse, error)
    After      func(ctx context.Context, resp *HookResponse) error
    OnFailover func(ctx context.Context, fromModel, toModel string, err error) error
}

// HookRequest 请求级上下文
type HookRequest struct {
    ModelName string
    Messages  []*message.Msg
    Options   []ChatOption
}

// HookResponse 响应级上下文
type HookResponse struct {
    ModelName string
    Response  *message.Msg
    Err       error
}

// QuotaTracker 配额追踪
type QuotaTracker struct {
    quotas map[string]*Quota // model → quota
    mu     sync.RWMutex
}

type Quota struct {
    ModelName        string
    MaxTokensPerMin  int
    MaxRequestsPerMin int
}

// WithHooks 为 Router 添加钩子
func WithHooks(hooks ...*ProviderHook) RouterOption
func WithQuotaTracker(tracker *QuotaTracker) RouterOption
```

#### 9.2.3 工作量估算：~5 天

### 9.3 记忆补齐 — MMR + Time Decay + Reranker

#### 9.3.1 目标
在 ReMe 成熟基础上，补齐 MMR (Maximal Marginal Relevance) + Time Decay + Reranker。

#### 9.3.2 Go 实现方案

```go
// memory/mmr.go

// MMRSelector 最大边际相关性选择器
type MMRSelector struct {
    lambda   float64 // 0-1, 多样性权重 (0=最大相关, 1=最大多样)
    embedder embedding.Provider
}

// Select 从候选记忆中选择 MMR 子集
func (m *MMRSelector) Select(ctx context.Context, query []float64, candidates []MemoryEntry, topK int) ([]MemoryEntry, error)

// memory/time_decay.go

// TimeDecayScorer 时间衰减评分器
type TimeDecayScorer struct {
    halfLife time.Duration // 半衰期
    decayFn  func(age time.Duration) float64
}

// Score 计算记忆的时间衰减分数
func (t *TimeDecayScorer) Score(createdAt time.Time) float64

// memory/reranker.go

// Reranker 重排序接口
type Reranker interface {
    Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error)
}

// CrossEncoderReranker 基于 Cross-Encoder 的重排序 (可选, 需要外部 API)
type CrossEncoderReranker struct {
    model model.ChatModel // 复用现有模型接口
}
```

#### 9.3.3 工作量估算：~8 天

---

## 十、阶段七：Channel 合约 (P3) + Identity 合并 (P3)

### 10.1 Channel 合约系统

```go
// gateway/channel.go

// ChannelHealth 通道健康状态 (对齐 OpenSquilla GREEN/YELLOW/RED)
type ChannelHealth int
const (
    ChannelGreen  ChannelHealth = iota // 正常
    ChannelYellow                       // 降级 (限流/延迟高)
    ChannelRed                         // 不可用
)

// ChannelCapabilityProfile 通道能力描述
type ChannelCapabilityProfile struct {
    ChannelType      string         // "agui" / "a2a" / "cli" / "api"
    SupportsTools    bool
    SupportsStreaming bool
    SupportsVision   bool
    MaxTokenRate     int            // tokens/second
    Health           ChannelHealth
}

// ChannelAdapter 通道适配器 (Contract Pattern)
type ChannelAdapter interface {
    Profile() *ChannelCapabilityProfile
    AdaptInput(ctx context.Context, msg *message.Msg) (*message.Msg, error)
    AdaptOutput(ctx context.Context, msg *message.Msg) (*message.Msg, error)
}
```

### 10.2 Identity 三文档合并

```go
// agent/persona.go

// Persona Agent 的人格对象 (对齐 SOUL.md + IDENTITY.md + AGENTS.md)
type Persona struct {
    Soul     string            // 核心人格 (不可变)
    Identity string            // 身份描述 (可随对话演化)
    Agents   string            // Agent 间协作规范
    Vars     map[string]string // 模板变量
}

// Render 合并三文档为系统提示 (对齐 Jinja2 rendering, 用 Go text/template)
func (p *Persona) Render() (string, error)
```

### 10.3 工作量估算：Channel ~5 天, Identity ~3 天

---

## 十一、实施时间线

```
2026-Q3 (Phase 1) ───────────────────────────────────────────────
  Week 1-2:  智能模型路由 (SquillaRouter) ─── 12 天
  Week 3-5:  Turn 状态机 (TurnRunner) ──────── 15 天
  ────────────────────────────────────────────
  里程碑 M1: ReActAgent 每轮推理自动路由 + 7 阶段可观测

2026-Q4 (Phase 2) ───────────────────────────────────────────────
  Week 1-3:  安全纵深防御 (5层) ─────────────── 15 天
  Week 4-5:  工具 Token 压缩 (TokenJuice) ───── 8 天
  Week 6:    决策日志与可观测 ────────────────── 6 天
  ────────────────────────────────────────────
  里程碑 M2: 安全纵深 + 成本优化 + 决策审计

2027-Q1 (Phase 3) ───────────────────────────────────────────────
  Week 1-2:  Skills DAG 编排 ────────────────── 10 天
  Week 3:    Provider Plugin Hooks ───────────── 5 天
  Week 4-5:  记忆补齐 (MMR + TimeDecay + Reranker) ─ 8 天
  ────────────────────────────────────────────
  里程碑 M3: DAG 编排 + 插件生态 + 记忆增强

2027-Q2 (Phase 4) ───────────────────────────────────────────────
  Week 1-2:  Channel 合约系统 ────────────────── 5 天
  Week 3:    Identity 三文档合并 ─────────────── 3 天
  ────────────────────────────────────────────
  里程碑 M4: 多通道适配 + 人格系统
```

---

## 十二、关键设计原则

1. **零 CGO 依赖不变**：SquillaRouter 不用 ONNX/LightGBM，改用 Go 原生分类方案 (关键词 + embedding 余弦)
2. **Builder 模式一致**：所有新组件通过 Builder 配置注入
3. **接口优先**：DecisionSink、AuditSink、Reranker 等均为接口，可替换实现
4. **渐进式集成**：SquillaRouter/TurnRunner 均为可选组件，不配置则降级到现有行为
5. **事件驱动对齐**：所有新阶段 emit 事件到现有 event.Bus
6. **单二进制部署**：不引入外部进程依赖 (ONNX Runtime 等)
7. **TurnRunner 不重写**：作为 replyInternal() 的内部引擎注入，保持外部接口不变
8. **安全防御作为 TurnHook**：注入 TurnRunner 的 PhaseReceive/PhaseObserve 阶段
9. **复用现有模块**：CircuitBreaker、Workspace、Permission Engine、event.Bus 等全部复用

---

## 十三、收益量化

| 维度 | 演进前 | 演进后 | 量化收益 |
|------|--------|--------|---------|
| 模型路由 | 静态主备 | 4 层智能路由 | 89% 成本降低潜力 (PinchBench 参考) |
| Turn 结构 | 340 行 for 循环 | 7 阶段状态机 | 每阶段可 hook/suspend/observe |
| 安全防御 | 单层权限引擎 | 5 层纵深防御 | 注入防护 + 输出过滤 + 审计日志 |
| 工具效率 | 全量工具描述 | 动态子集 + 描述压缩 | Token 消耗降低 30-50% (估算) |
| 可观测性 | OTel + LangSmith | + 结构化决策审计 + 成本节省统计 | 完整决策回溯能力 |
| 技能编排 | 扁平 SkillBox | DAG 编排 + 条件分支 + IO Schema | 复杂工作流支持 |
| 记忆能力 | ReMe (极成熟) | + MMR 多样性 + 时间衰减 + Reranker | 检索质量提升 15-25% (估算) |

**总工作量**：~96 天，分 4 个实施阶段

**Go 版演进后仍保持的核心优势**：零 CGO、单二进制、ReMe 记忆、A2A 协议、事件驱动 V2、GEP 自演化。

---

## 附录 A：OpenSquilla 关键架构特征（跨领域）

1. **Protocol-based DI**：所有核心组件 (Memory, Provider, Tool, Channel) 通过 Protocol 接口注入，非具体实现依赖
2. **显式状态机**：AgentState enum 驱动，非隐式 if-else 分支
3. **流式生成器架构**：async generator 作为核心输出模式
4. **多层安全防线**：失败闭合 (fail-closed) 设计，默认拒绝
5. **节省优先设计**：SquillaRouter + Tokenjuice + PromptPolicy 三位一体
6. **DAG 编排系统**：MetaOrchestrator + 6 step kinds + SOP compilation
7. **不可变数据传递**：跨阶段传 TurnState 而非共享可变状态
8. **版本化持久化**：SchemaVer 字段确保向后兼容

## 附录 B：agentscope.go 已有优势（本方案保留并发扬）

1. **ReMe 记忆系统**：5 实现 + 7 向量后端 + BM25 + 知识图谱 + Dream + ReMe — Go 生态最成熟
2. **事件驱动 V2**：20+ 事件类型 + Bus + AG-UI 映射 + HITL 挂起恢复 — 对齐 Python v2 范式
3. **A2A 协议**：完整实现 + 分布式注册 + 分片路由 + Watch 故障转移 — Python v2 无对等实现
4. **Gateway 生产级**：HTTP/SSE/WS + AG-UI + Tool Offload + 多租户 — 接近 Python create_app 体验
5. **GEP 自演化**：Gene/Capsule + Run/Reflect/Solidify — 创新特性
6. **零 CGO 单二进制**：纯 Go SQLite/ONNX HTTP 代理 — 部署优势
7. **Builder 一致性**：所有组件 Builder 模式统一 — 开发体验
8. **测试文化强**：`-race` 全绿 + 261 测试文件 — 质量保障
