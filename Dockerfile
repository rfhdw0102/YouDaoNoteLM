# ========== 阶段1: 构建前端 ==========
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# 复制前端依赖文件
COPY frontend/package.json frontend/package-lock.json* ./

# 安装前端依赖
RUN npm ci

# 复制前端源码
COPY frontend/ .

# 构建前端
RUN npm run build

# ========== 阶段2: 构建后端 ==========
FROM golang:1.25-alpine AS backend-builder

# 安装构建依赖
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# 复制依赖文件（vendor 不提交到仓库，构建时从模块代理下载）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server ./cmd/server

# ========== 阶段3: 最终镜像 ==========
FROM nginx:1.27-bookworm

# 安装运行时依赖（Debian-based，提供 glibc 兼容性，youadonote CLI 基于 Bun 需要 glibc）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata curl python3 python3-pip bash ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# 安装 youdaonote CLI（使用官方一键脚本，自动处理 AVX2 兼容性）
RUN curl -fsSL https://artifact.lx.netease.com/download/youdaonote-cli/install.sh | bash -s -- -f -b /usr/local/bin

# 设置时区
ENV TZ=Asia/Shanghai

# 复制 Nginx 配置
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/dist /usr/share/nginx/html

# 复制后端二进制文件
COPY --from=backend-builder /app/server /app/server

# 复制配置文件目录
COPY configs/ /app/configs/

# 复制有道转换脚本
COPY scripts/ /app/scripts/
RUN pip3 install --no-cache-dir -r /app/scripts/youdao/requirements.txt 2>/dev/null || \
    pip3 install --break-system-packages --no-cache-dir -r /app/scripts/youdao/requirements.txt

# 创建必要目录
RUN mkdir -p /app/logs

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

# 启动脚本
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

CMD ["/docker-entrypoint.sh"]
