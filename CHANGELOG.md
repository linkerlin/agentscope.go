# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Formatter Abstraction Layer**: Introduced a dedicated `Formatter` interface (`formatter/formatter.go`) decoupling message-to-API conversion from model implementations.
  - `OpenAIFormatter`, `AnthropicFormatter`, `GeminiFormatter`, `DashScopeFormatter`, `OllamaFormatter`.
  - DashScope and Ollama inject their own formatters into the OpenAI-compatible backend for future backend-specific customization.
- **New Model Backends**:
  - **Anthropic** (`model/anthropic/`): native HTTP client with SSE streaming support.
  - **Gemini** (`model/gemini/`): native HTTP client with SSE streaming support.
- **Multi-Agent Orchestration**:
  - **Pipeline** (`pipeline/`): sequential agent chaining where output of step N becomes input of step N+1.
  - **MsgHub** (`msghub/`): lightweight message hub for registering and broadcasting messages to multiple agents concurrently.
  - **Workflow** (`workflow/`): advanced orchestration primitives:
    - `Parallel` — concurrent execution with configurable result joining.
    - `Condition` — branch selection based on an evaluator function.
    - `Loop` — repeated execution with a max-iteration safeguard.
- **A2A Protocol Stack**:
  - Server-side SSE streaming via `/task/sendSubscribe`.
  - `HTTPClient` implementing the `Client` interface with `Send` and `SendSubscribe`.
- **AgentBase Lifecycle Unification** (`agent/base.go`):
  - `Base` struct centralizes shared lifecycle, hooks, state management, and usage tracking.
  - `ReActAgent` now embeds `Base` and uses `Call()` / `Observe()` wrappers.
  - Auto-fires `HookPreReply` / `HookPostReply` / `HookPreObserve` / `HookPostObserve`.
- **ToolResponse Standardization** (`tool/response.go`):
  - Replaced bare `any` returns with `*tool.Response` (carrying `[]message.ContentBlock`).
  - Migrated `toolkit/executor`, `SubagentTool`, `MCP adapter`, `plan_notebook`, and examples.
- **Memory Auto-Integration**:
  - `ReActAgent.buildHistory()` automatically detects `PreReasoningPrepare` on memory (ReMe) and injects compressed summaries before model calls.
- **Message Blocks Aligned to Python 2.0**:
  - `DataBlock` with nested `Source` struct (URL / base64 / media_type) unifies multimedia handling.
  - `ToolUseBlock.RawInput` supports streaming JSON accumulation.
  - `ToolResultBlock` gains `ID`, `Name`, and `State` fields.
  - `message/json.go` supports `source` serialization and backward-compatible deserialization.
- **BM25/FTS5 Full-Text Search**: Integrated `modernc.org/sqlite` (pure Go, no CGO) with FTS5 virtual tables for real BM25 ranking. `ReMeVectorMemory` automatically syncs the FTS index on `AddMemory`/`DeleteMemory`, and `RankMemoryNodesHybrid` fuses BM25 scores with vector cosine similarity.
- **Multi-Backend VectorStore**:
  - `QdrantVectorStore` using the official `qdrant/go-client` (gRPC).
  - `ChromaVectorStore` using a lightweight `net/http` REST client.
  - `ESVectorStore` using Elasticsearch 8.x native kNN search.
  - `PGVectorStore` using `pgx` + `pgvector-go` with HNSW/IVFFlat index support.
- **ToolMemory Auto-Trigger Closed Loop**:
  - `ReActAgent` now collects `ToolCallResult` after each tool execution.
  - `MemoryOrchestrator` buffers results and automatically invokes `ToolSummarizer.SummarizeToolUsage`, persisting generated tool guides into the vector store.
- **ReMeInMemoryMemory Abstraction**: Extracted `ReMeInMemoryMemory` as a standalone struct responsible for in-memory message buffering, mark tracking, compressed/long-term memory, and dialog file appending. `ReMeFileMemory` now composes it.
- **Performance Optimizations**:
  - `EmbeddingCache`: LRU cache wrapper for `EmbeddingModel` with hit/miss statistics, supporting both single and batch embedding deduplication.
  - `ReMeFileMemory` async summarization now uses a semaphore-backed worker limit (default 4 concurrent tasks) to prevent goroutine explosion.
- **Configuration Factory**: Added `handler.BuildReMeVectorMemory(cfg *config.ReMeMemoryConfig, embed, cm)` to build a fully wired `ReMeVectorMemory` (including vector store selection and optional `MemoryOrchestrator`) from a single configuration object.

### Changed

- `ReMeVectorMemory.store` field changed from concrete `*LocalVectorStore` to `VectorStore` interface, enabling transparent swapping of vector backends.
- `DeduplicateAgainstStore` signature updated to accept `VectorStore` interface instead of `*LocalVectorStore`.
- `Orchestrator` and `VectorMemory` interfaces extended with `AddToolCallResult` and `SummarizeToolUsage` methods.
- `config.ReMeMemoryConfig` expanded with remote vector store connection fields (`Host`, `Port`, `Collection`, `BaseURL`, `Index`, `ConnStr`, `Table`) and `Language`.

### Fixed

- **OpenAI Streaming Panic**: Added nil check for `resp.Usage` in `model/openai/openai.go` `chatStreamOnce` to prevent nil pointer dereference on intermediate stream chunks.
- **A2A Data Race**: Protected `InMemoryTaskStore` with `sync.RWMutex` and returned shallow copies to eliminate races between HTTP handler and async runner goroutines.
- Resolved Windows temp-directory cleanup failures in memory tests by ensuring `ReMeFileMemory.Close()` / `ReMeVectorMemory.Close()` is always deferred in tests, preventing open SQLite handles from locking `reme.db`.

### Benchmarks (baseline on Intel i9-13900HX)

| Benchmark | ops/s | ns/op |
|-----------|-------|-------|
| `BenchmarkEmbeddingCacheHit` | ~2.7M | 454 |
| `BenchmarkEmbeddingCacheMiss` | ~16K | 60,171 |
| `BenchmarkLocalVectorStoreSearch` (50 docs) | ~71K | 14,096 |
| `BenchmarkFTSIndexSearch` (100 docs) | ~1.6K | 623,041 |
| `BenchmarkRankMemoryNodesHybrid` (20 docs) | ~1K | 990,777 |
| `BenchmarkReMeFileMemoryAdd` | ~2.4K | 411,857 |
| `BenchmarkWindowMemoryAdd` | ~147K | 6,777 |

## [0.1.0] - 2026-04-14

### Added

- **ReMe Memory System**: Full implementation of `ReMeFileMemory` and `ReMeVectorMemory` with file-based persistence, vector CRUD, and hybrid retrieval.
- **Orchestrator Layer**: `MemoryOrchestrator` coordinating `PersonalSummarizer`, `ProceduralSummarizer`, `ToolSummarizer`, `MemoryHandler`, `ProfileHandler`, and `HistoryHandler`.
- **Async Summarization**: `AddAsyncSummaryTask` / `AwaitSummaryTasks` for non-blocking daily memory summarization.
- **Hook Integration**: `ReMeHook` injects `PreReasoningPrepare` into the agent loop via `HookBeforeModel`.
- **Vector Snapshot Persistence**: `LocalVectorStore` supports JSON snapshot round-trip for session recovery.
