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

    srv := gateway.NewServer(agent)
    srv.RegisterV2Routes()
    log.Println("Listening on :8080")
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
    "github.com/linkerlin/agentscope.go/service"
)

func main() {
    // 1. 创建 Redis 存储
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    storage := service.NewRedisStorage(rdb)

    // 2. 创建 Agent
    agent, _ := /* ... */

    // 3. 创建 Gateway，注入存储
    srv := gateway.NewServer(agent).
        WithStorage(storage).
        WithAuthenticator(gateway.NewJWTAuthenticator("your-secret"))
    srv.RegisterV2Routes()

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

Gateway 内置 OpenTelemetry 追踪。启用方式：

```go
import "github.com/linkerlin/agentscope.go/observability"

observability.InitTracerProvider("agent-service")
srv.WithOtelHandler(/* ... */)
```

事件流指标可通过 `event/metrics` 的 `MetricsHandler` HTTP 端点暴露：

```
GET /metrics/events → JSON 格式的事件统计
```
