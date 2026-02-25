#!/bin/bash
# log-manager 启动脚本
# 用法:
#   ./start.sh           # 开发：前台启动后端+前端，Ctrl+C 停止
#   ./start.sh -d        # 开发：后台运行，输出写入 logs/
#   ./start.sh build     # 打包前端到 backend/web，用于生产部署
#   ./start.sh prod      # 生产：仅启动后端（托管前端），单端口 8080

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

MODE="dev"
if [ "${1:-}" = "-d" ]; then
    MODE="daemon"
elif [ "${1:-}" = "build" ]; then
    MODE="build"
elif [ "${1:-}" = "prod" ]; then
    MODE="prod"
fi

# ---------- build: 打包前端到 backend/web ----------
do_build() {
    command -v npm >/dev/null 2>&1 || { echo "错误: 需要 Node.js/npm"; exit 1; }
    echo "打包前端..."
    if [ ! -d "frontend/node_modules" ]; then
        (cd frontend && npm install)
    fi
    (cd frontend && CI=false npm run build)
    rm -rf backend/web
    cp -r frontend/build backend/web
    echo "已输出到 backend/web，可用于生产部署"
}

if [ "$MODE" = "build" ]; then
    do_build
    exit 0
fi

# ---------- prod: 仅启动后端，托管前端 ----------
do_prod() {
    command -v go >/dev/null 2>&1 || { echo "错误: 需要 Go"; exit 1; }
    if [ ! -d "backend/web" ]; then
        echo "请先执行 ./start.sh build 打包前端"
        exit 1
    fi
    check_port() {
        if command -v lsof >/dev/null 2>&1; then
            lsof -Pi :"$1" -sTCP:LISTEN -t >/dev/null 2>&1 && { echo "端口 $1 已被占用"; exit 1; }
        fi
    }
    check_port 8080

    DAEMON=false
    [ "${2:-}" = "-d" ] || [ "${1:-}" = "-d" ] && DAEMON=true

    if [ "$DAEMON" = true ]; then
        mkdir -p logs
        echo "生产模式（后台）: http://localhost:8080"
        (cd backend && CONFIG=config.prod.yaml go run main.go >> ../logs/backend.log 2>&1) &
        echo $! > .pids
        echo "使用 ./stop.sh 停止"
    else
        echo "生产模式: http://localhost:8080"
        (cd backend && CONFIG=config.prod.yaml go run main.go)
    fi
    exit 0
}

if [ "$MODE" = "prod" ]; then
    do_prod "$@"
    exit 0
fi

# ---------- dev: 后端 + 前端开发服务器 ----------
DAEMON=false
[ "$MODE" = "daemon" ] && DAEMON=true

command -v go >/dev/null 2>&1 || { echo "错误: 需要 Go"; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "错误: 需要 Node.js/npm"; exit 1; }

check_port() {
    local port=$1 name=$2
    if command -v lsof >/dev/null 2>&1; then
        if lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo "错误: 端口 $port 已被占用，$name 无法启动"
            exit 1
        fi
    fi
}
check_port 8080 "后端"
check_port 3000 "前端"

if [ ! -d "frontend/node_modules" ]; then
    echo "安装前端依赖..."
    (cd frontend && npm install)
fi

cleanup() {
    [ -n "${BACKEND_PID:-}" ] && kill "$BACKEND_PID" 2>/dev/null || true
    [ -n "${FRONTEND_PID:-}" ] && kill "$FRONTEND_PID" 2>/dev/null || true
    rm -f .pids
    echo "log-manager 已停止"
}
trap cleanup EXIT INT TERM

if [ "$DAEMON" = true ]; then
    mkdir -p logs
    REDIRECT_BACKEND=">> logs/backend.log 2>&1"
    REDIRECT_FRONTEND=">> logs/frontend.log 2>&1"
    echo "后台模式: 输出写入 logs/"
else
    REDIRECT_BACKEND=""
    REDIRECT_FRONTEND=""
fi

echo "启动后端..."
eval "(cd backend && go run main.go) $REDIRECT_BACKEND" &
BACKEND_PID=$!
[ "$DAEMON" = true ] && echo "$BACKEND_PID" > .pids

sleep 2
if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "错误: 后端启动失败"
    exit 1
fi

echo "启动前端..."
eval "(cd frontend && npm start) $REDIRECT_FRONTEND" &
FRONTEND_PID=$!
[ "$DAEMON" = true ] && echo "$FRONTEND_PID" >> .pids

echo ""
echo "=== log-manager 已启动 ==="
echo "后端: http://localhost:8080"
echo "前端: http://localhost:3000"
echo ""
if [ "$DAEMON" = true ]; then
    echo "使用 ./stop.sh 停止"
    trap - EXIT INT TERM
    exit 0
fi
echo "按 Ctrl+C 停止"
wait
