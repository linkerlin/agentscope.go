# 部署指南

AgentScope.Go 支持从单机开发到生产集群的多种部署形态。

---

## 1. 单机开发（零依赖）

```go
package main

import (
    "log"
    "net/http"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/gateway"
    "github.com/linkerlin/agentscope.go/memory"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    model, _ := openai.Builder().APIKey("key").Build()
    agent, _ := react.Builder().Name("bot").Model(model).Memory(memory.NewInMemoryMemory()).Build()

    srv := gateway.NewApp(gateway.AppConfig{
        Agent: agent,
        // 推荐：Storage + WorkspaceBaseDir + AutoStandardTools + EmbeddingCacheDir 等
    })
    srv.RegisterAppRoutes(jwtAuth)
    srv.Start()
    log.Fatal(http.ListenAndServe(":8080", srv))
}
```

无需 Redis、无需数据库，所有状态保存在内存中。

---

## 2. 生产部署（Redis + Gateway）

### 依赖

- Go 1.25+
- Redis 7.0+（用于 Session 持久化和 AgentState 快照）

### 代码

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/linkerlin/agentscope.go/gateway"
    "github.com/linkerlin/agentscope.go/observability"
    "github.com/linkerlin/agentscope.go/service"
)

func main() {
    // 1. 创建 Redis 存储
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    storage := service.NewRedisStorage(rdb)

    // 2. 创建 Agent
    agent, _ := /* ... */

    // 3. 创建 Gateway，注入存储 + tracing (Phase 5)
    tracingMW := &observability.TracingMiddlewareAdapter{
        Tracer: observability.NewOTelTracer(...), // 或 LangSmith
        Name:   "prod",
    }
    srv := gateway.NewServer(agent).
        WithStorage(storage).
        WithAuthenticator(gateway.NewJWTAuthenticator("your-secret"))
    srv.RegisterV2Routes()

    // 可选：tracing middleware 可在 agent builder 中使用
    // agentBuilder.Middlewares(tracingMW)

    log.Fatal(http.ListenAndServe(":8080", srv))
}
```

### 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `JWT_SECRET` | JWT 签名密钥 | `change-me-in-production` |
| `OPENAI_API_KEY` | OpenAI API Key | `sk-...` |
| `DEEPSEEK_API_KEY` | DeepSeek API Key | `sk-...` |

---

## 3. Docker 部署

### Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o agent-service ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent-service .
EXPOSE 8080
CMD ["./agent-service"]
```

### docker-compose.yml

```yaml
version: "3.8"
services:
  agent:
    build: .
    ports:
      - "8080:8080"
    environment:
      - REDIS_ADDR=redis:6379
      - JWT_SECRET=${JWT_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

---

## 4. Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agent
  template:
    metadata:
      labels:
        app: agent
    spec:
      containers:
        - name: agent
          image: your-registry/agentscope-go:latest
          ports:
            - containerPort: 8080
          env:
            - name: REDIS_ADDR
              value: "redis-service:6379"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: agent-secrets
                  key: jwt-secret
---
apiVersion: v1
kind: Service
metadata:
  name: agent-service
spec:
  selector:
    app: agent
  ports:
    - port: 80
      targetPort: 8080
```

> **注意**：多副本部署时，必须使用 Redis 存储 AgentState 快照，否则跨副本的挂起-恢复无法工作。

---

## 5. 健康检查与监控

### 健康检查端点

Gateway 内置 `/health` 端点（无需认证）：

```bash
curl http://localhost:8080/health
# {"status":"healthy","version":"2.0.0","storage":"configured","auth":"enabled","active_sessions":3}
```

Kubernetes 探针配置：

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 3
  periodSeconds: 5
```

### OpenTelemetry 追踪

```go
import "github.com/linkerlin/agentscope.go/observability"

observability.InitTracerProvider("agent-service")
srv.WithOtelHandler(/* ... */)
```

事件流指标可通过 `event/metrics` 的 `MetricsHandler` HTTP 端点暴露：

```
GET /metrics/events → JSON 格式的事件统计
```

---

## 6. systemd 部署

创建 `/etc/systemd/system/agentscope-go.service`：

```ini
[Unit]
Description=AgentScope.Go Agent Service
After=network.target redis.service

[Service]
Type=simple
User=agentscope
Group=agentscope
WorkingDirectory=/opt/agentscope-go
ExecStart=/opt/agentscope-go/agent-service
Restart=always
RestartSec=5
Environment="REDIS_ADDR=localhost:6379"
Environment="JWT_SECRET=change-me-in-production"
Environment="OPENAI_API_KEY=sk-..."
Environment="WORKSPACE_BASE_DIR=/var/lib/agentscope-go/workspaces"

[Install]
WantedBy=multi-user.target
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now agentscope-go
sudo systemctl status agentscope-go
```

---

## 7. Nginx 反向代理与 HTTPS

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/ssl/certs/api.example.com.crt;
    ssl_certificate_key /etc/ssl/private/api.example.com.key;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }
}
```

> WebSocket `/chat/ws` 和 SSE `/chat/stream` 需要 `Upgrade` 与 `Connection` 头才能正常工作。

---

## 8. 环境变量完整参考

| 变量 | 说明 | 是否必填 |
|------|------|----------|
| `REDIS_ADDR` | Redis 地址 | 生产必填 |
| `REDIS_PASSWORD` | Redis 密码 | 可选 |
| `JWT_SECRET` | JWT 签名密钥 | 启用认证时必填 |
| `WORKSPACE_BASE_DIR` | 本地工作区根目录 | 使用 Workspace 时必填 |
| `EMBEDDING_CACHE_DIR` | Embedding 文件缓存目录 | 可选 |
| `OPENAI_API_KEY` | OpenAI API Key | 使用 OpenAI 模型时必填 |
| `ANTHROPIC_API_KEY` | Anthropic API Key | 使用 Claude 时必填 |
| `GEMINI_API_KEY` | Google Gemini API Key | 使用 Gemini 时必填 |
| `DASHSCOPE_API_KEY` | 阿里云 DashScope API Key | 使用通义千问时必填 |
| `DEEPSEEK_API_KEY` | DeepSeek API Key | 使用 DeepSeek 时必填 |
| `MOONSHOT_API_KEY` | Moonshot API Key | 使用 Kimi 时必填 |
| `XAI_API_KEY` | xAI API Key | 使用 Grok 时必填 |
| `LANGSMITH_API_KEY` | LangSmith API Key | 可选 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry Collector 端点 | 可选 |

---

## 9. Redis 数据备份与恢复

### 备份

```bash
redis-cli --rdb /backup/agentscope-go.rdb
```

或使用 AOF：

```bash
cp /var/lib/redis/appendonly.aof /backup/agentscope-go.aof
```

### 恢复

```bash
# 停止服务
sudo systemctl stop agentscope-go

# 清空当前数据（谨慎操作）
redis-cli FLUSHDB

# 恢复 RDB
cp /backup/agentscope-go.rdb /var/lib/redis/dump.rdb
sudo systemctl restart redis

# 启动服务
sudo systemctl start agentscope-go
```

---

## 10. 水平扩展与负载均衡

多副本部署要点：

1. **必须使用共享存储**：RedisStorage 用于 Session、AgentState、Schedule 持久化
2. **无状态设计**：每个 Gateway 实例不保存本地状态
3. **Sticky Session**：SSE/WebSocket 客户端需固定到单一实例，或使用共享 pub/sub 事件总线
4. **推荐架构**：

```
                    ┌─────────────┐
     Clients ──────▶│  Nginx/ALB  │
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │  Gateway 1  │ │  Gateway 2  │ │  Gateway 3  │
    └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
           │               │               │
           └───────────────┼───────────────┘
                           ▼
                    ┌─────────────┐
                    │    Redis    │
                    └─────────────┘
```

---

## 11. 生产安全 checklist

- [ ] 修改默认 JWT secret，使用至少 32 字节随机字符串
- [ ] Redis 启用密码和 TLS
- [ ] Gateway 通过 Nginx/ALB 暴露 HTTPS
- [ ] Workspace 使用 Docker/E2B 隔离不可信代码
- [ ] 启用 PermissionEngine 并配置最小权限规则
- [ ] API Key 不提交到代码仓库，使用环境变量或密钥管理器
- [ ] 启用 `service.Cipher` 加密持久化凭证
- [ ] 定期运行 `govulncheck ./...` 检查依赖漏洞
- [ ] 配置日志轮转和监控告警
- [ ] 限制文件工具可访问的路径（权限规则）
- [ ] 生产环境禁用 demo register（`EnableDemoRegister: false`）

---

## 12. 故障排查

### Gateway 无法连接 Redis

```bash
redis-cli -h <addr> PING
# 应返回 PONG
```

### Schedule 重启后未恢复

确认使用 `gateway.NewApp` 并在启动后调用 `srv.Start()`，且 `Storage` 已配置。

### SSE 连接断开

检查 Nginx `proxy_read_timeout` 是否足够长；SSE 连接应保持开启。

### 内存占用过高

- 检查 `tool/file/cache.go` 缓存大小
- 检查 ReMe 向量记忆是否未配置窗口限制
- 检查是否有 goroutine 泄漏（使用 pprof）

---

## 13. 更多资源

- [examples/production](../examples/production/main.go) — 生产级 Gateway 示例
- [examples/full_service](../examples/full_service/main.go) — 一键自动装配示例
- [examples/multi_tenant_workspace](../examples/multi_tenant_workspace/main.go) — 多租户 + Workspace
- [SECURITY.md](../SECURITY.md) — 安全策略
