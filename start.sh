#!/bin/bash
# log-manager 一键启动脚本
# 启动后端（8080）和前端（3000）
# 用法: ./start.sh         # 前台运行，Ctrl+C 停止
#       ./start.sh -d      # 后台运行，输出写入 logs/

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

DAEMON=false
if [ "${1:-}" = "-d" ]; then
    DAEMON=true
fi

# 检查环境
command -v go >/dev/null 2>&1 || { echo "错误: 需要安装 Go (https://golang.org/)"; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "错误: 需要安装 Node.js/npm (https://nodejs.org/)"; exit 1; }

# 检查端口占用
check_port() {
    local port=$1
    local name=$2
    if command -v lsof >/dev/null 2>&1; then
        if lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo "错误: 端口 $port 已被占用，$name 无法启动"
            exit 1
        fi
    fi
}
check_port 8080 "后端"
check_port 3000 "前端"

# 安装前端依赖（若不存在）
if [ ! -d "frontend/node_modules" ]; then
    echo "安装前端依赖..."
    (cd frontend && npm install)
fi

# 清理函数
cleanup() {
    if [ -n "${BACKEND_PID:-}" ]; then
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
    fi
    if [ -n "${FRONTEND_PID:-}" ]; then
        kill "$FRONTEND_PID" 2>/dev/null || true
        wait "$FRONTEND_PID" 2>/dev/null || true
    fi
    if [ -f .pids ]; then
        rm -f .pids
    fi
    echo ""
    echo "log-manager 已停止"
}
trap cleanup EXIT INT TERM

if [ "$DAEMON" = true ]; then
    mkdir -p logs
    BACKEND_LOG="logs/backend.log"
    FRONTEND_LOG="logs/frontend.log"
    echo "后台模式: 输出写入 $BACKEND_LOG 和 $FRONTEND_LOG"
    REDIRECT_BACKEND=">> $BACKEND_LOG 2>&1"
    REDIRECT_FRONTEND=">> $FRONTEND_LOG 2>&1"
else
    REDIRECT_BACKEND=""
    REDIRECT_FRONTEND=""
fi

# 启动后端
echo "启动后端服务..."
eval "(cd backend && go run main.go) $REDIRECT_BACKEND" &
BACKEND_PID=$!
if [ "$DAEMON" = true ]; then
    echo "$BACKEND_PID" > .pids
fi

# 等待后端就绪
sleep 2
if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "错误: 后端启动失败，请查看上方输出或 logs/backend.log"
    exit 1
fi

# 启动前端
echo "启动前端服务..."
eval "(cd frontend && npm start) $REDIRECT_FRONTEND" &
FRONTEND_PID=$!
if [ "$DAEMON" = true ]; then
    echo "$FRONTEND_PID" >> .pids
fi

echo ""
echo "=== log-manager 已启动 ==="
echo "后端: http://localhost:8080"
echo "前端: http://localhost:3000"
echo ""
if [ "$DAEMON" = true ]; then
    echo "后台运行中，使用 ./stop.sh 停止"
    trap - EXIT INT TERM  # 移除 trap，不终止后台进程
    exit 0
fi

echo "按 Ctrl+C 停止"
wait
