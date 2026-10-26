# ai-service（tellurion-agent 仓）的镜像构建。
#
# 本仓在 QT-17 S3 已删除 `replace tellurion => ../tellurion`：镜像只依赖本仓
# 源码与版本化依赖（contracts / kit），不再需要 core 的命名构建上下文。
# config/ai-service.yaml 与 deploy/wait-for-deps.sh 都是本仓自有副本。

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build/agent

# 依赖清单先行（层缓存友好）
COPY go.mod go.sum* ./

# agent 源码
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ai-service ./cmd/ai

# Final stage
FROM alpine:3.19

WORKDIR /app

# nc 供 deploy/wait-for-deps.sh 探依赖端口；curl 供 healthcheck。
RUN apk add --no-cache ca-certificates curl netcat-openbsd

COPY --from=builder /app/ai-service /app/ai-service

# 本仓自持的依赖等待脚本与配置（内容取自 core，只读复制）。
COPY deploy/wait-for-deps.sh /app/deploy/wait-for-deps.sh
RUN chmod +x /app/deploy/wait-for-deps.sh

COPY config/ai-service.yaml /app/config/ai-service.yaml

EXPOSE 8086

CMD ["/app/ai-service"]
