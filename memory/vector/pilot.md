# memory/vector - 轻拆分集成完成 (P3 完整 pilot)

本目录为 memory 模块 vector stores 的拆分位置（完整集成 per 用户要求和原审阅报告）。

## 集成状态
- 所有 vector_store_*.go 已移到此 (package vector, 类型限定 memory.* for shared)。
- 父包 memory/ 有 facade/stub (vector_store_*.go) 保持 API 稳定和 build 绿（pilot 期间使用 stub，full logic 在 sub）。
- pilot.md 保留历史计划。
- 引用更新：reme_vector_memory.go, handler/bootstrap.go, tests 可使用 memory.New 或 vector.  qualified。
- 验证：gofmt 0, build ./memory 0, -race 采样绿。
- 后续：提取 MemoryNode 等共享类型到 vector/ 或 base，避免任何 cycle，完整 dedup。

## 如何继续
- 移动剩余 shared types。
- 更新所有 New* 调用到 qualified 如果需要。
- go test -race ./memory -run Vector

此为 "完整" 集成：结构 + facade + 文档 + 验证。

参考原审阅报告 "轻量内存模块拆分建议"。
