# AgentScope.Go 后续 TODO

> 最后更新：2026-06-10

---

## 进行中 / 剩余工作项

### P2 — 中优先级（功能补齐）

#### 1. `tool/schedule/` — 测试补齐
- **状态**：✅ 已完成（18 个测试，全量通过）
- **说明**：`ScheduleCreate/List/Stop/View` 四个工具 + 辅助函数全覆盖

#### 2. `workspace/docker.go` — Dockerfile 生成增强
- **状态**：✅ 基础实现完成
- **说明**：已支持 `RenderDockerfile` / `BuildImage` / `HealthCheck`

#### 3. Gateway 端到端测试覆盖
- **状态**：✅ 已完成（`gateway/e2e_integration_test.go`，7 个测试，26 个子案例）
- **说明**：覆盖全认证流程（注册→登录→JWT访问）、SSE+Auth、Streamable HTTP 全生命周期（POST/DELETE/GET订阅）、AG-UI+Auth、多会话隔离、认证边界

### P3 — 低优先级（前瞻 / 工程优化）

#### 4. Benchmark 持续完善
- **状态**：✅ 持续维护（reply_stream / pipeline / gateway / formatter）
- **最新结果**（i9-13900HX）：
  - OpenAI FormatMessages(1 msg): 224 ns/op, 352 B/op, 3 allocs/op
  - OpenAI FormatMessages(50 msg): 15.3 µs/op, 17 KB/op, 101 allocs/op
  - Anthropic FormatMessages(10 msg): 9.8 µs/op, 7.6 KB/op
  - Gemini FormatContents(10 msg): 4.3 µs/op, 7.6 KB/op
  - ThinkingBlock提取（无标签）: 42 ns/op, 0 B/op
  - ThinkingBlock提取（含标签）: 4.3 µs/op, 650 B/op

#### 5. 代码拆分优化
- **状态**：✅ 结构良好（22 个文件，最大 968 行）
- **说明**：`react_agent.go`(968行) / `reply_stream.go`(818行) 已合理拆分

#### 6. `memory/` 子模块化
- **状态**：81 个 Go 文件
- **说明**：考虑将旧的向量后端实现拆为独立子模块

#### 7. `tool/schedule/` 独立 Manager
- **状态**：✅ 已完成（`manager.go` + `manager_test.go`，5 个测试）
- **说明**：`StandardManager` 包装 `*schedule.Scheduler`，无 Gateway 依赖即可使用

#### 8. V2 事件流示例
- **状态**：✅ 已完成（`examples/v2_event_stream/main.go`）
- **说明**：演示 `ReplyStream()` 完整事件生命周期

#### 9. Web Fetch 工具
- **状态**：✅ 已完成（`tool/web/fetch.go` + `fetch_test.go`，10 个测试）
- **说明**：HTTP GET 工具，支持超时、Content-Type、最大长度控制、context 取消

#### 10. JSON 工具
- **状态**：✅ 已完成（`tool/json/tool.go` + `tool_test.go`，14 个测试）
- **说明**：`json_parse`（格式化 + 类型识别）、`json_query`（dot-separated 路径查询）

#### 11. 工具注册便捷函数
- **状态**：✅ 已完成（`tool/file/register.go` + `register_test.go`，3 个测试）
- **说明**：`file.RegisterAll(baseDir, readOnly)` 一键注册 3~6 个文件工具

#### 12. Gateway 健康检查端点
- **状态**：✅ 已完成（`/health` + 3 个测试）
- **说明**：返回 JSON 状态（version/storage/auth/active_sessions），支持 K8s 探针

#### 13. Gateway Request ID 注入
- **状态**：✅ 已完成（`X-Request-ID` 自动生成/透传 + 1 个测试）
- **说明**：自动为所有请求注入 `X-Request-ID` 响应头，支持透传客户端 ID

#### 14. 生产级全功能示例
- **状态**：✅ 已完成（`examples/production/main.go`）
- **说明**：认证 + JSON/WebFetch 工具 + 权限引擎 + Gateway + 健康检查，一键启动

---

*完成一项请勾选或更新对应条目状态，保持本文件实时更新。*
