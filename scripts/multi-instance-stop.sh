#!/usr/bin/env bash
set -euo pipefail
echo "停止所有多实例..."
for pidfile in /tmp/llm-gateway-instance-*.pid; do
    [ -f "$pidfile" ] || continue
    pid=$(cat "$pidfile")
    instance=$(echo "$pidfile" | grep -oE "[0-9]+")
    if kill -0 "$pid" 2>/dev/null; then
        echo "停止实例 ${instance} (PID=${pid})"
        kill "$pid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
done
# 清理可能残留的进程
pkill -f "llm-gateway" 2>/dev/null || true
echo "所有实例已停止"
