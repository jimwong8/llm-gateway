#!/usr/bin/env bash
set -euo pipefail
cd /home/jimwong/projects/llm-gateway

INSTANCES="${1:-2}"
BASE_PORT=8090
echo "启动 ${INSTANCES} 个网关实例..."

for i in $(seq 1 "$INSTANCES"); do
    PORT=$((BASE_PORT + i - 1))
    LOG="/tmp/llm-gateway-instance-${i}.log"
    echo "启动实例 ${i} 在端口 ${PORT} ..."
    
    # 每个实例使用独立的环境变量前缀
    APP_PORT="${PORT}" \
    LONG_CONTEXT_WORKERS=0 \
    LONG_CONTEXT_ENABLED=false \
    nohup ./llm-gateway > "$LOG" 2>&1 &
    echo $! > "/tmp/llm-gateway-instance-${i}.pid"
    echo "实例 ${i} PID=$(cat /tmp/llm-gateway-instance-${i}.pid) 端口=${PORT}"
done

echo "等待实例就绪..."
sleep 5
for i in $(seq 1 "$INSTANCES"); do
    PORT=$((BASE_PORT + i - 1))
    for attempt in $(seq 1 30); do
        if curl -sS -o /dev/null -w "%{http_code}" -H "X-Admin-Key: ok0115ok" "http://127.0.0.1:${PORT}/admin/long-context/health" 2>/dev/null | grep -q "200"; then
            echo "实例 ${i} (端口 ${PORT}) 就绪"
            break
        fi
        sleep 1
    done
done
echo "所有实例启动完成"
