# AgentScope.Go ReMe Memory 后续 TODO

> 本文件记录 ReMe Memory 系统在 agentscope.go 中的剩余工作项，按优先级排序。
> 最后更新：2026-04-15

---

## P1 - 高优先级

（P1 已全部完成）

---

## P2 - 中优先级

### 4. 性能优化（并发、缓存）
- **目标**：
  - 为 `LocalVectorStore.Search` 引入读写锁优化或分片锁
  - 为 Embedding 调用增加本地缓存（LRU），避免重复文本的重复嵌入
  - 为异步摘要任务增加 goroutine 池或速率限制
- **影响文件**：`vector_store_local.go`、`embedding/`、`reme_file_memory.go`。

---

## P3 - 低优先级

### 6. 与 AgentScope-Java 功能对齐验证
- **目标**：对照 Java 版 AgentScope 的记忆接口与行为，验证 Go 版的一致性，编写跨语言对齐测试用例。

### 7. 配置系统深度整合
- **现状**：`config.ReMeMemoryConfig` 已定义，但尚未与 `ReMeVectorMemory` / `MemoryOrchestrator` 的构造函数打通。
- **目标**：实现 `config.BuildReMeVectorMemory(cfg config.ReMeMemoryConfig)` 工厂函数，一键从 YAML/JSON 配置初始化完整记忆系统。
- **影响文件**：`config/reme.go`。

### 8. 发布准备
- 完善 API 文档（godoc）
- 编写 CHANGELOG
- 发布前性能基准测试

---

*完成一项请勾选或删除对应条目，保持本文件实时更新。*
