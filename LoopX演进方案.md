# LoopX 演进方案：为 AgentScope.Go 引入长时序 Agent 治理控制平面

> 研究对象：[`huangruiteng/loopx`](https://github.com/huangruiteng/loopx) v0.4.x（Python，MIT）
> 本仓库：`agentscope.go` v2.5.0（Go，Apache-2.0）
> 编写日期：2026-08-09
> 文档定位：**深度研究 + 差距分析 + 详尽演进路线图**。本文不实现代码，只产出可执行的设计与分期计划。

---

## 0. TL;DR（执行摘要）

一句话结论：**两个项目不是竞品，而是互补的两层。**

| 维度 | LoopX | AgentScope.Go |
|------|-------|---------------|
| 定位 | **控制平面 / 状态内核**（govern long-running work） | **Agent 运行时 / 框架**（execute work） |
| 核心问题 | “长时序工作如何可复盘、可重启、可交接” | “如何用 Go 干净地构建一个会干活的 Agent” |
| 是否执行工作 | **否**。Codex/Claude Code/Cursor 执行，LoopX 只管状态 | **是**。ReAct 循环、工具调用、记忆都在这里 |
| 语言/规模 | Python，stdlib 零依赖 CLI | Go，~66,500 行非测试 / 162 包 |
| 心智模型 | Agent-native Kanban（卡片=身份+权威+证据+续接） | 事件驱动 ReAct + 多 Agent 编排 + Gateway |

LoopX 领先的本质，是它回答了 AgentScope.Go **从未回答**的两个问题：

1. **“这个回合到底该不该跑？”** —— 配额/交互契约（`should-run` → `deliver / ask / wait / self-repair / quiet`）。AgentScope.Go 的 `schedule/` 是纯 cron、`middleware/budget.go` 是回合内 token 软提示，**没有任何东西**决定“这一轮干脆别跑”。
2. **“跨越成百上千个回合、跨天跨周，权威的‘目标’是什么？”** —— 持久化生命周期目标（lifetime objective）作为唯一真相源。AgentScope.go 的 `plan/RichPlan`、`agentic_memory` 的 `MEMORY.md` 都是临时草稿，不是治理真相源。

这两点构成 LoopX 控制平面的**脊柱**。其余 12 项能力（门、证据、租约、投影……）大多依附其上。

**本方案的核心动作**：在 AgentScope.Go 新增一个 `controlplane/` 顶层包，吸收 LoopX 的脊柱（Goal 内核 + Quota 决策 + Interaction Contract + User Gate），并**最大限度复用**已有的 `evolver/`、`messagebus/`、`permission/`、`schedule/`、`service/`、`gateway/projection.go`，避免重造。

---

## 1. 两个项目的定位与方法论差异

### 1.1 LoopX 的核心方法论（来自 `docs/architecture.md` + `state-interaction-model.md`）

**Lifetime Goal Invariant（生命周期目标不变量）**：一个目标是一个持久的意图，可能比任何线程/执行器/计划都活得长。它必须稳定到“未来的 Agent 能恢复（目标是什么、谁来改、下一个安全转移是什么）”，又必须窄到“自动化只做一次**有界的、可验证的**移动”。

**四种运行时责任（Runtime Responsibility Model）**——请求流和结果流方向相反：

```
Agent -> Capability -> Provider -> 外部系统
外部观测/副作用回读 -> Provider -> Capability
类型化转移提案 -> Kernel -> 下一个 todo / gate / monitor / turn
```

关键纪律：**观测 ≠ 转移；Provider 收据 ≠ 已接受进度**。只有 Capability 校验通过、Kernel 提交后，状态才改变。这就是“validated writeback（经校验的写回）”。

**Tick（核心节拍）**：

```
quota should-run   → 这一回合该不该跑？（只读，不改状态）
todo claim         → 谁拥有这一片？（软 claim）
todo update        → 发生了什么？
refresh-state      → 下一回合该看到什么？
quota spend-slot   → 为一个已完成、已校验的片记账（仅在校验写回之后）
```

### 1.2 AgentScope.go 的核心方法论（来自 `AGENTS.md`）

事件驱动 ReAct + 洋葱模型中间件（7 钩子）+ channel 背压 + struct embedding 复用 + Formatter 解耦 + Workspace 沙箱。强项是**执行**：10+ 模型后端、ReMe 世界级记忆、a2a 领先协议、gateway MCP 桥接、evolver GEP 自演化。

**方法论差异的根本点**：AgentScope.go 的“回合”是**一次用户请求触发的 ReAct 循环**；LoopX 的“回合”是**一次心跳唤醒后、配额许可下的有界执行片**。前者是“即时对话”，后者是“长时序任务治理”。这是两套完全不同的时间尺度。

---

## 2. LoopX 相对 AgentScope.go 的领先之处（能力差距矩阵）

下表是逐概念比对结论（证据见 `path:line`）。判定标准：

- **ABSENT（缺失）**：grep 全仓库 0 业务命中，无任何等价物。
- **PARTIAL（部分）**：机制存在，但缺 LoopX 的核心语义。
- **PRESENT（已具备）**：已有等价实现，可直接复用。

| # | LoopX 概念 | 判定 | AgentScope.go 现状（证据） | 差距 |
|---|-----------|------|--------------------------|------|
| 1 | **持久化生命周期目标（真相源）** | **ABSENT** | `objective\|lifetime\|north_star` grep 0 命中；`plan/enhanced.go:44` `RichPlan` 是单 notebook 草稿；`agentic_memory` 的 `MEMORY.md` 是非结构化自由文本 | 缺一个跨千回合/跨天周的**治理级**目标对象 |
| 2 | **带 peer claim/lease 的类型化 todo** | **PARTIAL** | 三套 todo 模型（`plan/plan.go:26`、`plan/enhanced.go:33`、`state/task.go:20`）均**无 claim/lease**；claim 仅在 `evolver/types.go:126` ATP hub 路径；lease 原语 `messagebus/coord.go:18` `Lock(ttl)` 存在但**未接到任何 task** | todo 无所有权协议；lease 已有却闲置 |
| 3 | **具体 user gate（非“等 owner”）** | **PARTIAL** | 真实 HITL 悬挂/恢复存在：`permission/engine.go:147` ASK、`event/hitl_events.go:6` `RequireUserConfirmEvent`、`gateway/resume_handler.go:50` | gate 只拦**工具调用确认**，无“拦整条 lane 直到人回答一个具体问题”的类型 |
| 4 | **证据 + 经校验写回** | **PARTIAL** | 仅 evolver 有：`evolver/types.go:107` `Outcome`、`:139` `ValidationCommands`、`client.go:86` `SolidifyRequest{DecisionSource,PrimaryCause,...}`；`plan/plan.go:107` `UpdateStep` 写自由文本 result，**零校验零证据** | todo/plan 完成无证据、无校验、无决策谱系 |
| 5 | **配额/交互契约（should-run / deliver-ask-wait-self-repair-quiet）** | **ABSENT** | `scheduler_hint\|should_run\|spend-slot\|self-repair` grep **0 命中**；`schedule/scheduler.go:13` 纯 cron 无条件触发；`middleware/budget.go:48` 是**回合内** token 软提示（on_reasoning 注 hint + 强制 tool_choice=none），不决定“回合跑不跑” | 无“这一轮干脆别跑/自愈/静默”的决策层 |
| 6 | **不绕过门的安全 fallback** | **PARTIAL** | `permission/engine.go:297` `checkSafety` 绕过免疫；`:403` `ModeDontAsk` 用户不在时**拒绝**而非放行；`evolver` `GeneConstraints/ToolPolicy/BlastRadius` | fallback 分散在各机制，无“门答不上时的统一 fallback 策略” |
| 7 | **紧凑 run history + 决策谱系** | **PARTIAL** | 碎片化：`plan/enhanced.go:60` 存计划快照不存“为何”；`gateway/background_task.go:212` 只存单槽 LastRun/LastError；`gateway/audit.go:32` 是 HTTP 访问审计非决策谱系；决策谱系**仅 evolver**（`SolidifyRequest`） | 无统一谱系；agent 决策无血脉 |
| 8 | **类型化 continuation / 跨回合交接** | **PARTIAL** | `agent/agent_state.go:10` `AgentState` 可序列化快照（消息+悬挂工具态）跨请求/崩溃恢复；`gateway/wakeup_dispatcher.go:136` 靠 inbox 消息重组 `<team-message>` 重跑 | 续接 = “重载消息日志”，无承载**目标/todo 态/证据**的类型化交接对象 |
| 9 | **配额驱动的心跳自动化** | **ABSENT** | `wakeup_dispatcher.go` 是**事件驱动**（inbox wakeup），非配额驱动；`messagebus/keys.go:13` `WakeupKind{wake,resume}` 都非配额；`schedule/` 心跳=cron 时间非预算 | 无“配额值触发/抑制心跳”的代码路径 |
| 10 | **运营者只读 review packet / 看板** | **PARTIAL** | `gateway/audit.go`（HTTP 审计）、`examples/web_ui`（Chat/KB/System 三视图）、`gateway/projection.go:54`（跨会话卡片） | 无“汇总目标+开放 todo+待决门+近期证据+决策谱系”的运营者视图 |
| 11 | **外部投影（Kanban）只读视图** | **PARTIAL** | `gateway/projection.go:11` `SessionProjection`（key→blob 卡片，背后 `CoordBus.Registry*`），**今天只用于 HITL 卡片** | 通用卡片注册表，无 todo/目标/证据的 Kanban 投影，无 row lineage（`row_lifecycle/supersedes`） |
| 12 | **领域能力打包（issue-fix 等）** | **PARTIAL** | `evolver/types.go:21` `Gene{Category,SignalsMatch,Strategy,Validation,...}` + `skill/` 包；类别 `repair/optimize/innovate/explore` | 是**通用演化策略**按信号匹配选择，非 issue-fix 这种**命名领域 playbook 库** |
| 13 | **奖励记忆 / 上下文学习** | **PRESENT** | `evolver` `Capsule/Gene` + `Remember/Recall`；`middleware/reflection_memory.go:42` 原子事实提取；`agentic_memory.go`；`LongTermMemory` 三模式 + ReMe | **唯一全命中项**。差距仅在 LoopX 的 5 类权威分离（`run_bound_reward/hard_policy/soft_preference/procedural_experience/working_context`），evolver 未做此区分 |
| 14 | **公私边界纪律** | **PARTIAL** | `evolver/types.go:94` `Capsule.Visibility`（自由字段无 `public/private` 常量无强制）；`service/access/policy.go` 是**跨用户**资源共享纪律 | 无 **agent 内部**“可观测 vs 保密”的纪律性分区 |

**净结论**：

- 唯一 **PRESENT**：奖励记忆（#13）—— 这是 AgentScope.go 与 LoopX 最强重合点，复用 `evolver`。
- **ABSENT 的两根脊柱**：#1 持久化生命周期目标、#5+#9 配额/交互契约。这两点是 LoopX 控制平面的脊椎，本仓库无任何等价物。
- 其余 11 项 **PARTIAL**：机制碎片化存在，缺 LoopX 的核心语义（claim/lease 闲置、gate 只拦工具、证据仅 evolver 有、continuation 无目标对象……）。

**附加发现（peer 模型）**：AgentScope.go 的 Agent Team（`service/entities.go:150` `Team.LeaderSessionID`、`gateway/team_tools.go:107`）是**固定 leader 的异步消息协作**，leader 在创建时写死、无租约、无重选、无 failover；协调靠 inbox push + wakeup 信号。这与 LoopX 的 **peer_v1（无持久 leader，靠 claim/lease/continuation policy/确定性哈希选临时协调者）**是不同模型。通用 `messagebus.CoordBus.Lock` 租约原语已存在却**未被 team 模型使用**。

---

## 3. 演进目标与原则

### 3.1 目标

把 LoopX 的**控制平面脊柱**以**地道 Go 方式**引入 AgentScope.go，使其具备“长时序 Agent 治理”能力，同时**不破坏**现有运行时框架。

具体可度量的里程碑：

- **M1**：一个跨重启仍存活、作为决策真相源的 `Goal` 对象。
- **M2**：一次心跳能被 `ShouldRun` 决策**拒绝**（“这一轮别跑，因为门未答/预算耗尽/无实质变化”）。
- **M3**：一条 todo 的完成必须携带**经校验的证据**，否则不接受、不记账。
- **M4**：运营者能从一个只读 endpoint 看到“当前目标 + 开放 todo + 待决门 + 近期证据 + 决策谱系”。
- **M5**：多 Agent 能靠 claim/lease 抢同一 goal 的不同 todo，无持久 leader。

### 3.2 原则（地道 + 最小 + 复用）

1. **补层，不替换**。新增 `controlplane/` 顶层包，**不动** `agent/`、`gateway/`、`tool/`、`memory/` 的核心抽象。运行时框架照旧；控制平面是它之上的一层治理。
2. **先复用，再新建**。租约用 `messagebus.CoordBus.Lock`；证据谱系抄 `evolver.SolidifyRequest` 的字段设计；门扩展 `event/hitl_events.go`；投影扩 `gateway/projection.go`；持久化走 `service.Storage`（SQLStorage 加表）。
3. **地道 Go**。状态转移用枚举 + 显式 `Transition()` helper（LoopX 的 `contract.py` 思路），不让非法状态可表达（AGENTS.md“Make illegal states hard to express”）。channel 替代 Python 的 dispatcher。
4. **LLM 不进热路径**。对齐 LoopX `interaction_contract.py:41` `PROTOCOL_ACTION_PACKET_LLM_POLICY = "no_api"`——`ShouldRun` 决策是纯状态机，不调模型。
5. **默认关闭、可选启用**。对齐 LoopX 的 default-off 哲学（reward memory、explore 都是 opt-in）。控制平面通过 `gateway.AppConfig` 一个开关接入。
6. **ponytail：先脊柱，后血肉**。P0 只交付“Goal + ShouldRun + 经校验写回”三件最小可用品；门/投影/peer/能力库全部往后排。每个 ponytail: 标记的简化点都写明天花板与升级路径。

---

## 4. 分期演进路线图

四期，每期是一个可独立验收、可回滚的阶段包。P0 是脊柱，P1 是治理纪律，P2 是协作与可视化，P3 是高级能力。

```
P0 控制平面脊柱（~3-4 周）
  └─ Goal 内核 + Quota.ShouldRun + 经校验写回 + Interaction Contract 枚举
P1 治理纪律（~3 周）
  └─ 具体化 User Gate（decision_scope）+ 安全 fallback 策略 + run history 谱系
P2 协作与可视化（~3-4 周）
  └─ peer claim/lease（接线 CoordBus.Lock）+ 运营者 review board + Kanban 投影（row lineage）
P3 高级能力（按需，~4+ 周）
  └─ 领域能力库（issue-fix playbook）+ reward memory 5 类权威分离 + 公私边界强制
```

依赖关系：P0 是其余一切的地基。P1 依赖 P0 的 Goal/Quota。P2 的 peer 依赖 P0 的 todo + P1 的 gate。P3 依赖 P2 的投影与协作。

---

## 5. P0 详设：控制平面脊柱

### 5.1 包布局（新建）

```
controlplane/
├── goal/
│   ├── goal.go            # Goal 实体 + 生命周期不变量 + LegalTransitions
│   ├── store.go           # GoalStore 接口（SQL/Redis/内存三后端，复用 service.Storage 模式）
│   └── projection.go      # 只读投影（喂给 review board）
├── todo/
│   ├── todo.go            # Todo 实体 + 4 状态机（open/done/blocked/deferred）
│   ├── claim.go           # Claim（软所有权）—— 复用 messagebus.CoordBus.Lock 做硬租约
│   └── contract.go        # TaskClass 枚举 + DecisionScope
├── quota/
│   ├── should_run.go      # ShouldRun 决策机（纯状态机，no_api）
│   ├── states.go          # ComputeState 枚举（blocked_health/operator_gate/.../eligible/.../paused）
│   ├── schema.go          # Quota{Compute,WindowHours,SlotMinutes,AllowedSlots,SpentSlots,...}
│   └── spend.go           # SpendSlot（仅在校验写回后）
├── evidence/
│   ├── evidence.go        # Evidence 实体 + Validation
│   └── writeback.go       # ValidatedWriteback（抄 evolver.SolidifyRequest 字段）
├── interaction/
│   └── contract.go        # InteractionContract（loopx_interaction_contract_v0 的 Go 版）+ TurnRoute/TurnResultKind 枚举
└── controlplane.go        # Kernel 门面：ShouldRun/Claim/Update/Refresh/Spend 的编排
```

**放置理由**（对齐 AGENTS.md“Capability And Extension Placement”）：这是一个新的顶层 bounded context，不属于 `agent/`（执行）也不属于 `gateway/`（传输）。它有自己的变更原因（治理语义），独立成包便于本地化、测试、回滚。

### 5.2 核心数据模型

#### Goal（生命周期目标，治理真相源）

```go
// controlplane/goal/goal.go
type Goal struct {
    ID            string         // goal_id 边界
    Objective     string         // 持久意图（跨千回合稳定）
    Scope         []string       // 显式边界（non-goals）
    Authority     []AuthoritySrc // 谁有权改（替代隐式模型记忆）
    State         GoalState      // active|paused|completed|abandoned
    CurrentTodoID string         // 下一个安全转移
    Quota         quota.Schema
    CreatedAt, UpdatedAt time.Time
}

type GoalState string
const (
    GoalActive    GoalState = "active"
    GoalPaused    GoalState = "paused"     // 硬暂停：所有自动权限 false
    GoalCompleted GoalState = "completed"
    GoalAbandoned GoalState = "abandoned"
)

// LegalTransitions —— 让非法状态难表达（AGENTS.md 原则）
var goalTransitions = map[GoalState]map[GoalState]bool{
    GoalActive: {GoalPaused: true, GoalCompleted: true, GoalAbandoned: true},
    GoalPaused: {GoalActive: true, GoalAbandoned: true},
    // completed/abandoned 终态，无出边
}
```

**持久化**：新增 `service.Storage` 的一张 `goals` 表（对齐 SQLStorage 现有 8 表模式，`service/sql_storage.go`），原子 upsert + 级联删除（删 goal → 级联 todos/evidence）。Redis 后端用于多进程。`:memory:` 模式零配置测试。

#### Todo（带所有权的类型化待办）

```go
// controlplane/todo/todo.go
type Todo struct {
    ID           string
    GoalID       string
    Description  string
    TaskClass    TaskClass  // advancement_task|continuous_monitor|user_gate|user_action|blocker
    State        TodoState  // open|done|blocked|deferred（终态=done/deferred）
    ClaimedBy    string     // 软所有权（注册 agent_id）
    Lease        *Lease     // 硬租约（可选，复用 CoordBus.Lock）
    DecisionScope DecisionScope  // 见 P1
    Continuation ContinuationPolicy  // independent_handoff|same_agent_non_delivery
    Evidence     []string   // 完成时必须非空（P0 先软约束，P1 强制）
}
```

#### InteractionContract（第一类协议，对齐 `loopx_interaction_contract_v0`）

```go
// controlplane/interaction/contract.go
type Mode string
const (
    ModeBoundedDelivery           Mode = "bounded_delivery"
    ModeUserGate                  Mode = "user_gate"
    ModeScopedUserGateFallback    Mode = "scoped_user_gate_fallback"
    ModeUserTodoBlockerPush       Mode = "user_todo_blocker_push"
    ModeSuccessorReplanRequired   Mode = "successor_replan_required"
    ModeExternalEvidenceObservation Mode = "external_evidence_observation"
    ModeMonitorQuietSkip          Mode = "monitor_quiet_skip"
    ModeAutonomousReplan          Mode = "autonomous_replan"
    ModeOutcomeFloorRecovery      Mode = "outcome_floor_recovery"
    ModeQuotaThrottled            Mode = "quota_throttled"
)

type InteractionContract struct {
    Mode         Mode
    AgentChannel AgentAction   // primary_action
    UserChannel  UserNotify    // NOTIFY|DONT_NOTIFY
    CLIChannel   []string      // next_cli_actions
}
```

#### TurnRoute / TurnResultKind（类型化回合词汇）

直接移植 LoopX 的两个枚举（`architecture.md:70-93`）：

```go
type TurnRoute string // pre-execution
const (
    RouteReadyForHost        TurnRoute = "ready_for_host"
    RouteRepairRequired      TurnRoute = "repair_required"
    RouteReplanRequired      TurnRoute = "replan_required"
    RouteUserActionRequired  TurnRoute = "user_action_required"
    RouteWait                TurnRoute = "wait"
    RouteBlocked             TurnRoute = "blocked"
    RouteContractError       TurnRoute = "contract_error"
)

type TurnResultKind string // post-execution
const (
    ResultValidatedProgress   TurnResultKind = "validated_progress"
    ResultValidatedCompletion TurnResultKind = "validated_completion"
    ResultRepairRequired      TurnResultKind = "repair_required"
    ResultReplanRequired      TurnResultKind = "replan_required"
    ResultUserActionRequired  TurnResultKind = "user_action_required"
    ResultWait                TurnResultKind = "wait"
    ResultHostFailure         TurnResultKind = "host_failure"
    ResultValidationFailed    TurnResultKind = "validation_failed"
    ResultWritebackFailed     TurnResultKind = "writeback_failed"
    ResultQuotaSpendFailed    TurnResultKind = "quota_spend_failed"
)
```

注意 LoopX 把“deliver”拆成 `validated_progress` vs `validated_completion`；“静默”是行为（`monitor_quiet_skip`）非枚举成员。

### 5.3 ShouldRun 决策机（纯状态机，no_api）

这是 P0 最核心、最该抄 LoopX 的部分。**状态优先级顺序**（`docs/quota-allocation.md:117-134`），配额**最后**应用，永远不变成第二套权限系统：

```go
// controlplane/quota/should_run.go
func (k *Kernel) ShouldRun(ctx context.Context, goalID, agentID string, opts ShouldRunOpts) (*Decision, error) {
    // 1. 健康/安全门（registry health、goal 存在、pause 态）
    // 2. 运营者门（user gate 是否阻塞 —— P1 接入，P0 先 stub）
    // 3. 证据等待（awaiting evidence）
    // 4. 焦点等待（focus wait）
    // 5. 计算配额（compute quota —— 唯一能产出 should_run=true 的，除 eligible 外）
    //    paused（compute≤0）= goal 级硬暂停，产出一条终态契约，所有自动权限 false
    // 返回一个 Decision 包：should_run bool + state ComputeState + effective_action + scheduler_hint + interaction_contract
}

type Decision struct {
    ShouldRun        bool
    State            ComputeState
    EffectiveAction  string  // monitor_quiet_skip|external_evidence_observe|...|outcome_floor_recovery|quota_skip
    SchedulerHint    SchedulerHint
    Contract         InteractionContract
}
```

**SchedulerHint**（跨运行时等待策略，非权限）直接移植 `scheduler_hint_v0`：

```go
type SchedulerAction string
const (
    HintRunNow                       SchedulerAction = "run_now"
    HintBackoffWaitingForUser        SchedulerAction = "backoff_waiting_for_user"
    HintBackoffUntilReassigned       SchedulerAction = "backoff_until_reassigned"
    HintBackoffUntilMaterialTransition SchedulerAction = "backoff_until_material_transition"
    HintBackoffUntilFreshEvidence    SchedulerAction = "backoff_until_fresh_evidence"
    HintStopUntilExplicitResume      SchedulerAction = "stop_until_explicit_resume"
)
```

**Spend 纪律**（`quota-allocation.md:1014-1063`）：`SpendSlot` 默认 dry-run，`--execute` 才追加一条 `quota_slot_spent` 事件。**仅在校验写回之后**合法；quiet skip / preflight 失败 / dry-run / 重复记账全部拒绝。`after.spent_slots == before.spent_slots + slots`。记账事件**不是权限**，只是记录“检查放行后确实花了算力”。

### 5.4 经校验写回（抄 evolver.SolidifyRequest）

```go
// controlplane/evidence/writeback.go
type ValidatedWriteback struct {
    TodoID          string
    Outcome         Outcome          // Status + Score（抄 evolver/types.go:107）
    DecisionSource  string           // 决策谱系（抄 evolver/client.go:86 SolidifyRequest）
    PrimaryCause    string
    ContributingFactors []string
    Evidence        []Evidence       // 必须非空 + 已 Validation 通过
    HumanIntervention string
    RunID           string
}
```

**与 evolver 的关系**：evolver 的 `SolidifyRequest` 本质就是 GEP 演化场景的“经校验写回”。P0 把它的字段设计**抽象成通用的 `ValidatedWriteback`**，evolver 未来可改为调用它（去重，AGENTS.md“Distinguish duplicate knowledge from duplicate-looking code”）。

### 5.5 Kernel 门面（tick 编排）

```go
// controlplane/controlplane.go
type Kernel struct {
    goals   goal.Store
    todos   todo.Store
    quota   quota.Store
    ledger  Ledger     // append-only 事件账本（event_sourced_state_contract_v0）
    leases  messagebus.CoordBus  // 复用现有租约原语
}

// 一次完整 tick 的编排（对齐 LoopX 心跳协议 heartbeat-automation-prompt.md:302-511）
func (k *Kernel) Tick(ctx, goalID, agentID, turnInstanceID string) (*TickResult, error) {
    dec, err := k.quota.ShouldRun(ctx, goalID, agentID, ...)  // 1. 只读决策
    if !dec.ShouldRun { return &TickResult{Decision: dec}, nil }
    lease, err := k.claim(ctx, goalID, dec.NextTodoID, agentID) // 2. 软 claim / 硬租约
    // 3. —— 这里不执行工作；AgentScope.go 的 ReAct 运行时执行 ——
    // 4. 运行时回写：Writeback(ctx, ValidatedWriteback{...})
    // 5. 仅当回写校验通过：k.quota.SpendSlot(ctx, goalID, 1, Execute)
}
```

**关键纪律**：控制平面**不执行工作**。ReAct Agent（`agent/react/`）照旧执行；控制平面只在执行前后做治理决策。这通过一个新的 `middleware.ControlPlaneMiddleware`（洋葱模型 on_reply 钩子）接入：on_reply 调 `ShouldRun` 决定是否放行/悬挂/静默。

### 5.6 接入点

`gateway.AppConfig` 新增：

```go
appCfg := gateway.AppConfig{
    ...
    ControlPlane: cpConfig,  // 可选；nil = 控制平面关闭，行为完全同今天
}
```

`ControlPlane` 非 nil 时：

- `Server.Start` 拉起一个 `QuotaHeartbeat`（订阅 `schedule/` 的 cron，但每次触发**先过 `ShouldRun`**，被拒则静默跳过）；
- `BuildSessionAgent` 注入 `ControlPlaneMiddleware`。

### 5.7 P0 验收标准

- [ ] 一个 Goal 跨进程重启仍可恢复（SQL/Redis 持久化 + `:memory:` 测试）。
- [ ] `ShouldRun` 在 `paused` 态返回终态契约 + 所有自动权限 false。
- [ ] `ShouldRun` 在配额耗尽时返回 `quota_throttled` + `backoff_until_material_transition`。
- [ ] `SpendSlot` 在未校验写回时被拒绝（负向测试）。
- [ ] `SpendSlot` 重复记账被拒（幂等）。
- [ ] 控制平面关闭时，全量 `go test ./... -race` 行为不变（零回归）。
- [ ] 一个 smoke：注册 goal → 触发心跳 → ShouldRun=true → 模拟写回 → SpendSlot → spent_slots+1。

**ponytail: P0 不做** peer 抢占、Kanban 投影、领域 playbook、reward memory 5 类分离。这些有明确升级路径（P1-P3）。

---

## 6. P1 详设：治理纪律

### 6.1 具体化 User Gate（decision_scope，非全局布尔）

LoopX 的核心反模式是“含糊地等 owner”。门不是全局布尔，而是**带作用域的决策**（`docs/concepts/interaction-pattern-catalog.md:146-200`）：

```go
// controlplane/todo/contract.go
type DecisionScope struct {
    Kind        ScopeKind        // private_read|write_scope|resource|production|public_claim|direction|other
    Granularity ScopeGranularity // action|lane|goal|project|global
    ScopeKey    string           // 具体键
}
```

**与现有 HITL 的整合**：扩展现有 `event/hitl_events.go:6` `RequireUserConfirmEvent`。今天它只承载 `ToolCallSummary`（确认/拒绝/修改一个工具调用）。P1 增加一个 `RequireUserAnswerEvent`（问一个具体问题，拦整条 lane 直到回答）：

```go
type RequireUserAnswerEvent struct {
    GateID     string
    Question   string          // 具体问题，非“等 owner”
    Scope      DecisionScope   // 门覆盖的作用域
    Fallback   *FallbackPolicy // 见 6.2；nil = 无 fallback，纯阻塞
}
type UserAnswerEvent struct {
    GateID  string
    Outcome DecisionOutcome    // approve|reject|cancel
}
```

**解决矩阵**（直接移植 `interaction-pattern-catalog.md:168-174`）：

| 门↔动作关系 | user channel | agent channel |
|------------|--------------|---------------|
| 门覆盖动作，无 fallback | 提具体 todo | 停止 gated 交付，不 spend |
| 门覆盖动作，有 fallback | 通知 | 执行 fallback，spend 一次 |
| 门不覆盖动作 | 保持可见 | 正常执行 |
| 作用域含糊 | 提问/修投影 | **永不从散文推断** |

`TaskClass` 驱动：`user_gate`（阻塞运营者决策）vs `user_action`（非阻塞可见提醒）。`continuous_monitor` + `user_action` 显式非门控。

### 6.2 安全 fallback 策略（不绕过门）

P0/P1 现状：fallback 分散在 `permission/engine.go:297` 和 evolver 约束。P1 抽一个统一 `FallbackPolicy`，**仅当门答不上时激活**，且**永不绕过门**：

```go
type FallbackPolicy struct {
    Scope       DecisionScope   // 必须与门同作用域
    Action      string          // 有界安全动作
    Audit       bool            // 必须审计
    SpendOnce   bool            // 执行后 spend 一次
}
```

纪律：fallback 执行后必须写回证据 + spend 一次；门本身保持开放。这复用 `permission/engine.go:403` `ModeDontAsk` 的“拒绝而非放行”语义。

### 6.3 run history 决策谱系

把 evolver 的 `SolidifyRequest` 谱系模式推广到所有 todo 完成。`gateway/audit.go` 的 HTTP 访问审计保留（它是**访问**审计，不是**决策**审计，两者不混）。新增 `controlplane/ledger/`：

- append-only 事件账本（对齐 `event_sourced_state_contract_v0`）；
- 事件类：`work`、`decision`、`accounting`（`quota_slot_spent`）、`evidence`；
- 确定性重放 + 幂等追加 + 隐私分区；
- 紧凑 run index（喂给运营者视图）+ 私有 run payload 分离。

复用 `gateway/session_manager.go:30` 的 completed 事件缓冲思路，但持久化、跨会话、跨进程（Redis LIST 游标日志，对齐 `messagebus/coord_redis.go` 的 LIST 模式）。

### 6.4 P1 验收

- [ ] 一个 `user_gate` todo 阻塞交付直到 `UserAnswerEvent`。
- [ ] 门未答时，agent channel 停止且**不 spend**（负向测试）。
- [ ] fallback 执行后 spend 恰好一次，门仍开放。
- [ ] 作用域含糊时拒绝从散文推断（负向测试）。
- [ ] `ledger` 跨进程重放一致性测试。

---

## 7. P2 详设：协作与可视化

### 7.1 peer claim/lease（接线 CoordBus.Lock）

**现状**（来自比对）：通用 `messagebus.CoordBus.Lock(key, ttl)` 已存在（Redis `SET NX PX` + Lua token 释放），但**未被 team 模型使用**。team 是固定 leader 的异步消息协作。

P2 做两件事：

1. **接线**：`controlplane/todo/claim.go` 的硬租约直接调 `CoordBus.Lock(ctx, key=goalID:todoID, ttl=45*time.Minute)`。 contention 单位是**单个 todo**，不是整个 goal——不同 todo 可并行（对齐 LoopX `architecture.md:284-298`）。
2. **可选 peer 模型**：给 `service.Team` 增加一个 `Mode = "leader" | "peer"`。peer 模式下：

   - 无持久 leader；注册 agent 等权（LoopX `peer_agent_identity_v1`）；
   - 工作所有权优先级：`claimed_by`/活动租约 > 未领必须先领 > agent 作用域 replan 留在该 agent > 未作用域 replan **按规范 work key 在排序注册 agent 集上哈希，确定性指派给恰好一个 peer**（非持久化为 rank）；
   - 临时协调者：bounded 多 agent 编排时，按任务包哈希选临时协调者，作用域仅限该包的写回，无隐式 review/merge/publish 权限。

**租约操作**（对齐 `cli_commands/task_lease.py`）：`acquire / renew / transfer / release / inspect`，TTL 默认 45min、上限 24h，幂等键 + CAS 冲突响应。

**复用点**：硬租约 = `CoordBus.Lock`；确定性指派 = 一个纯函数 `hash(workKey, sortedAgents) -> agentID`；身份 = 扩 `service.AgentConfig` 加 `AgentScope []string`（自然语言作用域）。

### 7.2 运营者 review board（只读汇总）

新增 `GET /api/v1/controlplane/goals/{id}/review-packet`：

```json
{
  "objective": "...",
  "scope": [...],
  "state": "active",
  "current_todo": {...},
  "open_todos": [...],
  "pending_gates": [{"gate_id":"...","question":"...","scope":{...}}],
  "recent_evidence": [...],
  "decision_lineage": [...],
  "quota": {"spent_slots":N,"allowed_slots":M,...}
}
```

复用现有 `examples/web_ui` 加一个 `ControlPlane` 视图（零构建 vanilla JS + SSE，对齐 AGENTS.md note 38）。**纪律**：board 是只读投影，永远不成为真相源（对齐 LoopX“Agent-Native Kanban Is A Projection”）。

### 7.3 Kanban 投影 + row lineage

扩展 `gateway/projection.go:11` `SessionProjection`（今天只用于 HITL 卡片）成通用 `GoalProjection`：把 todo/目标/证据状态投影成卡片。**关键：row lineage 作为数据**（对齐 LoopX `AGENTS.md` Projection Sink Design）：

```go
type ProjectionRow struct {
    RowLifecycle       string  // current|superseded|migrated|retired
    Supersedes         string  // 前驱 row_id
    SupersededBy       string  // 后继 row_id
    SourceID           string
    MigrationAudit     string  // 紧凑审计证据
}
```

复用 `extensions/lark/presentation/projection_rows.py:132` 的思路（Go 重写）。Lark/飞书适配器可走现有 `channel/feishu`（已存在 `feishu_send_message`）。

### 7.4 P2 验收

- [ ] 两个 worker 同时抢同一 goal 的不同 todo，互不阻塞（不同 todo 并行）。
- [ ] 抢同一 todo 时，CAS 冲突响应正确；TTL 到期自动释放。
- [ ] peer 模式下，未作用域 replan 确定性指派给同一 peer（哈希稳定性测试）。
- [ ] review-packet endpoint 返回完整汇总；web_ui 新视图可读。
- [ ] 投影 row 被 supersede 时，lineage 字段正确链式。

---

## 8. P3 详设：高级能力（按需）

### 8.1 领域能力库（issue-fix playbook）

LoopX 的 `BUILTIN_CAPABILITIES`（`capabilities/catalog.py:15`，12 项）是**命名的领域 playbook**，区别于 evolver 的通用策略 Gene。P3 建一个 `controlplane/capabilities/` 注册表：

```go
type Capability struct {
    ID            string         // "issue-fix"
    Title         string
    Status        string         // stable|experimental|compatibility-facade
    UserValue     string         // 调用方结果（非交付机制）
    ProviderID    string         // loopx-core 或扩展 manifest id
    Commands      []Command      // 每个带 purpose + write_boundary
    Protocols     []Protocol     // schema_version + 实现
    Smokes        []string
    Boundaries    []string
}
```

命名纪律（对齐 AGENTS.md）：按**调用方结果**命名，不按交付机制——`connector/provider/sink` 通常该是扩展 provider 而非能力。能力 vs 扩展是**两个正交轴**共享一个注册表：能力=能做什么（产品契约）；扩展=带独立安装/启用/升级生命周期的交付单元。

**与 evolver 的关系**：evolver Gene 是**策略级**演化资产；领域 capability 是**任务级** playbook。两者可叠加：一个 issue-fix capability 内部可调用 evolver 的 repair Gene。

**起步**：只移植 `issue-fix` 一个旗舰能力（feasibility→patch→checks→review→merge lane），证明模式可行，其余按需。

### 8.2 reward memory 5 类权威分离

#13 是唯一 PRESENT 项，但 evolver 未做 LoopX 的 5 类分离（`reward-memory-architecture-v0.md:147-165`）：

| 类 | 权威 | 生命周期 |
|----|------|---------|
| `run_bound_reward` | 仅单一结果证据 | append-only overlay |
| `hard_policy` | 已验证作用域内约束/否决 | 可取代/撤销/过期 |
| `soft_preference` | 仅建议排序/改写 | 可编辑/拒绝/退役 |
| `procedural_experience` | 当前工件验证后的诊断/路由建议 | 轨迹只增；经验可取代 |
| `working_context` | 仅当前会话 | 绑定会话/修订 |

P3 给 evolver 的 `Capsule/Gene` 加 `AuthorityClass` 字段 + 守护优先级（对齐 `reward-memory-architecture-v0.md:194-217`）。核心纪律：**置信度永不扩大作用域**；策略内容 vs 作用域权威是两个独立问题。

### 8.3 公私边界强制

`evolver/types.go:94` `Capsule.Visibility` 今天是自由字段无强制。P3：

- 引入 `public`/`private` 常量 + 投影时的 redaction；
- agent 内部“可观测 vs 保密”分区（区别于 `service/access/policy.go` 的**跨用户**纪律）；
- 对齐 LoopX AGENTS.md“Public And Private Boundary”：提交前扫描候选路径的内部/私有措辞与忽略本地工件引用。

---

## 9. 与现有模块的整合点（复用清单）

| 现有模块 | 复用方式 | 改动量 |
|---------|---------|--------|
| `messagebus/coord.go` `CoordBus.Lock(ttl)` | 直接作为 todo 硬租约后端 | 零（已实现 SET NX PX + Lua 释放） |
| `evolver/types.go` `Outcome` + `client.go` `SolidifyRequest` | 字段设计抽象成通用 `ValidatedWriteback` | 抽接口，evolver 改调用（去重） |
| `event/hitl_events.go` `RequireUserConfirmEvent` | 扩展为 `RequireUserAnswerEvent`（问问题拦 lane） | 新增类型，不改旧的 |
| `permission/engine.go` `Decision{allow,deny,ask,passthrough}` | User gate 的 outcome 复用决策模型 | 零 |
| `gateway/projection.go` `SessionProjection` | 扩成 `GoalProjection` + row lineage | 加类型，HITL 卡片不变 |
| `service/Storage` / `sql_storage.go` | 加 `goals` 表（+ 级联 todos/evidence/ledger） | 加表 + CRUD，复用 `scanRows[T]` |
| `schedule/scheduler.go` | 心跳触发源，但每次先过 `ShouldRun` | 加一个 `QuotaHeartbeat` 包装层 |
| `gateway/AppConfig` | 加 `ControlPlane` 可选字段 | 一字段 + 条件装配 |
| `middleware/` 洋葱模型 | 新 `ControlPlaneMiddleware`（on_reply 钩子调 ShouldRun） | 一个新中间件 |
| `examples/web_ui` | 加 ControlPlane 视图 | 一个新 view |

**核心纪律**：上表每一行都是“复用/扩展”，不是“重写”。AGENTS.md“Distinguish duplicate knowledge from duplicate-looking code”——evolver 的 `SolidifyRequest` 和未来的 `ValidatedWriteback` 是**重复知识**（决策谱系写回），必须折叠；而非各搞一套。

---

## 10. 关键设计决策与权衡

1. **为什么新建 `controlplane/` 而非塞进 `plan/`？**
   `plan/RichPlan` 是 per-notebook 草稿，变更原因是“计划编辑”；Goal 的变更原因是“治理语义”。两者会为不同理由演化，合并会产生参数爆炸的抽象（AGENTS.md 反模式）。新 bounded context 更易本地化、测试、回滚。

2. **为什么 ShouldRun 是纯状态机（no_api），不调 LLM？**
   对齐 LoopX `interaction_contract.py:41` `PROTOCOL_ACTION_PACKET_LLM_POLICY = "no_api"`。决策在热路径，调模型会引入延迟、成本、不确定性；治理决策必须是确定可重放的。LLM 留给运行时框架的 ReAct。

3. **为什么配额最后应用，不当第二套权限？**
   LoopX `quota-allocation.md:117-134` 明确：状态优先级是 健康/安全门 → 运营者门 → 证据等待 → 焦点等待 → 配额。配额插队会绕过门，破坏“门不可被预算绕过”的不变量。

4. **为什么默认关闭？**
   控制平面是长时序场景才需要的。即时对话场景开它会增加开销而无收益。对齐 LoopX 的 default-off 哲学 + AGENTS.md“ship right-sized, reversible batches”。`AppConfig.ControlPlane=nil` 时行为完全同今天。

5. **为什么 P0 不做 peer？**
   peer 抢占依赖 todo + claim/lease + continuation policy，而 claim/lease 又依赖 P0 的 todo 模型先稳定。先有脊柱再长肢体（ponytail：先脊柱后血肉）。

6. **为什么抄 evolver 的 SolidifyRequest 而非新设计？**
   它已经是“经校验写回 + 决策谱系”的成熟字段集，只是被 GEP 场景专用。抽象成通用 `ValidatedWriteback` 是去重，不是新造。

7. **确定性哈希选 peer vs 围租约选举？**
   选哈希：无状态、可重放、不需共识协议、注册顺序不影响结果（LoopX `peer-agent-runtime-v1.md:31-40`）。围租约选举引入分布式一致性问题，过度工程。哈希的代价是“无法偏好某 peer”，可用 `excluded_agents` 补偿。

8. **row lineage 为什么是数据不是散文？**
   对齐 LoopX AGENTS.md“Projection Sink Design”：rows 取代/迁移/退役时，用 `row_lifecycle/supersedes/superseded_by/source_id/migration_audit` 字段表达，而非要求读项目私有文档理解 row 为何变化。这是审计与可调试性的关键。

---

## 11. 验证与回归策略

- **单元**：每个状态机的 `LegalTransitions` 表驱动测试 + 负向测试（非法转移被拒）。对齐 AGENTS.md“Design tests from semantics, not observed output”。
- **不变量测试**：`SpendSlot` 仅在校验写回后合法；门不可被预算绕过；peer 确定性指派稳定性。
- **回归**：`AppConfig.ControlPlane=nil` 时 `go test ./... -race -count=1` 全绿、行为不变。这是零回归硬门。
- **smoke**：注册 goal → 触发心跳 → ShouldRun=true → 模拟写回 → SpendSlot → spent_slots+1（P0）；user_gate 阻塞 → 回答 → 解除（P1）；两 worker 抢同 todo CAS 冲突（P2）。
- **对齐 LoopX 契约**：把 LoopX 的 `loopx_interaction_contract_v0`、`scheduler_hint_v0`、`peer_agent_identity_v1`、`event_sourced_state_contract_v0` 作为 Go 端的契约参考，跨语言契约测试放 `tests/`（对齐现有跨语言契约测试模式）。
- **smoke 保留纪律**（对齐 LoopX AGENTS.md Smoke Retention Policy）：只保留验证**持久公开行为**的 smoke（CLI/运行时行为、控制平面契约、公私边界、回归卡住的自动化）。一次性断言“某 dated 决策包文本”的 smoke 不留。

---

## 12. 不做什么（反范围）

明确排除，避免过度工程（AGENTS.md“Engineering Quality And Right-Sized Scope”）：

- **不重造运行时**。控制平面不执行工作；ReAct/channel/工具调用全留现有。
- **不做新的权限系统**。复用 `permission/`；配额不是权限。
- **不做新的记忆系统**。复用 ReMe/evolver/agentic_memory；reward memory 只加权威分类。
- **P0 不做** Kanban 投影、领域 playbook、peer 抢占、Lark 适配（全在 P2-P3）。
- **不做自动生产控制器**。对齐 LoopX README“LoopX is not an autonomous production controller”——危险权限、发布、生产写入、最终所有权留在人。
- **不硬编码项目特定监控**。对齐 LoopX AGENTS.md“Automation And Monitor Todos”：`continuous_monitor` todo 承载项目特定监控元数据；心跳保持通用，通过 status/quota/todo 投影发现监控工作。
- **不在热路径调 LLM**。ShouldRun 纯状态机。
- **不把投影当真相源**。board/kanban 永远只读。

---

## 13. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 控制平面引入复杂度，即时对话场景被拖累 | 默认关闭；`AppConfig.ControlPlane=nil` 零开销零回归 |
| Goal 状态机设计错误导致长时序任务卡死 | P0 先 4 状态最小集；负向测试覆盖死锁；`paused` 可人工恢复 |
| 与 evolver `SolidifyRequest` 抽象重叠产生两套 | P0 即让 `ValidatedWriteback` 成为单一来源，evolver 改调用（去重第一优先） |
| peer 哈希指派在 agent 集变化时迁移 | 哈希基于规范 work key + 排序 agent 集；注册顺序不影响；迁移通过 row lineage 记录 |
| 租约 TTL 与正在执行的长工具冲突 | 工具用现有 Tool Offload 机制卸载；租约可 renew；TTL 上限 24h |
| 控制平面 + 运行时两套状态混淆 | 严格分层：运行时=消息+工具态（`AgentState`）；控制平面=目标+todo+证据（`Goal`）。文档明确边界 |
| 范围蔓延成“第二个 LoopX” | 反范围清单（§12）+ 分期硬门 + 每期可独立验收回滚 |

---

## 14. 路线图总览

| 期 | 周期 | 交付 | 验收硬门 | 复用 |
|----|------|------|----------|------|
| **P0 脊柱** | 3-4 周 | Goal 内核 + ShouldRun + ValidatedWriteback + Interaction Contract + Kernel.Tick | Goal 跨重启；ShouldRun 拒 paused/throttled；SpendSlot 幂等 + 需写回；零回归 | service.Storage、schedule、middleware |
| **P1 纪律** | 3 周 | User Gate（decision_scope）+ FallbackPolicy + run history ledger | 门阻塞不 spend；fallback spend 一次门仍开；含糊拒推断；ledger 跨进程重放 | event/hitl、permission、evolver SolidifyRequest |
| **P2 协作+可视化** | 3-4 周 | peer claim/lease（接线 CoordBus.Lock）+ review board + Kanban 投影（row lineage） | 不同 todo 并行；同 todo CAS；peer 哈希稳定；review-packet 完整；lineage 链式 | messagebus.CoordBus、gateway/projection、web_ui |
| **P3 高级** | 4+ 周（按需） | 领域 capability 库（issue-fix 起步）+ reward memory 5 类分离 + 公私边界强制 | issue-fix lane 跑通；AuthorityClass 5 类；Visibility 强制 | evolver、skill、permission |

---

## 15. 结语

LoopX 和 AgentScope.go 是**互补的两层**，不是竞品。LoopX 领先的本质，是它回答了 AgentScope.go 从未回答的两个问题——**“这一回合该不该跑”**（配额/交互契约）和**“跨千回合的权威目标是什么”**（持久化生命周期目标）。这两点构成控制平面的脊柱，本仓库无任何等价物（grep 0 命中）。

本方案的策略是**补层不替换**：新增 `controlplane/` 顶层包，吸收 LoopX 脊柱，**最大限度复用**已有的 CoordBus.Lock、evolver.SolidifyRequest、event/hitl、gateway/projection、service.Storage。P0 只交付三件最小可用品（Goal + ShouldRun + 经校验写回），其余分期。每期可独立验收、可回滚，控制平面默认关闭保证零回归。

唯一全命中的奖励记忆（evolver）说明两个项目在“学习”层面已有共识；差距集中在“治理”层面。补上这层，AgentScope.go 将从“会干活的 Agent 框架”进化为“可治理、可复盘、可持续改进的数字员工平台”——这正是 LoopX 的口号。

> 把会干活的 Agent，接成可管理、可复盘、可持续改进的数字员工。
> —— LoopX README

---

### 附录 A：关键 LoopX 引用速查

| 概念 | LoopX 文档/源 |
|------|--------------|
| Lifetime Goal Invariant | `docs/architecture.md:231-258` |
| Runtime Responsibility Model | `docs/architecture.md:96-119`, `docs/reference/extensions.md:24-49` |
| Tick（核心节拍） | `docs/state-interaction-model.md:252-271` |
| Quota 状态优先级 | `docs/quota-allocation.md:117-134` |
| Interaction Contract | `docs/state-interaction-model.md:301-345`, `control_plane/work_items/interaction_contract.py:38` |
| Scheduler Hint | `docs/quota-allocation.md:531-539` |
| Spend 纪律 | `docs/quota-allocation.md:1014-1063` |
| Decision Scope（门） | `docs/concepts/interaction-pattern-catalog.md:146-200` |
| peer_v1（无持久 leader） | `docs/reference/protocols/peer-agent-runtime-v1.md:31-40` |
| 硬租约 | `cli_commands/task_lease.py`, `control_plane/work_items/task_lease.py` |
| 领域能力目录 | `loopx/capabilities/catalog.py:15` |
| 能力 vs 扩展（两正交轴） | `docs/reference/extensions.md:1-9, 272-340` |
| 投影 row lineage | `extensions/lark/presentation/projection_rows.py:132-234` |
| Reward Memory 5 类 | `docs/reference/protocols/reward-memory-architecture-v0.md:147-217` |
| 心跳自动化（通用发现） | `docs/heartbeat-automation-prompt.md:302-511`, `AGENTS.md` Automation And Monitor Todos |
| 事件账本契约 | `docs/state-interaction-model.md:527-571` (`event_sourced_state_contract_v0`) |

### 附录 B：AgentScope.go 待复用点速查

| 待复用 | 路径 | 用途 |
|--------|------|------|
| 分布式租约原语 | `messagebus/coord.go:18` `Lock(ctx,key,ttl)`；Redis 实现在 `coord_redis.go:13-57` | todo 硬租约后端 |
| 经校验写回模板 | `evolver/client.go:86` `SolidifyRequest{DecisionSource,PrimaryCause,ContributingFactors,...}` | 抽象成通用 `ValidatedWriteback` |
| Outcome/证据 | `evolver/types.go:107` `Outcome{Status,Score}`、`:139` `ValidationCommands` | evidence 字段 |
| HITL 悬挂/恢复 | `event/hitl_events.go:6`、`permission/engine.go:147`、`gateway/resume_handler.go:50` | 扩展为 User Gate |
| 决策模型 | `permission/engine.go:32` `Decision{allow,deny,ask,passthrough}` | gate outcome |
| 跨会话投影 | `gateway/projection.go:11` `SessionProjection` + `CoordBus.Registry*` | 扩成 GoalProjection + row lineage |
| 持久化 | `service/sql_storage.go`（8 表 + `scanRows[T]` + 级联删除） | 加 `goals` 表 |
| 心跳触发 | `schedule/scheduler.go:13` | 包装成 `QuotaHeartbeat`（先过 ShouldRun） |
| 配置接入 | `gateway/AppConfig` | 加 `ControlPlane` 可选字段 |
| 洋葱中间件 | `middleware/`（on_reply 钩子） | 新 `ControlPlaneMiddleware` |
| Web UI | `examples/web_ui`（零构建 vanilla JS + SSE） | 加 ControlPlane 视图 |
| 飞书回发 | `channel/feishu` `feishu_send_message` | Kanban 投影 sink |
