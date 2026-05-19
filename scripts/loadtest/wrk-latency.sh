#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# wrk 延迟测试脚本 — llm-gateway
#
# 测试端点：
#   /healthz
#   /api/memory/presets
#   /admin/observability/summary
#
# 输出：QPS、P50/P95/P99 延迟
#
# 用法：
#   chmod +x scripts/loadtest/wrk-latency.sh
#   scripts/loadtest/wrk-latency.sh
#   BASE_URL=http://staging:8080 scripts/loadtest/wrk-latency.sh
#   TOKEN=xxx scripts/loadtest/wrk-latency.sh
# ──────────────────────────────────────────────────────────────

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
TOKEN="${TOKEN:-}"
DURATION="${DURATION:-30s}"
CONNECTIONS="${CONNECTIONS:-100}"
THREADS="${THREADS:-4}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── 依赖检查 ────────────────────────────────────────────────
if ! command -v wrk &>/dev/null; then
  echo -e "${RED}[错误] wrk 未安装。请先安装 wrk：${NC}"
  echo "  Ubuntu/Debian: sudo apt install wrk"
  echo "  macOS:         brew install wrk"
  echo "  源码编译:       https://github.com/wg/wrk"
  exit 1
fi

# ── 构建认证头 ──────────────────────────────────────────────
AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="-H \"Authorization: Bearer ${TOKEN}\""
fi

# ── 输出格式化的 Lua 脚本 ──────────────────────────────────
LUA_SCRIPT=$(mktemp /tmp/wrk-latency-XXXXXX.lua)
cat > "$LUA_SCRIPT" <<'LUA'
-- 收集所有请求的延迟数据
latencies = {}
request = function()
  return wrk.format(nil, nil, nil, nil)
end

response = function(status, headers, body)
  -- wrk 不直接暴露单次请求延迟，我们通过 done() 汇总
end

done = function(summary, latency, requests)
  -- 输出结构化结果供 shell 解析
  print(string.format("WRK_RESULTS:%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f",
    summary.requests,
    summary.duration,
    latency:percentile(50) / 1000,   -- 转 ms
    latency:percentile(75) / 1000,
    latency:percentile(90) / 1000,
    latency:percentile(95) / 1000,
    latency:percentile(99) / 1000
  ))
end
LUA

# ── 测试函数 ────────────────────────────────────────────────
run_benchmark() {
  local label="$1"
  local path="$2"
  local url="${BASE_URL}${path}"

  echo ""
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${GREEN}▶ 测试: ${label}${NC}"
  echo -e "${YELLOW}  URL: ${url}${NC}"
  echo -e "${YELLOW}  参数: ${THREADS} 线程, ${CONNECTIONS} 连接, ${DURATION}${NC}"
  echo ""

  # 执行 wrk，捕获输出
  local output
  if [ -n "$TOKEN" ]; then
    output=$(wrk -t"$THREADS" -c"$CONNECTIONS" -d"$DURATION" \
      -s "$LUA_SCRIPT" \
      -H "Authorization: Bearer ${TOKEN}" \
      "$url" 2>&1) || true
  else
    output=$(wrk -t"$THREADS" -c"$CONNECTIONS" -d"$DURATION" \
      -s "$LUA_SCRIPT" \
      "$url" 2>&1) || true
  fi

  # 提取结构化结果
  local result_line
  result_line=$(echo "$output" | grep "^WRK_RESULTS:" | head -1)

  if [ -n "$result_line" ]; then
    # 解析: WRK_RESULTS:requests,duration_ms,p50,p75,p90,p95,p99
    IFS=',' read -r _ requests duration p50 p77 p90 p95 p99 <<< "$result_line"

    local qps
    qps=$(echo "scale=1; $requests * 1000 / $duration" | bc 2>/dev/null || echo "N/A")

    echo -e "  ${GREEN}┌─────────────── 结果 ───────────────┐${NC}"
    echo -e "  ${GREEN}│ 总请求数:    $(printf '%-20s' "$requests")${NC}│"
    echo -e "  ${GREEN}│ QPS:         $(printf '%-20s' "$qps")${NC}│"
    echo -e "  ${GREEN}│ P50 延迟:    $(printf '%-18s' "${p50}ms")${NC}│"
    echo -e "  ${GREEN}│ P75 延迟:    $(printf '%-18s' "${p77}ms")${NC}│"
    echo -e "  ${GREEN}│ P90 延迟:    $(printf '%-18s' "${p90}ms")${NC}│"
    echo -e "  ${GREEN}│ P95 延迟:    $(printf '%-18s' "${p95}ms")${NC}│"
    echo -e "  ${GREEN}│ P99 延迟:    $(printf '%-18s' "${p99}ms")${NC}│"
    echo -e "  ${GREEN}└────────────────────────────────────┘${NC}"
  else
    # 回退：直接输出 wrk 原始结果
    echo -e "  ${YELLOW}[提示] 使用标准 wrk 输出格式：${NC}"
    echo "$output" | sed 's/^/    /'
  fi
}

# ── 预热 ────────────────────────────────────────────────────
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  wrk 延迟测试 — llm-gateway${NC}"
echo -e "${CYAN}  目标: ${BASE_URL}${NC}"
echo -e "${CYAN}  时间: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"

echo ""
echo -e "${YELLOW}[预热] 发送 10 秒预热请求...${NC}"
wrk -t2 -c10 -10s "$BASE_URL/healthz" >/dev/null 2>&1 || true
echo -e "${GREEN}[预热] 完成${NC}"

# ── 执行测试 ────────────────────────────────────────────────
run_benchmark "Health Check" "/healthz"
run_benchmark "List Presets" "/api/memory/presets"
run_benchmark "Observability Summary" "/admin/observability/summary"

# ── 清理 ────────────────────────────────────────────────────
rm -f "$LUA_SCRIPT"

echo ""
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  测试完成${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
