# ============================================
# LLM Gateway — 多阶段构建 Dockerfile
# 目标：最小镜像体积 + 非root运行 + 健康检查
# ============================================

# ---- Stage 1: Build admin SPA ----
FROM node:20-alpine AS admin-builder

WORKDIR /app/web/admin

# 先复制依赖文件，利用 Docker 缓存层
COPY web/admin/package*.json ./
RUN npm ci --omit=dev --ignore-scripts && \
    npm cache clean --force

# 复制源码并构建
COPY web/admin/ .
RUN npm run build && \
    rm -rf node_modules

# ---- Stage 2: Build Go binary ----
FROM golang:1.22-alpine AS go-builder

# 安装 git（某些 go module 可能需要）
RUN apk --no-cache add git ca-certificates

WORKDIR /app

# 先下载依赖，利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 复制源码
COPY . .

# 从 Stage 1 复制前端构建产物
COPY --from=admin-builder /app/web/admin/dist /app/internal/httpserver/adminui

# 编译：静态链接、去除符号表和调试信息、减小体积
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags='-w -s -extldflags "-static"' \
    -installsuffix cgo \
    -o /app/llm-gateway \
    ./cmd/server && \
    # 验证二进制可执行
    chmod +x /app/llm-gateway && \
    /app/llm-gateway --help 2>/dev/null || true

# ---- Stage 3: Minimal runtime ----
FROM alpine:3.19

# 安全：创建非 root 用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 安装运行时依赖
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=go-builder --chown=appuser:appgroup /app/llm-gateway .

# 切换到非 root 用户
USER appuser:appgroup

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz > /dev/null || exit 1

# 启动
ENTRYPOINT ["./llm-gateway"]
