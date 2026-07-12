# agentic_memory — 自主长期记忆示例

演示 AgenticMemoryMiddleware：agent **自主管理**的文件式长期记忆。

## 与其他记忆的区别

| 机制 | 谁决定记什么 | 存储 |
|------|-------------|------|
| **ReMe / LongTermMemory (static)** | 系统自动检索写回 | 向量库（被动） |
| **LongTermMemory (agent)** | agent 调 search/add 工具 | 向量库 |
| **Agentic Memory**（本示例） | **agent 自己用文件工具写 Markdown** | 文件系统 |

"Agentic" 的核心：agent 凭注入的指令，用自身已有的 Write/Read/Edit 工具管理记忆文件，
中间件只负责把 MEMORY.md 索引注入系统提示词，并在推理时异步检索相关文件作为 HintBlock。

## 运行

```bash
go run ./examples/agentic_memory/
```

无需 API Key（使用关键词选择器）。

## 机制

1. **OnSystemPrompt**：追加记忆指令 + 有界 MEMORY.md 快照（agent 据此知道何时/如何存记忆）
2. **OnReply**：缓存用户查询，启动异步检索 goroutine
3. **OnReasoning**：检索完成后，将相关记忆文件内容注入为 HintBlock（仅一次）

## 生产使用

```go
store, _ := middleware.NewLocalMemoryStore("./Memory")
mw := middleware.NewAgenticMemoryMiddleware(store, "./Memory").
    WithSelector(llmSelector)  // 替换为 LLM 驱动选择器

agent, _ := react.Builder().
    Middleware(mw).
    // agent 需自带 Write/Read/Edit/Grep 工具来管理记忆文件
    Build()
```

记忆文件格式（frontmatter + 正文）：

```markdown
---
name: 记忆名
description: 一句话描述（用于未来判断相关性）
type: user|feedback|project|reference
---
记忆正文（feedback/project 类型建议含 Why: 与 How to apply:）
```
