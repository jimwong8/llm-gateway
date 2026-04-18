# Admin UI 批量操作单项重试与详情展开 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有的统一批量任务状态面板中，为失败项增加单项“重试”、“展开”功能，提升可观测性和错误恢复体验。

**Architecture:** 
1. 在 `renderBatchJobPanel` 中，为每个失败项渲染对应的按钮和初始隐藏的 payload 内容区。
2. 使用事件委托，在面板外层监听“重试”和“展开”点击事件。
3. 点击“展开”切换内联区块的显示状态；点击“重试”则调用改造后的 `retryFailedBatchJobItems`，通过传入指定 ID 仅重试该项，重试期间置灰对应的操作按钮，成功后自动移除，失败则更新错误信息。

**Tech Stack:** 原生 HTML/CSS/JavaScript

---

## Chunk 1: 扩展失败项渲染组件与样式

### Task 1: 改造失败项渲染模板

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 在 `renderBatchJobPanel` 调整失败项模板**

为失败项增加按钮组和初始隐藏的详情区块：
```javascript
    $("batch-job-failures").innerHTML = failures.length
      ? failures
          .map(
            (item) => `<div class="batch-job-failure-item" data-failure-id="${item.id}">
              <div class="batch-job-failure-main">
                <strong>#${AdminComponents.escapeHtml(item.id)}</strong>
                <span class="batch-job-failure-reason">${AdminComponents.escapeHtml(item.reason || "失败")}</span>
                <div class="batch-job-failure-actions">
                  <button data-action="retry-single" data-id="${item.id}">重试</button>
                  <button data-action="toggle-details" data-id="${item.id}">展开</button>
                </div>
              </div>
              <div class="batch-job-failure-details" id="failure-details-${item.id}" hidden>
                <pre><code>${AdminComponents.escapeHtml(JSON.stringify(item.retry_payload, null, 2))}</code></pre>
              </div>
            </div>`
          )
          .join("")
      : '<div class="batch-job-empty">暂无失败项</div>';
```

- [ ] **Step 2: 验证语法无误**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: PASS

### Task 2: 增加失败项辅助样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 新增样式类**

需要为主体行和展开行增加样式，并修复原有 flex 布局问题。
```css
.batch-job-failure-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #fef2f2;
  border: 1px solid #fecaca;
}

.batch-job-failure-main {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
}

.batch-job-failure-reason {
  color: #991b1b;
  font-size: 12px;
  flex: 1;
}

.batch-job-failure-actions {
  display: flex;
  gap: 4px;
}

.batch-job-failure-actions button {
  height: 24px;
  padding: 0 8px;
  font-size: 12px;
  background: #fff;
  border: 1px solid #fca5a5;
  border-radius: 4px;
  color: #991b1b;
  cursor: pointer;
}

.batch-job-failure-actions button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.batch-job-failure-details {
  width: 100%;
  background: #fff;
  border: 1px solid #fecaca;
  border-radius: 4px;
  padding: 8px;
  overflow-x: auto;
}

.batch-job-failure-details pre {
  margin: 0;
  font-size: 11px;
  color: #374151;
}
```
注意：原 `.batch-job-failure-item` 样式需做适当覆盖。

- [ ] **Step 2: 提交代码**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): add retry and expand actions to failure items"
```

---

## Chunk 2: 交互与重试逻辑改造

### Task 3: 绑定事件委托与展开逻辑

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 绑定事件委托**

在 `bootstrap` 中，或统一的地方，为 `#batch-job-failures` 绑定 click 事件。

```javascript
    $("batch-job-failures")?.addEventListener("click", async (e) => {
      const btn = e.target.closest("button");
      if (!btn) return;
      const action = btn.dataset.action;
      const id = Number(btn.dataset.id);
      
      if (action === "toggle-details") {
        const detailsEl = $(`failure-details-${id}`);
        if (detailsEl) {
          detailsEl.hidden = !detailsEl.hidden;
          btn.textContent = detailsEl.hidden ? "展开" : "收起";
        }
      } else if (action === "retry-single") {
        btn.disabled = true;
        btn.textContent = "重试中...";
        try {
          await retryFailedBatchJobItems([id]);
        } catch (err) {
          AdminComponents.renderMessage($("message"), err.message || "重试单项失败", "error");
        }
      }
    });
```

- [ ] **Step 2: 调整 `retryFailedBatchJobItems` 支持单项重试**

修改 `retryFailedBatchJobItems` 函数签名，接受可选的 `targetIds` 数组：

```javascript
  async function retryFailedBatchJobItems(targetIds = null) {
    const message = $("message");
    let allFailures = Array.isArray(state.batchJob?.failures) ? state.batchJob.failures.slice() : [];
    
    // 若指定了 targets，过滤出要重试的项
    let failuresToRetry = targetIds 
      ? allFailures.filter(f => targetIds.includes(f.id))
      : allFailures;

    if (failuresToRetry.length === 0) {
      throw new Error("没有可重试的失败项");
    }

    const action = state.batchJob.action || (failuresToRetry[0] && failuresToRetry[0].action) || "batch";
    
    // 如果不是单项重试，才重置进度条。单项重试不重置总进度
    if (!targetIds) {
      createBatchJob(action, failuresToRetry.length);
    }

    let success = 0;
    for (let i = 0; i < failuresToRetry.length; i += 1) {
      const failure = failuresToRetry[i];
      try {
        // ... 原有根据 action 执行 api 调用逻辑 ...

        // 成功后，从总失败列表中移除该项
        allFailures = allFailures.filter(f => f.id !== failure.id);
        success += 1;
      } catch (err) {
        // 失败时，更新总失败列表中的原因此项
        const idx = allFailures.findIndex(f => f.id === failure.id);
        if (idx !== -1) {
          allFailures[idx].reason = err.message || "重试失败";
        }
      } finally {
        if (!targetIds) {
          updateBatchJobProgress({ completed: i + 1, current_id: failure.id, failures: allFailures });
        } else {
          // 单项重试直接更新
          updateBatchJobProgress({ failures: allFailures });
        }
      }
    }

    if (!targetIds) {
      finishBatchJob();
    }
    
    if (allFailures.length > 0) {
      AdminComponents.renderMessage(message, `重试完成：成功 ${success} 项，失败 ${allFailures.length} 项`, "error");
    } else {
      AdminComponents.renderMessage(message, `重试成功：共 ${success} 项`);
      // 全成功则关闭面板
      if(targetIds && state.batchJob) state.batchJob.visible = false;
      renderBatchJobPanel();
    }
    await render();
  }
```

注意原有 `retryFailedBatchJobItems` 中 `appendBatchJobFailure` 需要替换为更新列表的逻辑，以免每次重试失败新增记录。

- [ ] **Step 3: 语法检查**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: PASS

- [ ] **Step 4: 提交代码**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): implement single item retry and details toggle"
```

---

## Chunk 3: 验证与验收

### Task 4: 本地验证

- [ ] **Step 1: 验证功能**

手动构造部分失败的任务。
预期：
- 面板渲染带有 `展开` 和 `重试` 按钮
- 点击展开可查看 JSON
- 点击重试单项，按钮变 `重试中...`
- 若重试成功，该项从列表消失，消息提示成功
- 若全部重试成功，面板隐藏

## 回滚策略
回滚以下三个文件即可：
- `internal/httpserver/adminui/app.js`
- `internal/httpserver/adminui/styles.css`
