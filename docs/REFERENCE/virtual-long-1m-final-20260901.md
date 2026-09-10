# virtual-long-1m 最终状态 (2026-09-01)

## 16 项任务全部完成

| # | 项目 | 状态 |
|---|------|------|
| 1.1 | BlockingChannelSelector | done |
| 1.2 | chunk 失败重试 MaxRetries=3 | done |
| 1.3 | Channel gate 泄漏修复 | done |
| 2.1 | Worker 模式灰度 (LONG_CONTEXT_WORKERS=1) | done |
| 2.2 | Lease 恢复 + 优雅退出 | done |
| 3.1 | response_format 兜底 | done |
| 3.2 | 探测超时修复 | done |
| 3.3 | chunk=500 默认值 | done |
| 4.1 | Embedding 向量检索 + 回填 | done (82证据75回填 91%) |
| 4.2 | Verifier 阶段 | done |
| 5.1 | RUNBOOK 更新 | done |
| 5.2 | /metrics 暴露 (12 类指标) | done |
| 5.3 | stuck 接口 | done |
| 6.2 | 三档回归测试 | done |
| 7.1 | 架构复查 | done |
| 7.2 | NVIDIA 移除 | done |

## 2026-09-01 修复记录

### JSON Repair (map 输出含换行)
- map 模型输出含换行, json.Unmarshal 失败, chunk 永久 failed
- 修复: map_task_processor.go 加入 RepairMapJSON 兜底
- 备份: map_task_processor.go.bak-jsonrepair-20260901

### /metrics 端点
- 后端返回 307 到 /admin/ui, nginx 8085 缺 /metrics 代理
- 修复: server.go 添加 promhttp metricsHandler + nginx 添加 /metrics 代理块
- 备份: server.go.bak-metrics-20260901

### 源码覆盖事故
- 本地旧版 server.go scp 覆盖了远端新版, 构建失败
- 教训: 后续源码部署禁止用本地旧版覆盖远端

## 端口与端点
- 8085 (nginx LB): /v1/models, /v1/chat/completions, /v1/long-context/*, /admin/long-context/*, /metrics, /health
- 8090 (gateway @1): 全部路由, worker 模式 (1 worker)
- 8091 (gateway @2): 全部路由

## 回滚方案
- 二进制: /tmp/llm-gateway.20260901.final (当前生产版)
- 源码备份: server.go.bak-* (按场景命名)

## 关键限制
- AUTO 池: deepseek-v4-flash, glm-5.2 (不含 NVIDIA)
- 1M 虚拟模型: 完全走 1M 项目配置, 不污染 AUTO
- 每 channel RPM=30, 并发=3, 测试需间隔
- embedding 服务: 10.100.1.15:8002 (bge-base, 768d)
