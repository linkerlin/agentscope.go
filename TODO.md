# AgentScope.Go 后续 TODO

> 最后更新：2026-06-10

---

## 进行中 / 剩余工作项

### P2 — 中优先级（功能补齐）

#### 1. `tool/schedule/` — 测试补齐
- **状态**：✅ 代码已实现，❌ 零测试
- **说明**：`ScheduleCreate/List/Stop/View` 四个工具无测试文件

#### 2. `workspace/docker.go` — Dockerfile 生成增强
- **状态**：✅ 基础实现完成
- **说明**：已支持 `RenderDockerfile` / `BuildImage` / `HealthCheck`

#### 3. Gateway 端到端测试覆盖
- **状态**：5+ 基础测试通过
- **说明**：需增加多租户 SSE/WS 场景的集成测试

### P3 — 低优先级（前瞻 / 工程优化）

#### 4. Benchmark 持续完善
- **状态**：✅ 骨架已有（reply_stream / pipeline / gateway）
- **说明**：持续维护核心路径基准

#### 5. 代码拆分优化
- **状态**：`agent/react/` 文件较大（~1,984 行）
- **说明**：可考虑按 reasoning / acting / tools 维度进一步拆分

#### 6. `memory/` 子模块化
- **状态**：81 个 Go 文件
- **说明**：考虑将旧的向量后端实现拆为独立子模块

---

*完成一项请勾选或更新对应条目状态，保持本文件实时更新。*
