# ADR 0001: Memory Vector Stores Light Split

## Status
Accepted (P3 pilot, 2026-06)

## Context
The memory/ package is large (~81 files, 6k+ lines) mixing ReMe, vector stores, handlers, pipeline.

Per original review report, to improve navigation, independent testing of vector backends, future library replacement.

## Decision
- Extract shared types (MemoryNode, RetrieveOptions, MemoryType, VectorStore interface, EmbeddingContent) to memory/vector/types.go .
- Move vector store impls (local, chroma, qdrant, etc.) to memory/vector/ as subpackage (package vector).
- Parent memory/ provides aliases for backward compat (type MemoryNode = vector.MemoryNode etc, and New* via facade or qualified).
- vector sub is self-contained (duplicated small EmbeddingModel interface to avoid cycle).
- Update references via aliases or qualified in creation sites (reme, handler, tests).
- Keep facade in parent for stable API.

## Consequences
- Better modularity.
- No breaking for users of memory.VectorStore etc.
- Cycle avoided by type extraction and dupe.
- Pilot in memory/vector/pilot.md ; full dedup can follow.

## Alternatives
- Full reme/core/vector split (bigger).
- Registry only (less explicit).

See project全面审阅报告.md and memory/vector/pilot.md .
