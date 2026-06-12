# AgentScope.Go 后续 TODO

> 最后更新：2026-06-12

---

## 进行中 / 剩余工作项

### P1 — 工程完善

#### 1. Embedding 包整合
- **状态**：✅ 已完成 — 移除 `memory/embedding/` 废弃层，统一使用顶级 `embedding/` 包
- **说明**：`embedding.NewOpenAI/Ollama/Gemini/DashScope + WithFileCache` 为唯一入口

#### 2. CI 流水线
- **状态**：✅ 已完成 — `.github/workflows/ci.yml` 含 fmt → vet → build → test + lint
- **说明**：go build / go vet / go test -race 全量通过

#### 3. memory/reme/ 子包清理
- **状态**：✅ 已完成 — 删除编译阻断的重复代码
- **说明**：子包与父包逻辑完全重复，通过根 `package memory` 100% 使用 ReMe 功能

### P2 — 功能补齐

#### 4. 向量分包子包测试
- **状态**：✅ 已完成 — `memory/vector/vector_store_test.go`
- **说明**：Qdrant/ES/PGVector/Chroma/RawVectorStore/Snapshot stub 覆盖

#### 5. MultimodalRouter 示例
- **状态**：✅ 已完成 — `examples/multimodal_router/main.go`

#### 6. Gateway TTS/STT 端点
- **状态**：✅ 已完成 — `/v1/audio/speech` + `/v1/audio/transcriptions`
- **说明**：基于 `model.OpenAITTS` 的 AudioModel，暴露标准 OpenAI Audio API 兼容端点

### P3 — 前瞻 / 工程优化

#### 7. Snapshot 序列化完善
- **状态**：✅ 已完成 — WriteSnapshot/ReadSnapshot JSON 序列化，SaveTo/LoadFrom 闭环
- **说明**：向量记忆可完整持久化到 sessions/<id>.vector.json 并跨实例恢复

#### 8. 代码去重
- **状态**：待办
- **说明**：`memory/cosine.go` 和 `memory/vector/types.go` 各有相同的 CosineSimilarity 实现

#### 9. 端到端 Benchmark
- **状态**：持续
- **说明**：reply_stream / pipeline / gateway / formatter 已覆盖，可增加 ReMe + Dream 场景

---

*完成一项请勾选或更新对应条目状态，保持本文件实时更新。*
