#!/bin/bash
# 停止 log-manager 后台服务（仅对 start.sh -d 启动的进程有效）
# 递归终止进程树，避免 go run / npm start 的子进程残留占用端口

ROOT="$(cd "$(dirname "$0")" && pwd)"
PIDFILE="$ROOT/.pids"

# 递归终止进程及其所有子进程（go run、npm 等会起子进程）
kill_tree() {
    local pid=$1
    [ -z "$pid" ] || ! kill -0 "$pid" 2>/dev/null && return
    # 先终止子进程
    for child in $(ps -eo pid=,ppid= 2>/dev/null | awk -v p="$pid" '$2==p {print $1}'); do
        kill_tree "$child"
    done
    kill -TERM "$pid" 2>/dev/null || kill -9 "$pid" 2>/dev/null || true
}

# 若端口仍被占用，按端口清理（兜底）
kill_by_port() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        local pids
        pids=$(lsof -ti :"$port" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            echo "端口 $port 仍被占用，强制结束: $pids"
            echo "$pids" | xargs kill -9 2>/dev/null || true
        fi
    fi
}

if [ -f "$PIDFILE" ]; then
    while read -r pid; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo "停止进程 $pid 及其子进程"
            kill_tree "$pid"
        fi
    done < "$PIDFILE"
    rm -f "$PIDFILE"
    sleep 1
fi

# 兜底：若端口仍被占用则强制结束（log-manager 使用的端口：8888 后端、3000 前端、8890 UDP）
for port in 8888 3000 8890; do
    kill_by_port "$port"
done

echo "log-manager 已停止"
