#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Go 基准测试脚本 — llm-gateway
#
# 运行 Go benchmark 并生成报告
#
# 用法：
#   chmod +x scripts/benchmark/go-bench.sh
#   scripts/benchmark/go-bench.sh
#   BENCH_TIME=5s BENCH_PKG=./internal/... scripts/benchmark/go-bench.sh
#   REPORT_DIR=./reports scripts/benchmark/go-bench.sh
# ──────────────────────────────────────────────────────────────

set -euo pipefail

# ── 配置 ────────────────────────────────────────────────────
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPORT_DIR:-${PROJECT_ROOT}/reports/benchmark}"
BENCH_PKG="${BENCH_PKG:-./...}"
BENCH_TIME="${BENCH_TIME:-3s}"
BENCH_COUNT="${BENCH_COUNT:-3}"
MEM_PROFILE="${MEM_PROFILE:-}"
CPU_PROFILE="${CPU_PROFILE:-}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── 依赖检查 ────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo -e "${RED}[错误] Go 未安装。请先安装 Go。${NC}"
  exit 1
fi

# ── 准备报告目录 ────────────────────────────────────────────
mkdir -p "$REPORT_DIR"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
REPORT_FILE="${REPORT_DIR}/bench-${TIMESTAMP}.txt"
JSON_FILE="${REPORT_DIR}/bench-${TIMESTAMP}.json"

echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Go 基准测试 — llm-gateway${NC}"
echo -e "${CYAN}  时间: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
echo -e "${CYAN}  包:   ${BENCH_PKG}${NC}"
echo -e "${CYAN}  时长: ${BENCH_TIME} × ${BENCH_COUNT} 轮${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo ""

# ── 构建 go test 命令 ───────────────────────────────────────
GO_TEST_CMD="go test -bench=. -benchmem -count=${BENCH_COUNT} -timeout=30m"

if [ -n "$BENCH_TIME" ]; then
  GO_TEST_CMD="${GO_TEST_CMD} -benchtime=${BENCH_TIME}"
fi

if [ -n "$MEM_PROFILE" ]; then
  GO_TEST_CMD="${GO_TEST_CMD} -memprofile=${REPORT_DIR}/mem-${TIMESTAMP}.prof"
fi

if [ -n "$CPU_PROFILE" ]; then
  GO_TEST_CMD="${GO_TEST_CMD} -cpuprofile=${REPORT_DIR}/cpu-${TIMESTAMP}.prof"
fi

GO_TEST_CMD="${GO_TEST_CMD} ${BENCH_PKG}"

echo -e "${YELLOW}[执行] ${GO_TEST_CMD}${NC}"
echo ""

# ── 运行基准测试 ────────────────────────────────────────────
cd "$PROJECT_ROOT"

# 运行并同时输出到终端和文件
eval "$GO_TEST_CMD" 2>&1 | tee "$REPORT_FILE"
BENCH_EXIT=${PIPESTATUS[0]}

echo ""

# ── 生成 JSON 格式（如果安装了 benchstat）────────────────────
if command -v benchstat &>/dev/null; then
  echo -e "${YELLOW}[分析] 使用 benchstat 生成统计...${NC}"
  # 将结果转为 benchstat 可分析的格式
  benchstat "$REPORT_FILE" | tee "$JSON_FILE" 2>/dev/null || true
else
  echo -e "${YELLOW}[提示] 安装 benchstat 可获得更好的统计报告：${NC}"
  echo "  go install golang.org/x/perf/cmd/benchstat@latest"
fi

# ── 输出摘要 ────────────────────────────────────────────────
echo ""
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
if [ $BENCH_EXIT -eq 0 ]; then
  echo -e "${GREEN}  基准测试完成${NC}"
else
  echo -e "${RED}  基准测试失败 (exit code: ${BENCH_EXIT})${NC}"
fi
echo -e "${CYAN}  报告: ${REPORT_FILE}${NC}"
if [ -f "$JSON_FILE" ]; then
  echo -e "${CYAN}  统计: ${JSON_FILE}${NC}"
fi
if [ -n "$MEM_PROFILE" ]; then
  echo -e "${CYAN}  内存: ${REPORT_DIR}/mem-${TIMESTAMP}.prof${NC}"
fi
if [ -n "$CPU_PROFILE" ]; then
  echo -e "${CYAN}  CPU:  ${REPORT_DIR}/cpu-${TIMESTAMP}.prof${NC}"
fi
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"

exit $BENCH_EXIT
