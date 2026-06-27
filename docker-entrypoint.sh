#!/bin/bash
set -e

# ========== 信号转发 ==========
# 确保 SIGTERM/SIGINT 能正确传递给子进程，实现优雅停机
cleanup() {
    echo "🛑 收到停止信号，正在关闭服务..."
    if [ -n "$APP_PID" ]; then
        kill -TERM "$APP_PID" 2>/dev/null || true
        wait "$APP_PID" 2>/dev/null || true
    fi
    nginx -s quit 2>/dev/null || true
    echo "✅ 服务已停止"
    exit 0
}
trap cleanup SIGTERM SIGINT

# ========== 修复端口冲突 ==========
# Nginx 监听 8080 并代理到 8081，Go 应用需要监听 8081
# Docker 环境中使用 docker_config.yaml 已配置端口为 8081，无需修改
echo "✅ Docker 环境：Go 应用端口已配置为 8081（Nginx 代理转发）"

# ========== 启动服务 ==========
cd /app

# 启动后端服务（后台运行）
/app/server &
APP_PID=$!
echo "✅ Go 后端服务已启动 (PID: $APP_PID)"

# 启动 Nginx（前台运行）
echo "✅ Nginx 已启动，监听 8080 端口"
nginx -g "daemon off;" &
NGINX_PID=$!

# ========== 进程监控 ==========
# 等待任一进程退出，确保容器在服务异常时能被 Docker 感知
while true; do
    # 检查 Go 进程是否存活
    if ! kill -0 "$APP_PID" 2>/dev/null; then
        echo "❌ Go 后端服务异常退出"
        kill -TERM "$NGINX_PID" 2>/dev/null || true
        exit 1
    fi
    # 检查 Nginx 进程是否存活
    if ! kill -0 "$NGINX_PID" 2>/dev/null; then
        echo "❌ Nginx 异常退出"
        kill -TERM "$APP_PID" 2>/dev/null || true
        exit 1
    fi
    sleep 5
done
