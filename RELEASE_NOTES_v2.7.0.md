# AgentScope.Go v2.7.0 Release Notes

> 🛡️ **AgentScope.Go v2.7.0** —— 可靠多副本与交付门禁。v2.6.0 建立了可信边界（认证、租户、流式透明性）；v2.7 把多副本从"协调存在"推进到"故障下正确"——fencing 租约、可恢复 HITL、保留清理、长连接 worker，并以四道自动化门禁（文档、覆盖率、依赖、导出 API）守住交付质量。
>
> 本版对应演进方案 Phase 23 全部七项 + 认证热路径索引 + 18.5 长连接 worker。发布链由 `scripts/check_release.sh` 与 CI `release-consistency` job 校验（22.5 首次完整实战）。

---

## 主要更新

### 1. 可续租会话租约（fencing，23.1）

固定 TTL 锁无法区分"持有者活着"与"持有者卡死"。v2.7 引入短 TTL fencing 租约：

- `messagebus.CoordLease`：非阻塞 `AcquireLease`（`ErrLeaseHeld`）、token CAS `RenewLease`（`ErrLeaseLost`）；Redis 获取经 Lua 一步原子完成 `INCR fence + SET NX PX owner|token`，接管方 token 恒更大，过期持有者的释放/续租均 token-gated 无害。
- `SessionCoordinator` 租约路径：15s TTL、TTL/3 续租 pump（detached context，SSE 断连不失锁）、**失租立即 Terminate 本地 run**（继续推进会双执行外部副作用）；running marker 带 owner/fence/expires，结束时 CAS 只删自己的 marker。
- 无租约能力的 bus 自动降级原 30min Lock 路径，单副本行为不变。

### 2. 可恢复 HITL（23.2）

人类确认不再依赖单个进程的运行态：

- `AgentSnapshot.PendingResume`：版本化、幂等的 resume 命令，与快照同一条存储记录（天然原子）。首次确认**先持久化再投递**——注入后崩溃留下可审计的 executing 命令。
- 重复确认幂等：`executing` 状态的重复确认被拒绝（HTTP 409 `{"status":"executing"}`），被恢复的工具**至多执行一次**。
- 投递不可达（run 挂在其他副本或已死）→ 409 `{"status":"pending"}`，命令留持久层待持有副本消费。
- 回合完成才删除快照（挂点：`ReplyEndEvent`）；删除点后移使崩溃窗口全程可审计。

### 3. 长连接 worker（18.5）

渠道监听、wakeup 消费、定时任务从 API 副本拆出，多副本不重复消费：

- **角色租约主循环**（`gateway/worker.go`）：每角色独立 acquire→hold→lose→re-acquire——非阻塞抢角色（`ErrLeaseHeld` 指数退避 500ms→30s 待命）、TTL/3 续租即心跳兼对账（续租失败=fence 被超越→立即停组件）、持有者死后一个 TTL 内 standby 接管。
- **三入口协调收口**：`WakeupDispatcher.WithRun` / `ChannelRunner.Run` / `BackgroundTaskManager.WithSessionRun` 注入 `SessionCoordinator.Run`——HTTP、channel、cron、wakeup 四类 turn 来源共享同一套 23.1 会话租约，双副本不重复消费。wakeup 遇 busy 有界重试后回推 inbox + 重排 wakeup（at-least-once，不丢消息）。
- **resume 跨副本消费闭环**：resume 确认落到无 waiter 的副本 → 409 pending + wakeup 信号 → worker 副本重建 agent、LoadState 恢复挂起 turn、租约证明原持有者已死后投递持久化命令、完成才删快照。
- **拆分部署**：`Server.WithWorkerRoles()`（无参 = 纯 API 副本）与 `AppConfig.Worker.Roles`（nil=全部/显式空=无）；默认不配置保持单进程全功能。

### 4. 会话保留与清理（23.3）

长时运行不再无界积累：completed buffer 上限（默认 1024 条 / 1h TTL）、协调事件日志摊销 trim（峰值 ≤2×cap，旧 cursor 得 `ErrLogCursorStale` 哨兵而非错读）、`StartReaper` 周期清理（过期 run marker 经 CAS 删除，不碰接管方）、`DELETE /api/v1/sessions/{id}` 联动清 running marker / 事件日志 / completed buffer。

### 5. 认证热路径哈希索引

`FindUserByAPIKey` 原为 O(用户×凭据) 线性扫描，生产规模不可用。新增 `service.APIKeyCredentialFinder` 可选接口：SQL 迁移 0002 加 `credentials.key_hash` 列 + 索引 + 存量回填（仅 `sha256:` 形态行；legacy 明文行不回填 = fail-closed），Redis/Memory 反向索引同步维护，轮换与删除自动失效。无效 key 现在 O(1) 拒绝；逐候选仍做常数时间终验，索引污染不能误认证。顺带修复 `SQLStorage.ListSessionsBySchedule` 引用不存在列导致 SQL 后端下必然报错的真 bug。

### 6. 存储迁移与方言契约（23.4）

Postgres 升级为受支持且被真实测试的方言：迁移方言白名单（未知引擎拒绝启动而非跑错 SQL）、`pg_advisory_lock` 串行化并发启动（双副本 Migrate 全部成功且每条恰好记录一次）、`SQLStorage` 占位符自适应（`?`→`$n`）、PG 门控测试（`TEST_POSTGRES_DSN`）跑真实 CRUD + 级联 + secret 往返契约。

### 7. 交付门禁四件（23.5–23.7）

| 门禁 | 内容 | CI job |
|------|------|--------|
| 文档契约 | 假 API 清剿（12 处编造签名替换为真实 API）+ docs/docs-site 镜像字节同步 + 33 处构造引用存在性 + 片段可编译可执行 | `docs-consistency` |
| 覆盖率与示例 | atomic 基线（总量 60.3%）+ 7 关键包非回退（0.5pp 容差）；示例孤儿/死链/限时编译 | `quality-gates` |
| 依赖更新 | Dependabot gomod + github-actions weekly（GO-2026-6222 红 4 天无人发现的对策） | — |
| 导出 API 兼容 | `internal/apidump` 4078 符号清单 + git worktree diff + 精确白名单（`quality/api_breaking_allowlist.txt`）；实测 vs v2.6.0 零破坏 | `api-compat` |

另：验收纪律第 2 条固化——**CI 绿是验收的一部分**（22.x 教训：本地 macOS 全绿而 CI 四 job 红）。

---

## Breaking Changes 与迁移

| 变更 | 影响 | 迁移 |
|------|------|------|
| 登录只凭持有证明 | `POST /api/v1/auth/login` 只接受 `{"api_key"}`；user_id 换 token 废除 | 客户端改用注册响应中的 API key 登录 |
| 旧明文 API key 失效 | 仅 `sha256:` 哈希形态凭据可认证，明文行 fail-closed | 重新注册或重置 key |
| 未知 session ID 一律 404 | storage 模式下客户端自造 ID 被拒（服务端铸造） | 从响应头 `Agent-Session-Id` 采纳服务端 ID |
| 跨用户资源默认拒绝 | AgentConfig 归属校验，`AccessPolicy` nil = DenyAll | 共享需显式配置策略 |

导出 API 兼容性：`make api-diff`（对照 v2.6.0）零 breaking、纯新增。

---

## 升级

```bash
go get github.com/linkerlin/agentscope.go@v2.7.0
```

存储后端升级即开即用：SQL 首次启动自动跑迁移 0002（`credentials.key_hash` 回填）；Redis 无需动作；无 CoordLease 能力的 bus 保持原行为。

---

## 致谢

感谢所有通过 issue 与 review 参与本项目的贡献者。push 与 GitHub Release 由维护者执行；本地验证链：`make ci`（fmt/vet/build/test/release-consistency/examples-check）+ `make docs-check` + `make api-diff`。
