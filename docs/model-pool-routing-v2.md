# LLM Gateway 模型池科学路由改造记录

日期: 2026-08-31
对象: 10.100.1.17 LLM Gateway（双实例 llm-gateway@1/2，nginx :8085）

## 背景
用户反馈 AUTO 池中的 glm-5.2 从未被选中使用。排查确认根因:
- deepseek-v4-flash 是 DEFAULT_MODEL，router.New() 内置增强分
  (Capability 0.82 / Cost 0.90 / Latency 0.92 / Health 0.95 → 总评 0.877)
- 其余池模型 (glm-5.2 等) 经 RegisterProductionModel 注册，硬编码基础分
  (0.8/0.8/0.8/0.8 → 总评 0.800)
- 评分公式 Capability 权重最高 (0.45)，deepseek 恒胜，glm 永远落选

## 改动清单（全部在 17 号机工作树）

### 1. 评分公平化（internal/router/router.go RegisterProductionModel）
- 池模型注册分从 0.8 提到与 defaultModel 相同的增强分
  (0.82/0.90/0.92/0.95)，让 AUTO 池所有模型从公平起点竞争
- 备份: internal/router/router.go.bak-poolscore-20260830-184139

### 2. Channel-aware 同分选择（新增 selectBestChannelAware）
- 移除临时随机 tie-break（math/rand），改为在 Decide 中:
  同分候选优先选「有 enabled channel」的模型，无 channel 才取首个
- 保证 prefix affinity 测试通过且路由更可靠
- 备份: internal/router/router.go.bak-tiebreak-20260830-184638

### 3. 任务识别增强（classifyTask）
- 原只识别 code/analysis/general 三类
- 现识别: code / math / translate / summarize / reasoning / creative / analysis / general
- 优先级: TaskHint 头 > translate > code > math > summarize > reasoning > creative > analysis > general
- 中英文关键词覆盖（代码/翻译/数学/摘要/推理/写作/分析）
- 修复: translate 请求含 "hello world" 被 code 分支误判的问题（translate 提前）

### 4. 模型能力画像 taskFit（scoreCandidates）
- 同任务内未命中模型 taskBoost=0.72，命中=1.0
- 未命中时按 targetTask 给擅长模型加 taskFit 系数:
  - code: glm / deepseek-v4-pro +0.08
  - math|reasoning: glm / minimax +0.06
  - analysis: deepseek-v4-pro / glm +0.06
  - translate: deepseek-v4-flash / sensenova +0.04
  - creative: glm / deepseek-v4-pro +0.05
- 实现"物尽其用": 代码任务倾向 glm，推理任务倾向 minimax/glm，翻译/摘要走快模型

### 5. 自适应反馈闭环（RecordFeedback 实现）
- 原为空实现，现维护每模型滑动窗口 (requests/successes/latency)
- ≥5 样本后更新 registry 的 HealthScore/LatencyScore:
  - Health = 0.3 + 0.7*成功率 (90%→1.0, 60%→0.55)
  - Latency = 1s 内→1.0，10s→~0.5，30s+→0.25
- 让评分自动进化: 表现好的模型自然上位，慢/失败模型降分
- 实测验证: glm 超时后健康分下降，通用请求转向快模型

### 6. 部署修正（systemd 双单元冲突）
- 发现 llm-gateway-1/2.service（旧独立单元）与 llm-gateway@1/2.service（模板单元）
  同时存在且互相抢占 8090/8091 端口，导致模板单元反复 bind 失败
- 处置: 禁用独立单元 (llm-gateway-1/2)，启用模板单元 (llm-gateway@1/2)
- 模板单元带 proxy.conf drop-in + ExecStartPre postgres 健康检查，是生产标准
- 验证: 132 channels 注册，5 默认池模型绑定 channel，/v1/models 正常

### 7. 监控（新增）
- /usr/local/bin/model_pool_report.py: 每小时聚合 usage_events
  输出 docs/model_pool_report.json（各模型成功率/延迟/p90/任务分布）
- cron: "17 * * * * python3 /usr/local/bin/model_pool_report.py"
- kimi_k3_monitor.sh 保持每 30 分钟探测（原有）

## 端到端验证结果（部署后实测）
- AUTO 通用: sensenova-6.8-flash-lite / deepseek-v4-flash 健康轮转，<4s
- CODE 任务: glm-5.2 优先 (task-fit boost: code model)
- MATH 任务: minimax-m3 / glm-5.2 (task-fit boost: reasoning model)
- TRANSLATE: sensenova (task-fit boost: fast general model)
- SUMMARIZE: sensenova (general weighted routing)
- 全部响应 finish=stop，内容非空

## 回滚方法
- 二进制备份: llm-gateway.bak-before-v2-20260830-192715
- 源码备份: *.bak-poolscore-* / *.bak-tiebreak-*
- 回滚: 停实例 → 恢复备份二进制 → 启动 → 验证 /v1/models

## 跨重启持久化（v2.1 — 2026-08-30 20:07）
### 问题
RecordFeedback 的评分窗口在进程内存，重启后丢失，AUTO 路由从 0 开始重新学习。

### 方案
- model_limits 表新增 health_score / latency_score / score_updated_at 三列
- limits.Store 提供 ListScores / SaveScore 方法读写持久化评分
- Router.RecordFeedback 每次更新评分后异步写库（provider='custom', channel='' 单行 upsert）
- 启动时 ListScores 加载 → SetModelScores 注入路由注册表

### 验证
- 实测写入: DB 中 3 模型评分健康（glm-5.2 health=0.65, deepseek-flash=0.58, sensenova-lite=0.53）
- 重启加载: 日志 "loaded persisted model scores", "count":3
- 重启后路由: 4/6 请求正常（2 超时属上游限速，路由选择正确）

### 回滚
- 二进制: llm-gateway.bak-before-persist-20260830
- 源码: patch_persist1/2/3.py 在 ~/scripts/（回滚见 patch_classify.py 模式）

## 遗留观察
- kimi-k3 (非 AUTO 池) 72h 成功率仅 37%，未入池，不影响 AUTO
- hy3 / deepseek-v4-flash-0731:free 成功率 0%（非池内），如需使用需先验证
