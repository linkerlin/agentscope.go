# 消息总线 (Message Bus)

AgentScope.Go 的消息总线提供**领域无关**的协调原语，是多进程 / 分布式部署的统一骨干。
对齐 Python AgentScope 的 `app/message_bus`（#1849 重构后的通用原语集）。

## 三层接口

总线能力分三层，均为**可选接口**（按需实现，`AsXxx` 类型断言获取）：

| 接口 | 能力 | 用途 |
|------|------|------|
| `Bus` | Publish / Subscribe / Close | 事件广播（pub/sub） |
| `TeamBus` | InboxPush/Drain + EnqueueWakeup/SubscribeWakeup | Agent 团队异步协作 |
| `CoordBus` | Lock / Registry / Queue / Log | 通用分布式协调原语 |

## 后端

| 后端 | 适用场景 | 实现 |
|------|---------|------|
| `LocalBus` | 单进程、零依赖 | 纯 stdlib（channel + sync） |
| `RedisBus` | 多进程 / 分布式 | Redis（pub/sub + Stream + HASH + LIST + SET NX） |

两者均实现全部三层接口。`NewLocalBus()` / `NewRedisBus(client, prefix)`。

## CoordBus 四原语

```go
cb := messagebus.AsCoordBus(bus)  // nil if bus 不支持协调原语
```

### Lock — 分布式锁
```go
release, err := cb.Lock(ctx, messagebus.Keys.LockKey("resource"), 30*time.Second)
defer release()  // 或等 TTL 自动释放
```
- LocalBus：capacity-1 channel 信号量 + `time.AfterFunc` TTL 自动释放 + ctx 可取消阻塞
- RedisBus：`SET key token NX PX ttl` + Lua 脚本 token 校验释放（防止误释放他人锁）+ 50ms 轮询

### Registry — 键值注册表
```go
cb.RegistrySet(ctx, Keys.RegistryNS("tenants"), "t1", []byte("config"))
v, _ := cb.RegistryGet(ctx, Keys.RegistryNS("tenants"), "t1")
all, _ := cb.RegistryList(ctx, Keys.RegistryNS("tenants"))
cb.RegistryDelete(ctx, Keys.RegistryNS("tenants"), "t1")
```
- LocalBus：`map[ns]map[key][]byte`
- RedisBus：`HSET/HGET/HGETALL/HDEL`

### Queue — FIFO 队列
```go
cb.QueuePush(ctx, Keys.QueueName("index-jobs"), []byte("task-1"))
v, _ := cb.QueuePop(ctx, Keys.QueueName("index-jobs"))  // 阻塞直到有值或 ctx 取消
```
- LocalBus：slice + notify channel（ctx 可取消阻塞）
- RedisBus：`RPUSH` + `BLPOP`（FIFO，2s 轮询 + ctx 取消）

### Log — 追加日志
```go
idx, _ := cb.LogAppend(ctx, Keys.LogNS("audit"), []byte("event"))
entries, nextCursor, _ := cb.LogRead(ctx, Keys.LogNS("audit"), 0, 100)
```
- LocalBus：slice，index = 位置
- RedisBus：`RPUSH`（返回长度）+ `LRANGE` 游标分页

## 业务键约定

`messagebus.Keys`（`CoordKeys`）集中管理键格式，避免散落字符串：

| 方法 | 格式 | 示例 |
|------|------|------|
| `Keys.QueueName(x)` | `as:queue:x` | 任务队列 |
| `Keys.LockKey(x)` | `as:lock:x` | 资源锁 |
| `Keys.RegistryNS(x)` | `as:reg:x` | 注册表命名空间 |
| `Keys.LogNS(x)` | `as:log:x` | 日志流 |
| `Keys.ProjectionNS(sid)` | `as:projection:sid` | 跨会话投影 |

`WakeupKind`：`wake`（唤醒空闲会话排空 inbox）/ `resume`（恢复挂起会话喂入 HITL 结果）。

## 跨会话投影 (SessionProjection)

一个会话可向另一会话投影 UI 卡片（如 worker 的 HITL 请求投影到 leader）：

```go
sp := gateway.NewSessionProjection(bus)
sp.Project(ctx, "leader-session", "hitl-worker-a", []byte(`{"prompt":"approve?"}`))
cards, _ := sp.List(ctx, "leader-session")
```
HTTP：`GET /api/v1/sessions/{id}/projections`、`DELETE /api/v1/sessions/{id}/projections/{key}`。
无 CoordBus 时优雅降级（no-op）。

## 优雅降级

所有 CoordBus/TeamBus 消费者都应通过 `AsCoordBus`/`AsTeamBus` 获取，nil 时降级为单进程行为，
确保 LocalBus-only 部署无需额外配置即可工作。

## 多进程部署

```go
client := redis.NewClient(&redis.Options{Addr: "redis:6379"})
bus := messagebus.NewRedisBus(client, "myapp")
srv := gateway.NewApp(gateway.AppConfig{
    Storage:    redisStorage,
    MessageBus: bus,
    // ...
})
```
多个 gateway 进程共享同一 Redis，自动获得：跨进程取消、唤醒、工具卸载完成通知、分布式锁、共享队列。
