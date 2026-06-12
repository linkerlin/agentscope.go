# memory/vector - 轻拆分试点 (per 原审阅报告)

本目录为 memory 模块轻量拆分试点起点。

## 建议结构（来自报告）
- memory/vector/ : 所有 VectorStore 实现 (local, pgvector, qdrant, es, chroma, remote, snapshot, raw)
- 父包 memory/ 保持 facade：type XXXVectorStore = vector.XXXVectorStore + NewXXX... 别名，保持 API 稳定。
- 共享类型 (MemoryNode, VectorStore interface, EmbeddingModel, RetrieveOptions 等) 保持在父包或进一步拆 base。
- 内部引用更新为 qualified "vector.XXX" 或通过 facade。

## 当前状态 (pilot)
- 已移动 vector_store_*.go 到此 (package vector, 类型限定 memory.*)。
- 父包 facade alias 已创建 (vector_store_local.go 等 thin files)。
- 问题：直接 import cycle (memory 导入 vector, vector 导入 memory for shared types)。
- 解决方案建议： 
  1. 先将 VectorStore interface + MemoryNode + 相关 helper 移到 memory/vector 或 memory/internal/base 。
  2. 或使用 registry 模式，子包通过 blank import 注册到父包 registry，无需父源文件 import 子。
  3. 渐进：先 vector stores, 然后 reme/ handler 等。

## 下一步
- 移动共享类型。
- 更新 reme_vector_memory.go, handler/bootstrap.go, tests 中的引用。
- go list -f '{{.Imports}}' ./memory/vector 检查。
- 保持 -race 测试绿。

此 pilot 展示了结构，完整实现需更多迭代（避免过度重构）。
