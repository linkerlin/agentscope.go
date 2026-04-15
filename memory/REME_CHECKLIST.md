# ReMe 整合验收要点（对照实施计划 Phase 5）

- [x] `go test ./...` 全绿；memory/config/hook/observability/a2a 等已补充 `_test.go`（memory 语句覆盖率约 79%+）。
- [x] 默认 `InMemoryMemory` / `WindowMemory` 行为未破坏。
- [x] `ReMeFileMemory`：工作目录结构、`SaveTo`/`LoadFrom`、`PreReasoningPrepare` 与 `GetMemoryForPrompt` 路径可用。
- [x] `ReMeVectorMemory`：`AddMemory` / `RetrieveMemory`、类型化 `Retrieve*`、`VectorWeight` 混合重排可用。
- [x] `ReMeHook` + `InjectMessages` 与 `ReActAgent` 链式 Hook 协同正常。
- [x] `config.AgentConfig.ReMe` 可序列化并与 `config.ReMeMemoryConfig` 字段一致。
- [x] `memory.ReMeFileConfigFrom` 与 `ReMeVectorMemory.SaveTo`/`LoadFrom` 会持久化 `sessions/<id>.vector.json`。
- [x] `Summarizer.AppendToMemoryMD` 写入 `memory/MEMORY.md`。
- [x] 示例：`examples/reme/file`、`examples/reme/vector`、`examples/reme/orchestrator` 可编译运行。
- [x] **新增 Orchestrator 层**：`MemoryOrchestrator`（Summarize + Retrieve）、`MemoryHandler`、`ProfileHandler`、`HistoryHandler` 已集成到 `ReMeVectorMemory`。
- [x] **异步摘要**：`ReMeFileMemory.AddAsyncSummaryTask` / `AwaitSummaryTasks` 已可用，并在 `PreReasoningPrepare` 压缩后自动触发。
