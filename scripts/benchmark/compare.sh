#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Go Benchmark 对比脚本
#
# 对比两次 benchmark 结果，展示性能变化。
# 依赖 benchstat（go install golang.org/x/perf/cmd/benchstat@latest）
#
# 用法：
#   chmod +x scripts/benchmark/compare.sh
#   scripts/benchmark/compare.sh before.txt after.txt
#   scripts/benchmark/compare.sh --latest  # 自动对比最近两份报告
# ──────────────────────────────────────────────────────────────

set -euo pipefail

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── 帮助信息 ────────────────────────────────────────────────
usage() {
  cat <<EOF
用法:
  $(basename "$0") <before.txt> <after.txt>   对比两份 benchmark 报告
  $(basename "$0") --latest                    自动对比最近两份报告
  $(basename "$0") --dir <path>                指定报告目录

参数:
  --dir <path>     报告目录 (默认: ./reports/benchmark)
  --latest         自动选择最近两份 bench-*.txt 报告
  --help           显示帮助

示例:
  $(basename "$0") reports/benchmark/bench-20250101_120000.txt reports/benchmark/bench-20250101_130000.txt
  $(basename "$0") --latest
  $(basename "$0") --dir /path/to/reports --latest
EOF
  exit 0
}

# ── 参数解析 ────────────────────────────────────────────────
REPORT_DIR="$(cd "$(dirname "$0")/../.." && pwd)/reports/benchmark"
FILE1=""
FILE2=""
LATEST=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      ;;
    --dir)
      REPORT_DIR="$2"
      shift 2
      ;;
    --latest)
      LATEST=true
      shift
      ;;
    -*)
      echo -e "${RED}[错误] 未知参数: $1${NC}"
      usage
      ;;
    *)
      if [ -z "$FILE1" ]; then
        FILE1="$1"
      elif [ -z "$FILE2" ]; then
        FILE2="$1"
      fi
      shift
      ;;
  esac
done

# ── 依赖检查 ────────────────────────────────────────────────
if ! command -v benchstat &>/dev/null; then
  echo -e "${RED}[错误] benchstat 未安装。${NC}"
  echo "  安装命令: go install golang.org/x/perf/cmd/benchstat@latest"
  exit 1
fi

# ── 自动查找最近两份报告 ────────────────────────────────────
if $LATEST; then
  if [ ! -d "$REPORT_DIR" ]; then
    echo -e "${RED}[错误] 报告目录不存在: ${REPORT_DIR}${NC}"
    exit 1
  fi

  # 按修改时间排序，取最近两份
  mapfile -t FILES < <(ls -t "${REPORT_DIR}"/bench-*.txt 2>/dev/null || true)

  if [ ${#FILES[@]} -lt 2 ]; then
    echo -e "${RED}[错误] 报告目录中不足两份 benchmark 报告。${NC}"
    echo "  目录: ${REPORT_DIR}"
    echo "  找到: ${#FILES[@]} 份"
    exit 1
  fi

  FILE1="${FILES[1]}"  # 倒数第二
  FILE2="${FILES[0]}"
  echo -e "${YELLOW}[自动] 选择最近两份报告进行对比${NC}"
fi

# ── 验证文件 ────────────────────────────────────────────────
if [ -z "$FILE1" ] || [ -z "$FILE2" ]; then
  echo -e "${RED}[错误] 需要提供两份 benchmark 报告文件。${NC}"
  usage
fi

if [ ! -f "$FILE1" ]; then
  echo -e "${RED}[错误] 文件不存在: ${FILE1}${NC}"
  exit 1
fi

if [ ! -f "$FILE2" ]; then
  echo -e "${RED}[错误] 文件不存在: ${FILE2}${NC}"
  exit 1
fi

# ── 输出对比 ────────────────────────────────────────────────
echo ""
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  Benchmark 对比报告${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${YELLOW}基准 (before):${NC} ${FILE1}"
echo -e "  ${YELLOW}对比 (after):${NC}  ${FILE2}"
echo ""

# benchstat 输出表格
benchstat "$FILE1" "$FILE2"

echo ""
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  对比完成${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BOLD}解读说明:${NC}"
echo -e "  • ${GREEN}负值${NC} (如 -10%) = 性能提升（耗时减少）"
echo -e "  • ${RED}正值${NC} (如 +15%) = 性能下降（耗时增加）"
echo -e "  • ${YELLOW}p-value < 0.05${NC} = 差异具有统计显著性"
echo -e "  • ${YELLOW}p-value ≥ 0.05${NC} = 差异可能是随机波动"
echo ""
