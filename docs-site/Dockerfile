FROM golang:1.25-alpine AS builder

WORKDIR /app

# 安装 git（部分依赖可能需要）
RUN apk add --no-cache git

# 下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o agent-service ./examples/full_service

# 生产镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# 创建非 root 用户
RUN adduser -D -s /bin/sh agentscope

COPY --from=builder /app/agent-service .

# 创建必要目录
RUN mkdir -p /var/lib/agentscope-go/workspaces /var/lib/agentscope-go/.embed_cache && \
    chown -R agentscope:agentscope /var/lib/agentscope-go /app

USER agentscope

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./agent-service"]
