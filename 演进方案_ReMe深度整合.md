# AgentScope.Go 演进方案 —— ReMe Memory 系统深度整合

## 目录

1. [执行摘要](#1-执行摘要)
2. [ReMe 架构深度解析](#2-reme-架构深度解析)
3. [agentscope.go 现状分析](#3-agentscopego-现状分析)
4. [整合架构设计](#4-整合架构设计)
5. [Go 版 ReMe 完整实现方案](#5-go-版-reme-完整实现方案)
6. [演进路线图与里程碑](#6-演进路线图与里程碑)
7. [详细模块设计](#7-详细模块设计)
8. [性能与优化策略](#8-性能与优化策略)
9. [示例与使用场景](#9-示例与使用场景)
10. [测试与验证方案](#10-测试与验证方案)

---

## 1. 执行摘要

### 1.1 项目背景

**ReMe (Remember Me, Refine Me)** 是 AgentScope 生态中的企业级记忆管理框架，解决了 AI Agent 的两大核心痛点：

- **上下文窗口限制**: 长对话中早期信息被截断或丢失，导致"失忆"
- **会话无状态**: 新会话无法继承历史，总是"从头开始"

### 1.2 整合价值

```
┌─────────────────────────────────────────────────────────────────────┐
│                   整合后的 agentscope.go 架构                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   ┌──────────────┐        ┌────────────────────────────────────┐   │
│   │  Agent Core  │◄───────┤      ReMe Memory Layer             │   │
│   │  (ReAct)     │        │  ┌─────────┐  ┌────────────────┐   │   │
│   └──────────────┘        │  │ Working │  │  Long-term     │   │   │
│          ▲                │  │ Memory  │  │  Memory        │   │   │
│          │                │  └────┬────┘  └───────┬────────┘   │   │
│   ┌──────┴──────┐         │       └───────┬───────┘            │   │
│   │   Hook      │         │  ┌────────────┴──────────┐         │   │
│   │  System     │         │  │   Memory Interface    │         │   │
│   └─────────────┘         │  │  (ReMeMemory)         │         │   │
│                           │  └───────────────────────┘         │   │
│                           └────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.3 能力对比

| 能力维度 | 当前 agentscope.go | 整合 ReMe 后 |
|---------|-------------------|-------------|
| **短期记忆** | InMemory/WindowMemory | ✅ Working Memory + 智能压缩 |
| **长期记忆** | ❌ 无 | ✅ File/Vector 双模式 |
| **上下文管理** | 无限制增长 | ✅ Token感知 + 自动压缩 |
| **工具结果处理** | 直接存储 | ✅ 智能压缩 + 文件引用 |
| **语义检索** | ❌ 无 | ✅ 向量检索 + BM25混合 |
| **个人化记忆** | ❌ 无 | ✅ Personal Memory |
| **任务经验** | ❌ 无 | ✅ Procedural Memory |
| **工具使用优化** | ❌ 无 | ✅ Tool Memory |

---

## 2. ReMe 架构深度解析

### 2.1 ReMe 双模式架构

ReMe 采用分层设计，提供两种互补的记忆模式：

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ReMe 架构概览                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────────────┐    ┌──────────────────────────────┐  │
│  │   File-Based (Light)     │    │      Vector-Based            │  │
│  │   "Memory as Files"      │    │   "Semantic Memory"          │  │
│  ├──────────────────────────┤    ├──────────────────────────────┤  │
│  │ • Markdown 存储          │    │ • 向量数据库存储             │  │
│  │ • 人可读可编辑           │    │ • 三种记忆类型               │  │
│  │ • Git 友好               │    │   - Personal (用户偏好)      │  │
│  │ • 本地文件系统           │    │   - Procedural (任务经验)    │  │
│  │                          │    │   - Tool (工具使用)          │  │
│  └──────────────────────────┘    └──────────────────────────────┘  │
│                                                                     │
│  共享核心组件:                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐ │
│  │  Compactor  │  │  Summarizer │  │  Retriever  │  │ VectorStore│ │
│  │  (压缩器)   │  │  (摘要器)   │  │  (检索器)   │  │ (向量存储) │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 记忆类型详解

#### 2.2.1 Personal Memory (个人记忆)

**目标**: 理解用户偏好，提供个性化体验

```python
# ReMe Python 核心逻辑
class PersonalMemory(BaseMemory):
    """用户偏好、习惯、交互风格"""
    
    when_to_use: str  # 检索条件描述，如"用户询问饮食建议时"
    content: str      # 实际记忆内容，如"用户是素食主义者"
    target: str       # 关联用户，如"user_alice"
```

**提取流程**:
1. `GetObservationOp`: 从对话提取观察
2. `InfoFilterOp`: 信息过滤去噪
3. `UpdateInsightOp`: 更新洞察
4. `ContraRepeatOp`: 去重与冲突处理

#### 2.2.2 Procedural Memory (程序记忆)

**目标**: 积累任务执行经验，提升性能

```python
class TaskMemory(BaseMemory):
    """任务执行经验、成功/失败模式"""
    
    when_to_use: str  # "处理数据库查询优化时"
    content: str      # "优先使用索引，避免全表扫描"
    author: str       # 提取该记忆的模型
```

**提取流程**:
1. `TrajectorySegmentationOp`: 轨迹分段
2. `SuccessExtractionOp`: 成功模式提取
3. `FailureExtractionOp`: 失败教训提取
4. `ComparativeExtractionOp`: 对比学习
5. `MemoryValidationOp`: 记忆验证

#### 2.2.3 Tool Memory (工具记忆)

**目标**: 优化工具使用，生成动态使用指南

```python
class ToolMemory(BaseMemory):
    """工具使用经验、参数优化建议"""
    
    when_to_use: str           # 工具名称作为检索条件
    content: str               # 生成的使用指南
    tool_call_results: List    # 历史调用记录
    # 统计信息: 成功率、平均耗时、token消耗
```

**核心功能**:
- 历史调用评估 (LLM-as-Judge)
- 使用模式总结
- 动态使用指南生成

### 2.3 Working Memory (工作记忆)

解决上下文窗口爆炸问题的核心机制：

```
传统方式 (无上下文管理):
50 messages → 95,000 tokens → 上下文腐烂开始
- 响应质量: 下降
- 推理速度: 变慢
- 能否继续: 否 (接近限制)

Message Offload 方式:
50 messages → 15,000 tokens (offload 后) → 保持最优性能
- 20 tool messages compacted → 存储到 /context_store/
- 15 older messages compressed → 摘要到 system message
- 5 recent messages preserved → 保持完整内容
- 外部存储: 80,000 tokens offloaded
- 活动上下文: 15,000 tokens (减少 84%)
```

**核心组件**:

| 组件 | 功能 | 实现策略 |
|-----|------|---------|
| `ContextChecker` | Token 阈值检查 | 计算消息token数，触发压缩 |
| `Compactor` | 智能压缩 | LLM生成结构化摘要 |
| `ToolResultCompactor` | 工具结果处理 | 超长结果存文件，保留引用 |
| `Summarizer` | 持久化摘要 | 每日记忆写入 Markdown |

### 2.4 MemoryNode 核心数据结构

```python
class MemoryNode(BaseModel):
    memory_id: str          # SHA-256 哈希生成 (16字符)
    memory_type: MemoryType # PERSONAL/PROCEDURAL/TOOL/SUMMARY
    memory_target: str      # 关联目标 (用户/任务/工具名)
    when_to_use: str        # 检索条件描述 (向量化用)
    content: str            # 实际记忆内容
    message_time: str       # 源消息时间
    ref_memory_id: str      # 关联原始记忆
    time_created: str       # 创建时间
    time_modified: str      # 修改时间
    author: str             # 作者/模型
    score: float            # 相关性分数
    vector: List[float]     # 嵌入向量
    metadata: dict          # 扩展元数据
```

### 2.5 向量存储抽象

```python
class BaseVectorStore(ABC):
    """向量存储接口，支持多后端"""
    
    # 支持的后端实现:
    # - LocalVectorStore: 内存实现，开发测试
    # - ChromaVectorStore: ChromaDB
    # - QdrantVectorStore: Qdrant
    # - ESVectorStore: Elasticsearch
    # - PGVectorStore: PostgreSQL + pgvector
    
    async def insert(self, nodes: List[VectorNode]) -> None
    async def search(self, query: str, limit: int = 5, filters: dict = None) -> List[VectorNode]
    async def delete(self, vector_ids: str | list[str]) -> None
    async def update(self, nodes: List[VectorNode]) -> None
```

---

## 3. agentscope.go 现状分析

### 3.1 当前 Memory 实现

```
agentscope.go/memory/
├── memory.go                   # 基础 Memory 接口
├── inmemory.go                 # InMemoryMemory 实现
├── window.go                   # WindowMemory 滑动窗口
├── reme_types.go               # ReMe 核心类型 (已部分实现)
├── reme_memory.go              # ReMeMemory 接口
├── reme_file_memory.go         # ReMeFileMemory 实现
├── reme_vector_memory.go       # ReMeVectorMemory 实现
├── reme_hook.go                # ReMeHook 集成
├── compactor.go                # Compactor 实现
├── summarizer.go               # Summarizer 实现
├── summarizer_personal.go      # PersonalSummarizer
├── summarizer_procedural.go    # ProceduralSummarizer
├── summarizer_tool.go          # ToolSummarizer
├── context_checker.go          # ContextChecker 实现
├── tool_result_compactor.go    # ToolResultCompactor
├── deduplicator.go             # MemoryDeduplicator
├── vector_store_local.go       # LocalVectorStore 实现
├── hybrid_search.go            # 混合搜索 (向量 + BM25)
├── embedding.go                # EmbeddingModel 接口
└── handler/                    # Handler 层（新增）
    ├── orchestrator.go         # MemoryOrchestrator
    ├── memory_handler.go       # MemoryHandler
    ├── profile_handler.go      # ProfileHandler
    └── history_handler.go      # HistoryHandler
```

### 3.2 已实现功能清单

| 模块 | 状态 | 说明 |
|-----|------|------|
| `ReMeFileMemory` | ✅ 已实现 | 文件型记忆基础功能（组合 ReMeInMemoryMemory） |
| `ReMeVectorMemory` | ✅ 已实现 | 向量记忆基础功能 |
| `Compactor` | ✅ 已实现 | 基础压缩功能 |
| `ContextChecker` | ✅ 已实现 | 上下文检查 |
| `ToolResultCompactor` | ✅ 已实现 | 工具结果压缩 |
| `LocalVectorStore` | ✅ 已实现 | 本地向量存储 |
| `QdrantVectorStore` | ✅ 已实现 | Qdrant 远程向量存储 |
| `ChromaVectorStore` | ✅ 已实现 | Chroma 远程向量存储 |
| `ESVectorStore` | ✅ 已实现 | Elasticsearch 远程向量存储 |
| `PGVectorStore` | ✅ 已实现 | PostgreSQL + pgvector 远程向量存储 |
| `ReMeHook` | ✅ 已实现 | Hook 系统集成 |
| `HybridSearch` | ✅ 已实现 | 向量检索 + SQLite FTS5 BM25 混合重排 |
| `PersonalSummarizer` | ✅ 已实现 | 个人记忆自动提取 |
| `ProceduralSummarizer` | ✅ 已实现 | 任务经验自动提取 |
| `ToolSummarizer` | ✅ 已实现 | 工具使用指南生成 |
| `MemoryOrchestrator` | ✅ 已实现 | 编排 Summarize + Retrieve |
| `MemoryHandler` | ✅ 已实现 | 向量库 CRUD + 草稿检索 |
| `ProfileHandler` | ✅ 已实现 | 本地用户画像管理 |
| `HistoryHandler` | ✅ 已实现 | 历史记录节点读写 |

### 3.3 待完善功能清单

| 模块 | 优先级 | 说明 |
|-----|-------|------|
| VectorStore 快照 | P1 | 会话级持久化（已部分实现） |
| 配置系统整合 | P1 | 统一配置管理（config.ReMeMemoryConfig 已存在） |



---

## 4. 整合架构设计

### 4.1 整体架构图

```go
// ============================================================
// 整合后的 Memory 层次结构
// ============================================================

package memory

// Memory 基础接口 (保持向后兼容)
type Memory interface {
    Add(msg *message.Msg) error
    GetAll() ([]*message.Msg, error)
    GetRecent(n int) ([]*message.Msg, error)
    Clear() error
    Size() int
}

// ReMeMemory 增强接口 - 上下文管理能力
type ReMeMemory interface {
    Memory
    
    // 上下文管理
    CheckContext(ctx context.Context, threshold, reserve int) (*ContextCheckResult, error)
    CompactMemory(ctx context.Context, messages []*message.Msg, opts CompactOptions) (string, error)
    PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *CompactSummary, error)
    
    // Token 管理
    EstimateTokens(messages []*message.Msg) (*TokenStats, error)
    
    // 持久化
    SaveTo(sessionID string) error
    LoadFrom(sessionID string) error
    
    // 长期记忆
    GetMemoryForPrompt(prepend bool) ([]*message.Msg, error)
    SetLongTermMemory(text string)
}

// VectorMemory 向量记忆接口 - 语义检索能力
type VectorMemory interface {
    ReMeMemory
    
    // CRUD
    AddMemory(ctx context.Context, node *MemoryNode) error
    RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
    UpdateMemory(ctx context.Context, node *MemoryNode) error
    DeleteMemory(ctx context.Context, memoryID string) error
    
    // 类型化检索
    RetrievePersonal(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error)
    RetrieveProcedural(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error)
    RetrieveTool(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error)
}
```

### 4.2 组件映射关系

| ReMe (Python) | Go 对应实现 | 状态 |
|---------------|------------|------|
| `ReMeLight` | `ReMeFileMemory` | ✅ 已实现 |
| `ReMeInMemoryMemory` | `ReMeInMemoryMemory` | ✅ 已实现 |
| `ReMe` | `ReMeVectorMemory` | ✅ 已实现 |
| `ContextChecker` | `context_checker.go` | ✅ 已实现 |
| `Compactor` | `compactor.go` | ✅ 已实现 |
| `Summarizer` | `summarizer.go` | ✅ 基础版 |
| `ToolResultCompactor` | `tool_result_compactor.go` | ✅ 已实现 |
| `MemoryNode` | `MemoryNode` struct | ✅ 已实现 |
| `BaseVectorStore` | `LocalVectorStore` | ✅ 已实现 |
| `ToolMemory` | `ToolSummarizer` + `ReActAgent` 自动闭环 | ✅ 已实现 |

### 4.3 与 Agent 层集成

```
┌─────────────────────────────────────────────────────────────────┐
│                      Agent 层整合                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌──────────────┐        ┌──────────────────────────────┐     │
│   │ ReActAgent   │───────►│    Hook System               │     │
│   │              │        │  ┌────────────────────────┐  │     │
│   └──────────────┘        │  │ HookBeforeModel        │  │     │
│          ▲                │  │  └─► ReMeHook          │  │     │
│          │                │  │       └─► PreReasoning │  │     │
│   ┌──────┴──────┐         │  │             Prepare    │  │     │
│   │  Memory     │         │  └────────────────────────┘  │     │
│   │  (ReMe)     │         └──────────────────────────────┘     │
│   └─────────────┘                                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 5. Go 版 ReMe 完整实现方案

### 5.1 核心类型定义 (已实现)

```go
// memory/reme_types.go
package memory

import (
    "crypto/sha256"
    "encoding/hex"
    "time"
    "github.com/linkerlin/agentscope.go/message"
)

// MemoryType 记忆类型
type MemoryType string

const (
    MemoryTypePersonal   MemoryType = "personal"   // 用户偏好
    MemoryTypeProcedural MemoryType = "procedural" // 任务经验
    MemoryTypeTool       MemoryType = "tool"       // 工具使用
    MemoryTypeSummary    MemoryType = "summary"    // 摘要
)

// MemoryNode 记忆节点
type MemoryNode struct {
    MemoryID      string         `json:"memory_id"`
    MemoryType    MemoryType     `json:"memory_type"`
    MemoryTarget  string         `json:"memory_target"`  // 用户/任务/工具名
    WhenToUse     string         `json:"when_to_use"`    // 检索条件
    Content       string         `json:"content"`
    MessageTime   time.Time      `json:"message_time"`
    RefMemoryID   string         `json:"ref_memory_id"`
    TimeCreated   time.Time      `json:"time_created"`
    TimeModified  time.Time      `json:"time_modified"`
    Author        string         `json:"author"`
    Score         float64        `json:"score"`
    Vector        []float32      `json:"vector,omitempty"`
    Metadata      map[string]any `json:"metadata"`
}

// NewMemoryNode 创建新记忆节点
func NewMemoryNode(memType MemoryType, target, content string) *MemoryNode {
    now := time.Now()
    return &MemoryNode{
        MemoryID:     GenerateMemoryID(content + "|" + target),
        MemoryType:   memType,
        MemoryTarget: target,
        Content:      content,
        TimeCreated:  now,
        TimeModified: now,
        Metadata:     make(map[string]any),
    }
}

// GenerateMemoryID 从内容生成唯一ID
func GenerateMemoryID(content string) string {
    sum := sha256.Sum256([]byte(content))
    return hex.EncodeToString(sum[:])[:16]
}

// CompactSummary 结构化压缩摘要
type CompactSummary struct {
    Goal            string   `json:"goal"`
    Constraints     []string `json:"constraints"`
    Progress        string   `json:"progress"`
    KeyDecisions    []string `json:"key_decisions"`
    NextSteps       []string `json:"next_steps"`
    CriticalContext []string `json:"critical_context"`
    Raw             string   `json:"raw"`
}

// RetrieveOptions 检索选项
type RetrieveOptions struct {
    TopK             int          `json:"top_k"`
    MinScore         float64      `json:"min_score"`
    MemoryTypes      []MemoryType `json:"memory_types,omitempty"`
    MemoryTargets    []string     `json:"memory_targets,omitempty"`
    EnableTimeFilter bool         `json:"enable_time_filter"`
    VectorWeight     float64      `json:"vector_weight"` // 0=纯BM25, 1=纯向量
}

// CompactOptions 压缩选项
type CompactOptions struct {
    MaxInputLength   int     `json:"max_input_length"`
    CompactRatio     float64 `json:"compact_ratio"`
    ReserveTokens    int     `json:"reserve_tokens"`
    PreviousSummary  string  `json:"previous_summary"`
    Language         string  `json:"language"`
    AddThinkingBlock bool    `json:"add_thinking_block"`
}
```

### 5.2 ReMeFileMemory 实现 (已实现)

```go
// memory/reme_file_memory.go
package memory

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model"
)

// ReMeFileMemory 文件型 ReMe 记忆
type ReMeFileMemory struct {
    mu       sync.RWMutex
    msgs     []*message.Msg
    marks    *MarkStore
    compSum  string
    longTerm string

    workingPath    string
    memoryPath     string
    toolResultPath string
    dialogPath     string
    sessionsPath   string

    tokenCounter TokenCounter
    compactor    *Compactor
    toolCompact  *ToolResultCompactor
    config       ReMeFileConfig
}

// ReMeFileConfig 配置
type ReMeFileConfig struct {
    WorkingDir              string  `json:"working_dir"`
    MaxInputLength          int     `json:"max_input_length"`          // 默认 128k
    CompactRatio            float64 `json:"compact_ratio"`             // 默认 0.7
    MemoryCompactReserve    int     `json:"memory_compact_reserve"`    // 默认 10k
    ToolResultRetentionDays int     `json:"tool_result_retention_days"` // 默认 3
    RecentMaxBytes          int     `json:"recent_max_bytes"`          // 默认 100KB
    OldMaxBytes             int     `json:"old_max_bytes"`             // 默认 3KB
    Language                string  `json:"language"`                  // 默认 "zh"
}

// DefaultReMeFileConfig 默认配置
func DefaultReMeFileConfig() ReMeFileConfig {
    return ReMeFileConfig{
        WorkingDir:              ".reme",
        MaxInputLength:          128 * 1024,
        CompactRatio:            0.7,
        MemoryCompactReserve:    10000,
        ToolResultRetentionDays: 3,
        RecentMaxBytes:          100 * 1024,
        OldMaxBytes:             3000,
        Language:                "zh",
    }
}

// NewReMeFileMemory 创建文件型记忆
func NewReMeFileMemory(cfg ReMeFileConfig, counter TokenCounter) (*ReMeFileMemory, error) {
    // 初始化目录结构
    base := cfg.WorkingDir
    dirs := []string{
        base,
        filepath.Join(base, "memory"),
        filepath.Join(base, "dialog"),
        filepath.Join(base, "tool_result"),
        filepath.Join(base, "sessions"),
    }
    for _, d := range dirs {
        if err := os.MkdirAll(d, 0o755); err != nil {
            return nil, err
        }
    }
    m := &ReMeFileMemory{
        marks:          NewMarkStore(),
        workingPath:    base,
        memoryPath:     filepath.Join(base, "memory"),
        dialogPath:     filepath.Join(base, "dialog"),
        toolResultPath: filepath.Join(base, "tool_result"),
        sessionsPath:   filepath.Join(base, "sessions"),
        tokenCounter:   counter,
        config:         cfg,
    }
    m.toolCompact = NewToolResultCompactor(m.toolResultPath, cfg.RecentMaxBytes, cfg.OldMaxBytes, cfg.ToolResultRetentionDays)
    return m, nil
}

// InitCompactorWithModel 注入用于压缩的 ChatModel
func (m *ReMeFileMemory) InitCompactorWithModel(cm model.ChatModel) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if cm == nil {
        m.compactor = nil
        return
    }
    m.compactor = NewCompactor(cm)
}

// PreReasoningPrepare 在模型调用前准备消息视图
func (m *ReMeFileMemory) PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *CompactSummary, error) {
    if len(history) == 0 {
        return history, nil, nil
    }
    
    // 1. 压缩工具结果
    h := history
    if m.toolCompact != nil {
        var err error
        h, err = m.toolCompact.Compact(h, 2)
        if err != nil {
            return nil, nil, err
        }
    }
    
    // 2. 检查上下文
    threshold := int(float64(m.config.MaxInputLength) * m.config.CompactRatio)
    reserve := m.config.MemoryCompactReserve
    cc, err := CheckContext(ctx, h, threshold, reserve, m.tokenCounter)
    if err != nil {
        return nil, nil, err
    }
    
    if len(cc.MessagesToCompact) == 0 {
        return h, nil, nil
    }
    
    // 3. 压缩记忆
    if m.compactor == nil {
        return h, nil, nil
    }
    
    sum, err := m.compactor.Compact(ctx, cc.MessagesToCompact, CompactOptions{
        Language:        m.config.Language,
        PreviousSummary: m.compSum,
    })
    if err != nil {
        return nil, nil, err
    }
    
    m.mu.Lock()
    m.compSum = sum.Raw
    m.mu.Unlock()
    
    // 4. 组装结果：摘要 + 保留消息
    var out []*message.Msg
    if sum.Raw != "" {
        out = append(out, message.NewMsg().
            Role(message.RoleUser).
            TextContent("# Summary of previous conversation\n\n"+sum.Raw).
            Build())
    }
    out = append(out, cc.MessagesToKeep...)
    return out, sum, nil
}

// GetMemoryForPrompt 返回带长期记忆的消息视图
func (m *ReMeFileMemory) GetMemoryForPrompt(prepend bool) ([]*message.Msg, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    filtered := m.getFiltered(true)
    if !prepend {
        return append([]*message.Msg(nil), filtered...), nil
    }
    var parts []string
    if m.longTerm != "" {
        parts = append(parts, "# Memories\n\n"+m.longTerm)
    }
    if m.compSum != "" {
        parts = append(parts, "# Summary of previous conversation\n\n"+m.compSum)
    }
    if len(parts) == 0 {
        return append([]*message.Msg(nil), filtered...), nil
    }
    sumMsg := message.NewMsg().
        Role(message.RoleUser).
        Name("user").
        TextContent(strings.Join(parts, "\n\n")).
        Build()
    return append([]*message.Msg{sumMsg}, filtered...), nil
}
```

### 5.3 ReMeVectorMemory 实现 (已实现)

```go
// memory/reme_vector_memory.go
package memory

import (
    "context"
    "os"
    "path/filepath"
    "github.com/linkerlin/agentscope.go/message"
)

// VectorMemory 在 ReMeMemory 之上增加向量 CRUD 与类型化检索
type VectorMemory interface {
    ReMeMemory
    AddMemory(ctx context.Context, node *MemoryNode) error
    RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
    UpdateMemory(ctx context.Context, node *MemoryNode) error
    DeleteMemory(ctx context.Context, memoryID string) error
    RetrievePersonal(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error)
    RetrieveProcedural(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error)
    RetrieveTool(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error)
}

// ReMeVectorMemory 组合文件记忆与向量存储
type ReMeVectorMemory struct {
    *ReMeFileMemory
    store *LocalVectorStore
}

// NewReMeVectorMemory 创建向量记忆
func NewReMeVectorMemory(cfg ReMeFileConfig, counter TokenCounter, store *LocalVectorStore, embed EmbeddingModel) (*ReMeVectorMemory, error) {
    f, err := NewReMeFileMemory(cfg, counter)
    if err != nil {
        return nil, err
    }
    if store == nil {
        if embed == nil {
            return nil, ErrEmbeddingRequired
        }
        store = NewLocalVectorStore(embed)
    }
    return &ReMeVectorMemory{ReMeFileMemory: f, store: store}, nil
}

// AddMemory 写入向量库
func (v *ReMeVectorMemory) AddMemory(ctx context.Context, node *MemoryNode) error {
    if v == nil || v.store == nil {
        return ErrEmbeddingRequired
    }
    return v.store.Insert(ctx, []*MemoryNode{node})
}

// RetrieveMemory 语义检索
func (v *ReMeVectorMemory) RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
    if v == nil || v.store == nil {
        return nil, ErrEmbeddingRequired
    }
    nodes, err := v.store.Search(ctx, query, opts)
    if err != nil {
        return nil, err
    }
    // 混合重排
    w := opts.VectorWeight
    if w > 0 && w < 1 && len(nodes) > 0 {
        nodes = RankMemoryNodesHybrid(nodes, query, w)
    }
    return nodes, nil
}

// 类型化检索
func (v *ReMeVectorMemory) RetrievePersonal(ctx context.Context, userName, query string, topK int) ([]*MemoryNode, error) {
    return v.RetrieveMemory(ctx, query, RetrieveOptions{
        TopK:          topK,
        MemoryTypes:   []MemoryType{MemoryTypePersonal},
        MemoryTargets: []string{userName},
    })
}

func (v *ReMeVectorMemory) RetrieveProcedural(ctx context.Context, taskName, query string, topK int) ([]*MemoryNode, error) {
    return v.RetrieveMemory(ctx, query, RetrieveOptions{
        TopK:          topK,
        MemoryTypes:   []MemoryType{MemoryTypeProcedural},
        MemoryTargets: []string{taskName},
    })
}

func (v *ReMeVectorMemory) RetrieveTool(ctx context.Context, toolName, query string, topK int) ([]*MemoryNode, error) {
    return v.RetrieveMemory(ctx, query, RetrieveOptions{
        TopK:          topK,
        MemoryTypes:   []MemoryType{MemoryTypeTool},
        MemoryTargets: []string{toolName},
    })
}

// SaveTo 持久化文件记忆 + 向量快照
func (v *ReMeVectorMemory) SaveTo(sessionID string) error {
    if v == nil || v.ReMeFileMemory == nil {
        return errors.New("memory: nil ReMeVectorMemory")
    }
    if err := v.ReMeFileMemory.SaveTo(sessionID); err != nil {
        return err
    }
    if v.store == nil {
        return nil
    }
    path := filepath.Join(v.sessionsPath, sessionID+".vector.json")
    return v.store.WriteSnapshot(path)
}

// LoadFrom 加载会话快照
func (v *ReMeVectorMemory) LoadFrom(sessionID string) error {
    if v == nil || v.ReMeFileMemory == nil {
        return errors.New("memory: nil ReMeVectorMemory")
    }
    if err := v.ReMeFileMemory.LoadFrom(sessionID); err != nil {
        return err
    }
    if v.store == nil {
        return nil
    }
    path := filepath.Join(v.sessionsPath, sessionID+".vector.json")
    if _, err := os.Stat(path); err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }
    return v.store.ReadSnapshot(path)
}
```

---

## 6. 演进路线图与里程碑

### 6.1 阶段规划

```
Phase 1: 基础层完善 (1-2 周) ✅ 已完成
├── TokenCounter 接口与实现
├── MessageMark 消息标记系统
├── msgHandler 消息处理工具
└── 基础测试覆盖 (79%+)

Phase 1: 基础层完善 (1-2 周) ✅ 已完成
├── TokenCounter 接口与实现
├── MessageMark 消息标记系统
├── msgHandler 消息处理工具
└── 基础测试覆盖 (79%+)

Phase 2: File-Based Memory 完善 (2-3 周) ✅ 已完成
├── ReMeFileMemory 核心实现 ✅
├── ContextChecker 上下文检查 ✅
├── ToolResultCompactor 工具结果压缩 ✅
├── Compactor 记忆压缩 ✅
├── Summarizer 持久化摘要 ✅
├── 异步摘要任务 ✅
└── 目录结构管理 ✅

Phase 3: Vector-Based Memory 完善 (2-3 周) ✅ 已完成
├── VectorStore 接口 ✅
├── LocalVectorStore 实现 ✅
├── EmbeddingModel 接口 ✅
├── ReMeVectorMemory 实现 ✅
├── MemoryNode CRUD ✅
└── VectorStore 快照持久化 ✅

Phase 4: 高级记忆功能 (3-4 周) ✅ 已完成
├── PersonalSummarizer 个人记忆提取 ✅
├── ProceduralSummarizer 任务经验提取 ✅
├── ToolSummarizer 工具使用指南生成 ✅
├── MemoryOrchestrator 编排层 ✅
├── MemoryHandler / ProfileHandler / HistoryHandler ✅
├── Hybrid Search 完善 (BM25索引) ⚠️ 简化版
└── 与 ReAct Agent 深度集成 ✅ (通过 ReMeHook)

Phase 4.5: BM25/FTS5 全文索引 (1-2 周) ✅ 已完成
├── SQLite + FTS5 封装 (`memory/fts_index.go`)
├── BM25 与向量混合重排升级
├── ReMeFileMemory/ReMeVectorMemory 集成
└── 端到端测试与示例更新

Phase 5: 生产就绪 (2-3 周) 📋 计划中
├── 性能优化 (并发、缓存)
├── 完整测试覆盖 ✅ (~85% 核心路径已覆盖)
├── 文档和示例完善 ✅
├── 多后端 VectorStore ✅ (Qdrant/Chroma/ES/PGVector)
├── 与 AgentScope-Java 功能对齐验证
└── 发布准备
```

### 6.2 里程碑定义

| 里程碑 | 目标日期 | 关键交付物 | 成功标准 |
|-------|---------|-----------|---------|
| M1 - 基础稳定 | Week 2 | ReMeFileMemory 完整实现 | ✅ `go test ./memory/...` 全绿 |
| M2 - 向量就绪 | Week 5 | ReMeVectorMemory 完整实现 | ✅ 向量检索 + 快照可用 |
| M3 - 智能提取 | Week 8 | Orchestrator + 自动提取 | ✅ `examples/reme/orchestrator` 运行成功 |
| M4 - 生产发布 | Week 11 | 完整文档与测试 | 🔄 与 Python ReMe 功能对齐度 ~85% |

---

## 7. 详细模块设计

### 7.1 PersonalSummarizer 个人记忆提取器

```go
// memory/summarizer_personal.go
package memory

import (
    "context"
    "strings"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model"
)

// PersonalSummarizer 从对话提取个人记忆
type PersonalSummarizer struct {
    Model model.ChatModel
}

// Extract 提取个人观察
func (s *PersonalSummarizer) Extract(ctx context.Context, msgs []*message.Msg, userName string) ([]*MemoryNode, error) {
    if s == nil || s.Model == nil {
        return nil, nil
    }
    
    // 过滤时间相关消息
    filtered := s.filterMessages(msgs)
    if len(filtered) == 0 {
        return nil, nil
    }
    
    // 构建提取提示
    prompt := s.buildExtractionPrompt(filtered, userName)
    
    resp, err := s.Model.Chat(ctx, []*message.Msg{
        message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
    })
    if err != nil {
        return nil, err
    }
    
    // 解析提取结果
    return s.parseObservations(resp.GetTextContent(), filtered, userName), nil
}

func (s *PersonalSummarizer) buildExtractionPrompt(msgs []*message.Msg, userName string) string {
    var sb strings.Builder
    sb.WriteString("从以下对话中提取关于用户的信息。\n\n")
    sb.WriteString("规则:\n")
    sb.WriteString("1. 只提取事实性信息，不猜测\n")
    sb.WriteString("2. 格式: 信息：<序号> <> <内容> <关键词>\n")
    sb.WriteString("3. 如果无有效信息，输出: 无\n\n")
    sb.WriteString("对话:\n")
    for i, m := range msgs {
        sb.WriteStringf("%d %s: %s\n", i+1, userName, m.GetTextContent())
    }
    return sb.String()
}

func (s *PersonalSummarizer) parseObservations(text string, msgs []*message.Msg, userName string) []*MemoryNode {
    // 解析格式: 信息：<序号> <> <内容> <关键词>
    // 返回 MemoryNode 列表
}
```

### 7.2 ProceduralSummarizer 任务经验提取器

```go
// memory/summarizer_procedural.go
package memory

// Trajectory 执行轨迹
type Trajectory struct {
    Messages []*message.Msg
    Score    float64  // 成功评分
    TaskName string
}

// ProceduralSummarizer 提取任务经验
type ProceduralSummarizer struct {
    Model model.ChatModel
}

// Extract 从轨迹提取任务记忆
func (s *ProceduralSummarizer) Extract(ctx context.Context, trajectories []Trajectory) ([]*MemoryNode, error) {
    var memories []*MemoryNode
    
    for _, traj := range trajectories {
        // 根据评分分类处理
        if traj.Score >= 0.9 {
            // 成功模式提取
            mems, err := s.extractSuccessPattern(ctx, traj)
            if err != nil {
                return nil, err
            }
            memories = append(memories, mems...)
        } else if traj.Score < 0.5 {
            // 失败教训提取
            mems, err := s.extractFailureLesson(ctx, traj)
            if err != nil {
                return nil, err
            }
            memories = append(memories, mems...)
        }
    }
    
    // 去重与验证
    return s.deduplicateAndValidate(memories), nil
}

func (s *ProceduralSummarizer) extractSuccessPattern(ctx context.Context, traj Trajectory) ([]*MemoryNode, error) {
    prompt := "从以下成功执行中提取可复用的经验:\n\n" + formatTrajectory(traj)
    // 调用模型提取...
}
```

### 7.3 ToolSummarizer 工具记忆提取器

```go
// memory/summarizer_tool.go
package memory

// ToolCallResult 工具调用结果
type ToolCallResult struct {
    CreateTime   time.Time
    ToolName     string
    Input        map[string]any
    Output       string
    TokenCost    int
    Success      bool
    TimeCost     float64
    IsSummarized bool
}

// ToolSummarizer 生成工具使用指南
type ToolSummarizer struct {
    Model model.ChatModel
}

// Summarize 总结工具使用模式
func (s *ToolSummarizer) Summarize(ctx context.Context, toolName string, results []ToolCallResult) (*MemoryNode, error) {
    if len(results) == 0 {
        return nil, nil
    }
    
    // 计算统计信息
    stats := calculateStats(results)
    
    // 构建总结提示
    prompt := s.buildSummaryPrompt(toolName, results, stats)
    
    resp, err := s.Model.Chat(ctx, []*message.Msg{
        message.NewMsg().Role(message.RoleUser).TextContent(prompt).Build(),
    })
    if err != nil {
        return nil, err
    }
    
    // 生成使用指南
    guide := resp.GetTextContent()
    content := guide + "\n\n## Statistics\n" + formatStats(stats)
    
    node := NewMemoryNode(MemoryTypeTool, toolName, content)
    node.WhenToUse = toolName
    
    // 标记已总结
    for i := range results {
        results[i].IsSummarized = true
    }
    
    return node, nil
}
```

---

## 8. 性能与优化策略

### 8.1 Token 计数优化

```go
// memory/token_counter.go

// TokenCounter Token 计数器接口
type TokenCounter interface {
    Count(text string) (int, error)
    CountMessages(msgs []*message.Msg) (int, error)
}

// SimpleTokenCounter 基于字符的近似计数
type SimpleTokenCounter struct {
    charsPerToken int
}

func NewSimpleTokenCounter() *SimpleTokenCounter {
    return &SimpleTokenCounter{charsPerToken: 4}
}

func (c *SimpleTokenCounter) Count(text string) (int, error) {
    return len([]rune(text)) / c.charsPerToken, nil
}

// TiktokenCounter 基于 tiktoken 的精确计数 (通过 WASM 或外部服务)
type TiktokenCounter struct {
    encoding string
}
```

### 8.2 向量检索优化

```go
// memory/vector_store_local.go

// LocalVectorStore 优化版本
type LocalVectorStore struct {
    mu      sync.RWMutex
    dim     int
    embed   EmbeddingModel
    nodes   map[string]*MemoryNode
    
    // 优化: 按类型索引
    byType map[MemoryType]map[string]struct{}
    // 优化: 按目标索引
    byTarget map[string]map[string]struct{}
}

// 预过滤优化
func (s *LocalVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
    // 1. 利用索引预过滤候选集
    candidates := s.preFilter(opts)
    
    // 2. 向量化查询
    qv, err := s.embed.Embed(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // 3. 计算相似度 (仅对候选集)
    var scored []scoredNode
    for _, n := range candidates {
        sim := CosineSimilarity(qv, n.Vector)
        if sim >= opts.MinScore {
            scored = append(scored, scoredNode{n, sim})
        }
    }
    
    // 4. TopK 排序
    return topK(scored, opts.TopK), nil
}
```

### 8.3 并发策略

```go
// memory/compactor.go

// AsyncCompact 异步压缩
func (c *Compactor) AsyncCompact(ctx context.Context, msgs []*message.Msg, opts CompactOptions) (<-chan *CompactSummary, <-chan error) {
    sumCh := make(chan *CompactSummary, 1)
    errCh := make(chan error, 1)
    
    go func() {
        defer close(sumCh)
        defer close(errCh)
        
        sum, err := c.Compact(ctx, msgs, opts)
        if err != nil {
            errCh <- err
            return
        }
        sumCh <- sum
    }()
    
    return sumCh, errCh
}
```

---

## 9. 示例与使用场景

### 9.1 基础使用示例

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
    ctx := context.Background()
    
    // 1. 创建 Token 计数器
    tokenCounter := memory.NewSimpleTokenCounter()
    
    // 2. 创建 ReMeFileMemory
    memConfig := memory.DefaultReMeFileConfig()
    memConfig.WorkingDir = ".my_agent_memory"
    memConfig.MaxInputLength = 128000
    
    mem, err := memory.NewReMeFileMemory(memConfig, tokenCounter)
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. 创建模型
    model, _ := openai.Builder().
        APIKey("your-api-key").
        ModelName("gpt-4o-mini").
        Build()
    
    // 4. 初始化 Compactor
    mem.InitCompactorWithModel(model)
    
    // 5. 创建 Agent
    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful assistant with long-term memory.").
        Model(model).
        Memory(mem).
        Hooks(memory.NewReMeHook(mem)).
        Build()
    
    // 6. 运行对话
    resp, _ := agent.Call(ctx, message.NewMsg().
        Role(message.RoleUser).
        TextContent("你好，我想学习 Go 语言").
        Build())
    
    fmt.Println(resp.GetTextContent())
}
```

### 9.2 向量记忆使用示例

```go
func vectorMemoryExample() {
    ctx := context.Background()
    
    // 创建嵌入模型
    embedding := memory.NewOpenAIEmbedding(
        "your-api-key",
        "text-embedding-3-small",
    )
    
    // 创建向量记忆
    fileConfig := memory.DefaultReMeFileConfig()
    mem, _ := memory.NewReMeVectorMemory(
        fileConfig,
        memory.NewSimpleTokenCounter(),
        nil,
        embedding,
    )
    
    // 添加个人记忆
    mem.AddMemory(ctx, &memory.MemoryNode{
        MemoryType:   memory.MemoryTypePersonal,
        MemoryTarget: "user_alice",
        Content:      "Alice prefers concise code examples",
        WhenToUse:    "When providing code examples to Alice",
    })
    
    // 检索记忆
    memories, _ := mem.RetrievePersonal(ctx, "user_alice", "Go learning", 5)
    
    for _, m := range memories {
        fmt.Printf("- %s (score: %.2f)\n", m.Content, m.Score)
    }
}
```

### 9.3 会话持久化示例

```go
func sessionPersistenceExample() {
    // 创建记忆
    mem, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embed)
    
    // 添加消息和记忆...
    mem.Add(msg)
    mem.AddMemory(ctx, node)
    
    // 保存会话
    sessionID := "user_alice_2024_01_15"
    if err := mem.SaveTo(sessionID); err != nil {
        log.Fatal(err)
    }
    
    // 新会话恢复
    mem2, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embed)
    if err := mem2.LoadFrom(sessionID); err != nil {
        log.Fatal(err)
    }
    
    // 继续对话，保留所有上下文
}
```

---

## 10. 测试与验证方案

### 10.1 单元测试覆盖

```go
// memory/reme_file_memory_test.go
func TestReMeFileMemory_PreReasoningPrepare(t *testing.T) {
    ctx := context.Background()
    
    // 创建测试记忆
    mem, _ := NewReMeFileMemory(DefaultReMeFileConfig(), NewSimpleTokenCounter())
    
    // 添加测试消息
    for i := 0; i < 100; i++ {
        mem.Add(message.NewMsg().
            Role(message.RoleUser).
            TextContent("Test message " + strconv.Itoa(i)).
            Build())
    }
    
    // 测试准备
    msgs, sum, err := mem.PreReasoningPrepare(ctx, mem.msgs)
    
    // 验证
    assert.NoError(t, err)
    assert.NotNil(t, msgs)
    assert.True(t, len(msgs) < 100) // 应该被压缩
}
```

### 10.2 集成测试

```go
// memory/reme_integration_test.go
func TestReMeVectorMemory_EndToEnd(t *testing.T) {
    ctx := context.Background()
    
    // 创建临时目录
    dir, _ := os.MkdirTemp("", "reme-test-*")
    defer os.RemoveAll(dir)
    
    cfg := DefaultReMeFileConfig()
    cfg.WorkingDir = dir
    
    // 使用伪嵌入
    embed := &fixedEmbed{dim: 8}
    
    // 创建向量记忆
    mem, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, embed)
    require.NoError(t, err)
    
    // 添加记忆
    nodes := []*MemoryNode{
        NewMemoryNode(MemoryTypePersonal, "alice", "喜欢Go语言"),
        NewMemoryNode(MemoryTypePersonal, "alice", "做后端开发"),
        NewMemoryNode(MemoryTypeProcedural, "db_query", "使用索引优化"),
    }
    
    for _, n := range nodes {
        err := mem.AddMemory(ctx, n)
        require.NoError(t, err)
    }
    
    // 检索测试
    results, err := mem.RetrievePersonal(ctx, "alice", "编程语言", 5)
    require.NoError(t, err)
    assert.True(t, len(results) > 0)
    
    // 保存/加载测试
    err = mem.SaveTo("test-session")
    require.NoError(t, err)
    
    mem2, _ := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, embed)
    err = mem2.LoadFrom("test-session")
    require.NoError(t, err)
}
```

### 10.3 性能基准测试

```go
// memory/reme_bench_test.go
func BenchmarkLocalVectorStore_Search(b *testing.B) {
    ctx := context.Background()
    store := NewLocalVectorStore(&fixedEmbed{dim: 128})
    
    // 预填充数据
    for i := 0; i < 10000; i++ {
        node := NewMemoryNode(MemoryTypePersonal, "user", "content")
        node.Vector = make([]float32, 128)
        store.Insert(ctx, []*MemoryNode{node})
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        store.Search(ctx, "query", RetrieveOptions{TopK: 10})
    }
}
```

### 10.4 验收清单

| 功能 | 验收标准 | 状态 |
|-----|---------|------|
| `ReMeFileMemory` | 工作目录结构正确、SaveTo/LoadFrom 可用 | ✅ |
| `ReMeVectorMemory` | Add/Retrieve、类型化检索正确 | ✅ |
| `ReMeHook` | 与 ReActAgent Hook 协同正常 | ✅ |
| `Compactor` | 生成结构化摘要 | ✅ |
| `ContextChecker` | 正确切分上下文 | ✅ |
| `ToolResultCompactor` | 超长结果存文件 | ✅ |
| 混合搜索 | VectorWeight 混合重排正确 | ✅ |
| 示例运行 | file/vector 示例可编译运行 | ✅ |

---

## 11. 总结

### 11.1 整合价值

通过深度整合 ReMe Memory 系统，agentscope.go 将获得：

1. **企业级记忆管理能力**: 短期 Working Memory + 长期 Vector Memory 双模式
2. **智能上下文管理**: Token感知、自动压缩、工具结果优化
3. **语义检索能力**: 向量 + BM25 混合检索
4. **个人化支持**: 用户偏好自动提取与应用
5. **经验积累**: 任务执行经验自动总结复用
6. **工具优化**: 动态使用指南生成

### 11.2 技术亮点

| 特性 | 实现方式 |
|-----|---------|
| Go 惯用设计 | interface、struct embedding、context 传递 |
| 向后兼容 | 基础 Memory 接口保持不变 |
| 模块化 | File/Vector 可独立使用或组合 |
| 可配置 | Config 结构体灵活配置 |
| 异步友好 | 压缩、摘要支持异步执行 |

### 11.3 生态对齐

```
AgentScope-Java ────► 企业级完整方案
      │
      ▼
  ReMe (Python) ────► Memory 专业框架
      │
      ▼
agentscope.go ────► 轻量 Go 实现 + ReMe 整合 ✅
```

通过本演进方案的实施，agentscope.go 将成为具备业界领先 Memory 管理能力的 Go 语言 Agent 框架。
