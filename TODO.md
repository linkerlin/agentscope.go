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
  - 通用方法下沉：`Shutdown()`, `IsClosed()`, `TotalUsage()`, `FireHooks()`, `FireStreamEvent()`
- **状态**：✅ 已完成（commit `c762455`）

### 2. Formatter 层设计与实现
- **目标**：补齐独立的 `Formatter` 抽象层，将 `Msg` 到模型 API 字典的转换逻辑从各模型实现中解耦。
- **关键交付物**：
  - `formatter/formatter.go`：定义 `Formatter` interface
  - `formatter/openai.go`：`OpenAIFormatter` 正式实现 `Formatter` interface，保留 `Typed` 强类型方法供内部使用
  - `model/openai/openai.go`：注入并使用 `OpenAIFormatter`
  - `formatter/dashscope.go`、`formatter/ollama.go`：已创建独立 formatter 文件（OpenAI 兼容别名）
  - `model/dashscope`、`model/ollama`：builder 支持 `Formatter` 注入
- **状态**：✅ 已完成

### 3. Hook / 事件生命周期统一
- **目标**：在 `agent.Base` 中统一封装 `pre_reply` / `post_reply` / `pre_observe` / `post_observe` 等高层生命周期，减少 `ReActAgent` 中的手动触发代码。
- **关键交付物**：
  - 扩展 `hook.HookPoint` 枚举（对齐 Python 版高层生命周期）
  - 在 `Base.Call()` / `Base.Observe()` 中自动触发 Hook
  - 保留 `StreamHook` 作为细粒度事件补充
- **状态**：✅ 已完成（新增 `HookPreReply`/`HookPostReply`/`HookPreObserve`/`HookPostObserve`，`Base.Call`/`Base.Observe` 封装，ReActAgent 接入）

---

## P1 - 功能级缺失（生产可用性）

### 4. 消息块对齐 Python 2.0
- **目标**：统一多媒体块、调整 Tool 块字段，提升与 Python 版的跨语言兼容性。
- **关键行动**：
  - 引入 `DataBlock` 统一替代 `ImageBlock` / `AudioBlock` / `VideoBlock`
  - `ToolUseBlock.Input` 增加 `RawInput` 兼容路径（支持流式 JSON 累积）
  - `ToolResultBlock` 增加 `ID`/`Name`/`State` 字段
  - `message/json.go` 支持 `source` 嵌套结构序列化，并向后兼容旧版平铺格式
- **状态**：✅ 已完成

### 5. 扩展模型后端
- **目标**：新增 Anthropic、Gemini 等主流后端。
- **关键行动**：
  - `formatter/anthropic.go` + `model/anthropic/`：✅ 已完成（原生 HTTP + SSE）
  - `formatter/gemini.go`：✅ 已完成
  - `model/gemini/`：✅ 已完成（原生 HTTP + SSE，含基础测试）
- **状态**：✅ 已完成

### 6. ToolResponse 规范类型
- **目标**：替换工具返回的裸 `any`，支持多媒体结果。
- **关键交付物**：
  - `tool/response.go`：定义 `tool.Response` struct（含 `[]message.ContentBlock`）
  - 修改 `Tool.Execute` 签名返回 `*tool.Response`
  - ReActAgent/toolkit/executor/subagent/MCP 全部迁移
- **状态**：✅ 已完成

### 7. Memory 自动集成到 ReActAgent
- **目标**：在 `ReActAgent.buildHistory` 中自动调用 `memory.PreReasoningPrepare()`，实现上下文压缩。
- **状态**：✅ 已完成（`buildHistory` 自动检测 ReMeMemory 并调用 `PreReasoningPrepare`）

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

## P2 - 运行时与扩展（新增完成项）

### 8. 多模态工具封装
- **目标**：补齐 OpenAI / DashScope 多模态 API 的 Tool 层封装。
- **关键交付物**：
  - `tool/multimodal/openai.go`：`OpenAIMultiModalTool`（text_to_image / image_to_text）
  - `tool/multimodal/dashscope.go`：`DashScopeMultiModalTool`（text_to_image / image_to_text / text_to_video / image_to_video）
  - 异步轮询通用逻辑、base64 / data URL 下载辅助函数
- **状态**：✅ 已完成

### 9. 中断与优雅关闭策略
- **目标**：实现 `InterruptContext` 抽象和 `PartialReasoningPolicy`，让 ReActAgent 支持外部中断和系统关闭。
- **关键交付物**：
  - `interruption/`：`InterruptContext` + `InterruptSource`
  - `shutdown/`：`GracefulShutdownConfig` + `PartialReasoningPolicy`
  - `agent.Base`：原子中断标志 + `CheckInterrupted()` + `CreateInterruptContext()`
  - `agent/react`：循环检查点 + `handleInterrupt`（USER 恢复 / SYSTEM 保存或丢弃）
- **状态**：✅ 已完成

### 10. A2A 协议补全
- **目标**：补全 A2A 服务端与客户端的缺失能力。
- **关键交付物**：
  - `a2a/agent_adapter.go`：`AgentAdapter` 桥接 `agent.Agent`
  - `a2a/server.go`：`/task/cancel` 端点 + SSE 格式规范化
  - `a2a/http_client.go`：`WaitForTask` / `CancelTask`
- **状态**：✅ 已完成

### 11. 分布式 Agent 协调
- **目标**：基于 A2A 构建多节点 Agent 发现与任务分发。
- **关键交付物**：
  - `dist/registry.go`：内存注册表 + `Discover` + `AutoDiscover`
  - `dist/coordinator.go`：Random / RoundRobin / Broadcast 策略
- **状态**：✅ 已完成

### 12. 性能优化
- **目标**：提升 ReActAgent 在高并发工具场景下的吞吐。
- **关键交付物**：
  - `agent/react`：并发工具执行（`errgroup`）+ 结果保序组装
- **状态**：✅ 已完成

### 13. 更多 Hook 点
- **目标**：补齐经典 Hook 生命周期。
- **关键交付物**：
  - `hook/hook.go`：新增 `HookPreCall`
  - `agent/react`：在 `replyInternal` 中触发 `HookPreCall`
- **状态**：✅ 已完成

---

*完成一项请勾选或删除对应条目，保持本文件实时更新。*
