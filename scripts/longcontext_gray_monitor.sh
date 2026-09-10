#!/usr/bin/env bash
# Long-context production gray-scale monitor.
# Usage: longcontext_gray_monitor.sh [admin_key]
set -euo pipefail

ADMIN_KEY="${1:-ok0115ok}"
BASE_URL="http://127.0.0.1:8080"
LOG_FILE="/var/log/llm-gateway/longcontext-gray.log"
ALERT_LOG="/var/log/llm-gateway/longcontext-alerts.log"

mkdir -p "$(dirname "$LOG_FILE")"
mkdir -p "$(dirname "$ALERT_LOG")"

health() {
  curl -sS -H "Authorization: Bearer $ADMIN_KEY" "$BASE_URL/admin/long-context/health" 2>/dev/null || echo '{}'
}

metrics() {
  curl -sS "$BASE_URL/debug/vars" 2>/dev/null || echo '{}'
}

timestamp=$(date -Iseconds)
h=$(health)
m=$(metrics)

status=$(echo "$h" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status","unknown"))' 2>/dev/null || echo unknown)

tasks_created=$(echo "$m" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("longcontext.tasks.created",0))' 2>/dev/null || echo 0)
worker_claims=$(echo "$m" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("longcontext.worker.claims",0))' 2>/dev/null || echo 0)
worker_errors=$(echo "$m" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("longcontext.worker.errors",0))' 2>/dev/null || echo 0)

# Count failed/mapping tasks via database
failed_sql="SELECT count(*) FROM long_context_tasks WHERE status='failed' AND updated_at > NOW() - INTERVAL '10 minutes';"
mapping_sql="SELECT count(*) FROM long_context_tasks WHERE status='mapping' AND (map_lease_until IS NULL OR map_lease_until < NOW() - INTERVAL '5 minutes');"

failed_count=$(docker exec swarm-platform-postgres psql -U llmadmin -d llmgateway -Atc "$failed_sql" 2>/dev/null || echo 0)
stuck_mapping=$(docker exec swarm-platform-postgres psql -U llmadmin -d llmgateway -Atc "$mapping_sql" 2>/dev/null || echo 0)

echo "$timestamp health=$status tasks_created=$tasks_created worker_claims=$worker_claims worker_errors=$worker_errors failed_10m=$failed_count stuck_mapping=$stuck_mapping" >> "$LOG_FILE"

if [ "$status" != "healthy" ]; then
  echo "$timestamp ALERT long-context health=$status" >> "$ALERT_LOG"
fi
if [ "$worker_errors" -gt 0 ] && [ "$worker_claims" -gt 0 ]; then
  err_rate=$(awk "BEGIN {printf \"%.2f\", $worker_errors/$worker_claims*100}")
  if awk "BEGIN {exit !($err_rate > 20)}"; then
    echo "$timestamp ALERT worker error rate ${err_rate}%" >> "$ALERT_LOG"
  fi
fi
if [ "$failed_count" -gt 5 ]; then
  echo "$timestamp ALERT $failed_count failed tasks in last 10m" >> "$ALERT_LOG"
fi
if [ "$stuck_mapping" -gt 0 ]; then
  echo "$timestamp ALERT $stuck_mapping chunks stuck in mapping" >> "$ALERT_LOG"
fi
