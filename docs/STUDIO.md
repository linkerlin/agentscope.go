# Studio 使用指南

AgentScope.Go 提供轻量级、纯 Go 实现的 Studio UI，用于在浏览器中管理 Agent、凭证、会话、定时任务和实时聊天。

---

## 1. 启动 Studio

Studio 位于 `examples/studio`，基于 `gateway.NewApp` 实现自动装配：

```bash
cd examples/studio
go run .
```

默认监听 `:8080`。打开浏览器访问 `http://localhost:8080`。

### 自动装配能力

Studio 示例默认启用：

- **Storage**：内存存储（演示用）
- **Workspace**：`./workspaces`
- **AutoStandardTools**：自动注入 file / task / web / json / schedule 工具
- **AutoToolOffload**：长耗时工具后台化
- **EmbeddingCache**：Embedding 文件缓存
- **Demo Register**：无需 JWT 即可快速注册测试账号

---

## 2. 页面说明

| 页面 | 路径 | 功能 |
|------|------|------|
| 首页 | `/` | 系统概览、自动装配效果演示 |
| Agents | `/agents` | 创建、列出、删除 Agent |
| Schedules | `/schedules` | 创建、列出 Cron 调度任务 |
| Chat | `/chat` | 实时 SSE 对话、查看 auto tools 调用结果 |

---

## 3. 快速体验

1. 打开首页，点击 **Demo Register** 自动注册测试用户
2. 进入 **Agents** 页面，创建 Agent（默认已带 StandardTools）
3. 点击 **Use in Chat** 设置当前聊天 Agent
4. 进入 **Chat** 页面，发送消息，观察：
   - 实时文本增量（SSE）
   - `[AUTO TOOL] xxx started` 工具调用提示
   - `[AUTO TOOL RESULT] ...` 工具执行结果

---

## 4. 自定义 Studio

### 修改端口

编辑 `examples/studio/main.go`：

```go
log.Fatal(http.ListenAndServe(":8080", srv))
```

### 切换存储

将 `MemoryStorage` 替换为 `RedisStorage` 即可支持多实例：

```go
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
storage := service.NewRedisStorage(rdb)
```

### 关闭 Demo 模式

生产环境应禁用 demo register：

```go
appCfg := gateway.AppConfig{
    EnableDemoRegister: false,
    // ...
}
```

### 添加自定义页面

Studio 使用 `html/template` 模板（位于 `examples/studio/templates/`）。新增页面：

1. 创建新的 `.html` 模板
2. 在 `main.go` 中注册路由 handler
3. 使用 HTMX 调用现有 `/api/v1/*` 端点

---

## 5. 与 Python Studio UI 互通

Go Gateway 完整实现了 AG-UI Protocol。Python 版 AgentScope 的 React Studio 前端可以通过以下方式连接 Go 后端：

```bash
# 启动 Go Gateway
cd examples/full_service
go run .

# 配置 Python Studio UI 指向 http://localhost:8080
```

在 SSE 连接时附加 `?protocol=agui` 或 header `X-Protocol: agui` 即可启用 AG-UI 事件转换。

---

## 6. 生产建议

- 使用 HTTPS 反向代理（Nginx / ALB）
- 禁用 demo register，启用 JWT 认证
- 使用 Redis Storage 替代 MemoryStorage
- 为 Workspace 设置持久化卷
- 配置 PermissionEngine 限制工具执行范围

---

## 7. 相关文件

- `examples/studio/main.go`
- `examples/studio/templates/`
- `gateway/agui.go`（AG-UI Protocol 转换）
- `gateway/app.go`（NewApp 自动装配）
