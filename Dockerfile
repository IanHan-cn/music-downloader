# syntax=docker/dockerfile:1

# ==================== 构建阶段 ====================
FROM golang:1.27-alpine AS builder

WORKDIR /src

# 先只复制依赖清单，让依赖下载这一层可以被 Docker 缓存复用
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 再复制源码并编译成静态二进制（项目无 CGO 依赖，关闭 CGO 便于使用精简基础镜像）
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/music-downloader .

# ==================== 运行阶段 ====================
FROM alpine:3.22

# 安装 HTTPS 所需证书与时区数据
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /out/music-downloader /app/music-downloader

ENV TZ=Asia/Shanghai

USER appuser
EXPOSE 8080

# 容器健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/music-downloader"]
