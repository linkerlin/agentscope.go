# AgentScope.Go v2.6.0 性能快照

> 环境：Windows 11 · i9-13900HX · Go 1.25.0 · 2026-08-22 采集（`go test -bench=. -benchmem`，单次运行）
> 本文件是**数据快照**（绝对值）；运行方法与基线对比见 `docs/benchmark.md` 与 Makefile `bench-*` 目标。

---

## 1. Gateway（HTTP 服务面）

| 基准 | ns/op | B/op | allocs/op | 折算 |
|------|-------|------|-----------|------|
| Gateway_Chat | 6,785 | 7,999 | 49 | ~147k req/s |
| Gateway_ChatStream | 13,538 | 8,079 | 56 | ~74k req/s |
| Gateway_RealServer（真 TCP） | 369,191 | 22,277 | 176 | ~2.7k req/s |
| Gateway_SessionCreate 并发 | 1,558 | 327 | 3 | ~641k/s |

### 并发扩展性（Chat，每 worker 延迟）

| 并发 | ns/op | 结论 |
|------|-------|------|
| 1 | 9,489 | — |
| 10 | 11,921 | +26% |
| 50 | 11,584 | +22% |
| 100 | 12,017 | +27% |

**1→100 并发每请求延迟仅上升 ~27%**，无锁竞争悬崖——会话串行锁 + 无共享热路径的设计在高并发下线性扩展。Stream 并发（1→50）同样平稳（12.5→13.2µs）。

## 2. ReAct 事件流（agent 核心）

| 基准 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| ReplyStream_TextOnly | 4.40ms | 2.1MB | 67 |
| ReplyStream_WithTools | 4.48ms | 2.1MB | 83 |

（mock 模型全事件流；含 thinking/tool_call/tool_result 事件组装。分配主要来自事件结构与消息快照。）

## 3. Formatter（消息格式化）

| 基准 | ns/op | 说明 |
|------|-------|------|
| OpenAI 10 msgs | 1,965 | ~196ns/条 |
| OpenAI 50 msgs | 14,418 | 线性扩展 |
| DashScope 10 msgs | 3,944 | OpenAI 别名，2× 差异来自路径开销 |
| Gemini 10 contents | 10,345 | 结构转换较重 |
| Anthropic 10 msgs | 18,633 | 最重（细粒度 block 映射） |
| FormatTools 20 | 2,066 | 线性 |

## 4. 编排（Pipeline / DAG / A2A）

| 基准 | ns/op | 说明 |
|------|-------|------|
| Pipeline_Sequential（3 步 mock） | 3.81ms | 受 mock 延迟主导 |
| DAGExecutor_Parallel | 73,182 | Kahn 拓扑 + 并行调度 |
| DAGExecutor_LargeFanOut | 144,593 | 大扇出仍亚毫秒 |
| ValidateDAG | 62,552 | 图校验 |
| A2ATool_ExecuteSync | 811 | A2A 委托开销极低 |
| A2A RateLimiter_Allow | 48.9 | **零分配**限流 |

## 5. 记忆与检索（memory）

| 基准 | ns/op | 说明 |
|------|-------|------|
| EmbeddingCacheHit | 505 | 文件缓存命中 4 allocs |
| VectorStoreLocal_Insert（100 维×N） | 435µs | 内存向量插入 |
| FTSIndexSearch | 49,684 | FTS5 检索 |
| RankMemoryNodesHybrid | 67,428 | BM25+向量混合排序 |
| ReMeFileMemoryAdd | 151,559 | 文件记忆写（含磁盘） |

## 6. 工具链（toolkit / a2a 中间件）

| 基准 | ns/op | allocs |
|------|-------|--------|
| RegistryToolSpecs | 148 | 2 |
| a2a AuthMiddleware | 1,026 | 8 |
| a2a Registry_AllTools | 7,855 | 12 |

---

## 结论

1. **服务面吞吐**：单进程 HTTP chat ~147k req/s（内存管线）、真实 TCP ~2.7k req/s（含网络栈）；会话创建 64 万/s。
2. **并发扩展性**：1→100 并发延迟仅 +27%，无串行瓶颈——多租户网关可直接横向叠加。
3. **事件流成本**：一次全事件 ReplyStream ~4.4ms / 2.1MB（事件驱动架构的固定成本，可接受）。
4. **Formatter 线性**：消息数线性扩展，无 O(n²)（对齐 Python #2158 修复语义）。
5. **热路径零/低分配**：限流 0 allocs、注册表 2 allocs、缓存命中 4 allocs——GC 压力集中在事件组装，不在基础设施。

## 复现

```bash
go test "-bench=." -benchmem -run=NONE ./gateway/ ./agent/react/ ./formatter/ ./pipeline/ ./plan/ ./toolkit/ ./tool/a2a/ ./a2a/ ./memory/
# 基线对比（需 benchstat）:
make bench-save && make bench-compare
```
