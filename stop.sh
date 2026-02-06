#!/bin/bash
# 停止 log-manager 后台服务（仅对 start.sh -d 启动的进程有效）

ROOT="$(cd "$(dirname "$0")" && pwd)"
PIDFILE="$ROOT/.pids"

if [ ! -f "$PIDFILE" ]; then
    echo "未找到 .pids 文件，可能未使用 start.sh -d 启动"
    exit 0
fi

while read -r pid; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        echo "停止进程 $pid"
        kill "$pid" 2>/dev/null || true
    fi
done < "$PIDFILE"
rm -f "$PIDFILE"
echo "log-manager 已停止"
