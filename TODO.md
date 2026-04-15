# AgentScope.Go 后续 TODO

> 本文件记录 agentscope.go 项目中的剩余工作项，按优先级排序。  
> 最后更新：2026-04-15

---

## P0 - 架构级缺失（优先补齐）

### 1. AgentBase 统一基类
- **目标**：提取所有 Agent 共享的生命周期、Hook、状态管理，构建 `agent.Base` struct，解决 `ReActAgent` 孤立实现的问题。
- **关键交付物**：
  - `agent/base.go`：`Base` struct（含 Name, SysPrompt, hooks, streamHooks, shutdown, usage 等）
  - `ReActAgent` 改为嵌入 `Base`
  - 通用方法下沉：`Shutdown()`, `IsClosed()`, `TotalUsage()`, `fireHooks()`
- **状态**：🔄 进行中

### 2. Formatter 层设计与实现
- **目标**：补齐独立的 `Formatter` 抽象层，将 `Msg` 到模型 API 字典的转换逻辑从各模型实现中解耦。
- **关键交付物**：
  - `formatter/formatter.go`：定义 `Formatter` / `TruncatedFormatter` interface
  - `formatter/openai.go`：迁移现有 OpenAI 格式转换逻辑
  - `formatter/dashscope.go`、`formatter/ollama.go`：同理迁移
  - `model.ChatModel` 可选注入 `Formatter`
- **状态**：⏳ 待开始

### 3. Hook / 事件生命周期统一
- **目标**：在 `agent.Base` 中统一封装 `pre_reply` / `post_reply` / `pre_observe` / `post_observe` 等高层生命周期，减少 `ReActAgent` 中的手动触发代码。
- **关键交付物**：
  - 扩展 `hook.HookPoint` 枚举（对齐 Python 版高层生命周期）
  - 在 `Base.Call()` / `Base.Observe()` 中自动触发 Hook
  - 保留 `StreamHook` 作为细粒度事件补充
- **状态**：⏳ 待开始

---

## P1 - 功能级缺失（生产可用性）

### 4. 消息块对齐 Python 2.0
- **目标**：统一多媒体块、调整 Tool 块字段，提升与 Python 版的跨语言兼容性。
- **关键行动**：
  - 引入 `DataBlock` 统一替代 `ImageBlock` / `AudioBlock` / `VideoBlock`
  - `ToolUseBlock.Input` 增加 `string` 兼容路径（支持流式 JSON 累积）
  - `ToolResultBlock` 增加 `State` 字段
  - `message/json.go` 支持 `source` 嵌套结构序列化
- **状态**：⏳ 待开始

### 5. 扩展模型后端
- **目标**：新增 Anthropic、Gemini 等主流后端。
- **关键行动**：
  - `model/anthropic/`
  - `model/gemini/`
  - 统一走 Formatter 层
- **状态**：⏳ 待开始

### 6. ToolResponse 规范类型
- **目标**：替换工具返回的裸 `any`，支持多媒体结果。
- **关键交付物**：
  - `tool/response.go`：定义 `tool.Response` struct（含 `[]message.ContentBlock`）
  - 修改 `Tool.Execute` 签名或增加适配层
- **状态**：⏳ 待开始

### 7. Memory 自动集成到 ReActAgent
- **目标**：在 `ReActAgent.buildHistory` 中自动调用 `memory.PreReasoningPrepare()`，实现上下文压缩。
- **状态**：⏳ 待开始

---

## 历史已完成项（存档参考）

### ReMe Memory 系统（已全部完成 ✅）
- BM25/FTS5 全文检索 ✅
- 多后端 VectorStore（Qdrant/Chroma/ES/PGVector）✅
- ToolMemory 自动触发闭环 ✅
- ReMeInMemoryMemory 抽取 ✅
- EmbeddingCache + 并发控制 ✅
- BuildReMeVectorMemory 工厂 ✅
- 性能基准测试 ✅
- CHANGELOG / .gitignore / godoc 清理 ✅
- AgentScope-Java 对齐验证 ✅

---

*完成一项请勾选或删除对应条目，保持本文件实时更新。*
