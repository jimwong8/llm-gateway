# 性能压测脚本

本目录包含 llm-gateway 的性能压测工具，使用 k6 和 wrk 进行 HTTP 层面的压力测试。

## 前置依赖

| 工具 | 安装方式 | 用途 |
|------|---------|------|
| [k6](https://k6.io) | `brew install k6` / `apt install k6` | 场景化压测 |
| [wrk](https://github.com/wg/wrk) | `brew install wrk` / `apt install wrk` | 延迟基准测试 |

> 脚本本身不引入外部依赖，k6/wrk 需单独安装。

---

## k6 场景压测 — `k6-http.js`

### 测试场景

模拟完整用户流程：

```
登录 → 查询 preset 列表 → 创建 preset → 查询 preset 详情
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `BASE_URL` | `http://localhost:8080` | 目标服务地址 |
| `VUS` | `10` | 虚拟用户数 |
| `DURATION` | `30s` | 测试持续时间 |
| `ADMIN_TOKEN` | *(空)* | 预置 token，跳过登录步骤 |

### 阈值

- **P95 < 500ms**（所有 HTTP 请求及各阶段单独统计）
- **错误率 < 1%**

### 用法

```bash
# 默认参数运行
k6 run scripts/loadtest/k6-http.js

# 50 虚拟用户，持续 60 秒
k6 run -e VUS=50 -e DURATION=60s scripts/loadtest/k6-http.js

# 指定目标环境
k6 run -e BASE_URL=https://staging.example.com scripts/loadtest/k6-http.js

# 使用预置 token（跳过登录）
k6 run -e ADMIN_TOKEN=eyJhbGciOi... scripts/loadtest/k6-http.js

# 输出 JSON 报告
k6 run --out json=results.json scripts/loadtest/k6-http.js

# 输出 InfluxDB / Prometheus
k6 run --out influxdb=http://localhost:8086/k6 scripts/loadtest/k6-http.js
```

### 自定义指标

| 指标名 | 说明 |
|--------|------|
| `errors` | 错误率（自定义 check 失败率） |
| `duration_login` | 登录接口延迟趋势 |
| `duration_list_presets` | 列表查询延迟趋势 |
| `duration_create_preset` | 创建 preset 延迟趋势 |
| `duration_preset_detail` | 详情查询延迟趋势 |

---

## wrk 延迟测试 — `wrk-latency.sh`

### 测试端点

| 端点 | 说明 |
|------|------|
| `/healthz` | 健康检查 |
| `/api/memory/presets` | Preset 列表 |
| `/admin/observability/summary` | 可观测性摘要 |

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `BASE_URL` | `http://localhost:8080` | 目标服务地址 |
| `TOKEN` | *(空)* | Bearer Token（可选） |
| `DURATION` | `30s` | 每个端点测试时长 |
| `CONNECTIONS` | `100` | 并发连接数 |
| `THREADS` | `4` | 线程数 |

### 输出

```
┌─────────────── 结果 ───────────────┐
│ 总请求数:    15234                 │
│ QPS:         507.8                 │
│ P50 延迟:    1.23ms                │
│ P75 延迟:    2.45ms                │
│ P90 延迟:    4.12ms                │
│ P95 延迟:    6.78ms                │
│ P99 延迟:   12.34ms                │
└────────────────────────────────────┘
```

### 用法

```bash
# 默认参数
chmod +x scripts/loadtest/wrk-latency.sh
./scripts/loadtest/wrk-latency.sh

# 指定目标和认证
TOKEN=xxx BASE_URL=http://staging:8080 ./scripts/loadtest/wrk-latency.sh

# 调整并发和时长
CONNECTIONS=200 DURATION=60s ./scripts/loadtest/wrk-latency.sh
```

---

## 快速开始

```bash
# 1. 确保服务已启动
#    (cd /path/to/llm-gateway && go run .)

# 2. 运行 k6 场景压测
k6 run scripts/loadtest/k6-http.js

# 3. 运行 wrk 延迟测试
./scripts/loadtest/wrk-latency.sh
```

## 注意事项

- 压测前确保服务已完成预热（wrk 脚本自带 10s 预热）
- 生产环境压测请先用小流量测试，逐步增加
- k6 的 `ADMIN_TOKEN` 可避免登录接口成为瓶颈，专注测试业务接口
- 如需修改 API 路径或请求体，直接编辑 `k6-http.js` 中的对应部分
