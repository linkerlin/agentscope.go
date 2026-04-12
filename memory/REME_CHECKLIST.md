# ReMe 整合验收要点（对照实施计划 Phase 5）

- [ ] `go test ./...` 全绿；新增能力均有 `_test.go` 覆盖。
- [ ] 默认 `InMemoryMemory` / `WindowMemory` 行为未破坏。
- [ ] `ReMeFileMemory`：工作目录结构、`SaveTo`/`LoadFrom`、`PreReasoningPrepare` 与 `GetMemoryForPrompt` 路径可用。
- [ ] `ReMeVectorMemory`：`AddMemory` / `RetrieveMemory`、类型化 `Retrieve*`、`VectorWeight` 混合重排。
- [ ] `ReMeHook` + `InjectMessages` 与 `ReActAgent` 链式 Hook 协同正常。
- [ ] `config.AgentConfig.ReMe` 可序列化并与 `config.ReMeMemoryConfig` 字段一致。
- [ ] `memory.ReMeFileConfigFrom` 与 `ReMeVectorMemory.SaveTo`/`LoadFrom` 会持久化 `sessions/<id>.vector.json`。
- [ ] `Summarizer.AppendToMemoryMD` 写入 `memory/MEMORY.md`。
- [ ] 示例：`examples/reme/file`、`examples/reme/vector` 可编译运行（向量示例使用伪嵌入）。
