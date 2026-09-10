#!/bin/bash
# LLM Gateway 启动脚本 - 使用 swarm-platform postgres/redis (内部端口 5432/6379)
# 位置: /home/jimwong/projects/llm-gateway/start.sh

export APP_ENV=production
export APP_PORT=8080
export ADMIN_API_KEY=ok0115ok
export POSTGRES_PASSWORD=swarm
export POSTGRES_DSN="postgres://swarm:swarm@localhost:5433/llmgateway?sslmode=disable"
export REDIS_ADDR=localhost:6380
export REDIS_PASSWORD=
export MOCK_MODE=false
export L1_CACHE_TTL_SECONDS=600
export SEMANTIC_CACHE_ENABLED=true
export SEMANTIC_CACHE_THRESHOLD=0.80
export SEMANTIC_VECTOR_SIZE=64
export CACHE_POSTGRES_DSN="postgres://swarm:swarm@localhost:5433/llmgateway?sslmode=disable"
export SEMANTIC_POSTGRES_DSN="postgres://swarm:swarm@localhost:5433/llmgateway?sslmode=disable"

cd /home/jimwong/projects/llm-gateway
./llm-gateway &>/tmp/llm-gateway.log &
GATEWAY_PID=$!
sleep 3
echo "Gateway PID: $GATEWAY_PID"
echo ""
ss -tlnp | grep 8080 && echo "✅ Port 8080 listening" || echo "❌ Port 8080 NOT listening"
curl -s http://localhost:8080/health 2>&1 | head -2
echo ""
echo "Logs: tail -f /tmp/llm-gateway.log"
