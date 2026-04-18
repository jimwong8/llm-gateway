(function () {
  const state = {
    view: "assets",
    query: {
      tenant_id: "",
      task_type: "",
      source_model: "",
      tag: "",
      keyword: "",
      include_deleted: false,
      limit: 20,
      offset: 0,
    },
    selectedAssetID: 0,
    selectedAssetIDs: [],
    lastCreatedAssetID: 0,
    highlightTimer: 0,
    assetRowsByID: {},
    batchJob: null,
    loading: false,
  };

  const PRESET_STORAGE_KEY = "admin_ui_filter_presets_v1";
  const ADMIN_KEY_STORAGE_KEY = "admin_api_key";
  const ADMIN_KEY_LAST_ACTIVE_KEY = "admin_api_key_last_active_at";
  const ADMIN_KEY_SET_AT_KEY = "admin_api_key_set_at";
  const ADMIN_KEY_IDLE_TTL_MS = 30 * 60 * 1000;
  const ADMIN_KEY_WARNING_MS = 5 * 60 * 1000;
  const OPLOG_STORAGE_KEY = "admin_ui_operation_logs_v1";
  const OPLOG_LIMIT = 200;

  const $ = (id) => document.getElementById(id);

  function getAdminKeyRaw() {
    return sessionStorage.getItem(ADMIN_KEY_STORAGE_KEY) || "";
  }

  function getAdminKeyLastActiveAt() {
    const raw = Number(sessionStorage.getItem(ADMIN_KEY_LAST_ACTIVE_KEY) || 0);
    return Number.isFinite(raw) && raw > 0 ? raw : 0;
  }

  function getAdminKeyRemainingMs() {
    const lastActiveAt = getAdminKeyLastActiveAt();
    if (!lastActiveAt) return 0;
    return Math.max(0, lastActiveAt + ADMIN_KEY_IDLE_TTL_MS - Date.now());
  }

  function isAdminKeyExpired() {
    return !!getAdminKeyRaw() && getAdminKeyRemainingMs() <= 0;
  }

  function maskAdminKey(key) {
    const value = String(key || "");
    if (!value) return "--";
    if (value.length <= 6) return `${value.slice(0, 2)}***`;
    return `${value.slice(0, 3)}****${value.slice(-4)}`;
  }

  function renderAdminKeyStatus() {
    const statusEl = $("security-status");
    const maskEl = $("security-key-mask");
    const expiryEl = $("security-expiry");
    if (!statusEl || !maskEl || !expiryEl) return;

    const rawKey = getAdminKeyRaw();
    if (!rawKey) {
      statusEl.className = "security-status";
      statusEl.textContent = "未设置 Admin Key";
      maskEl.textContent = "--";
      expiryEl.textContent = "--";
      return;
    }

    const remainingMs = getAdminKeyRemainingMs();
    const remainingMinutes = Math.ceil(remainingMs / 60000);
    maskEl.textContent = maskAdminKey(rawKey);

    if (remainingMs <= 0) {
      statusEl.className = "security-status expired";
      statusEl.textContent = "Admin Key 已过期";
      expiryEl.textContent = "请重新设置";
      return;
    }

    if (remainingMs <= ADMIN_KEY_WARNING_MS) {
      statusEl.className = "security-status warning";
      statusEl.textContent = "Admin Key 即将过期";
      expiryEl.textContent = `剩余 ${remainingMinutes} 分钟`;
      return;
    }

    statusEl.className = "security-status active";
    statusEl.textContent = "Admin Key 已设置";
    expiryEl.textContent = `剩余 ${remainingMinutes} 分钟`;
  }

  function touchAdminKeyActivity() {
    if (!getAdminKeyRaw()) return;
    sessionStorage.setItem(ADMIN_KEY_LAST_ACTIVE_KEY, String(Date.now()));
    renderAdminKeyStatus();
  }

  function setAdminKeyValue(input) {
    const key = String(input || "").trim();
    if (!key) return "";
    const now = Date.now();
    sessionStorage.setItem(ADMIN_KEY_STORAGE_KEY, key);
    sessionStorage.setItem(ADMIN_KEY_SET_AT_KEY, String(now));
    sessionStorage.setItem(ADMIN_KEY_LAST_ACTIVE_KEY, String(now));
    renderAdminKeyStatus();
    return key;
  }

  function clearAdminKeyStorage() {
    sessionStorage.removeItem(ADMIN_KEY_STORAGE_KEY);
    sessionStorage.removeItem(ADMIN_KEY_SET_AT_KEY);
    sessionStorage.removeItem(ADMIN_KEY_LAST_ACTIVE_KEY);
    renderAdminKeyStatus();
  }

  function promptAdminKeyUpdate() {
    const input = window.prompt("请输入 Admin API Key（仅保存在当前浏览器会话）", getAdminKeyRaw());
    if (!input || !input.trim()) return "";
    return setAdminKeyValue(input);
  }

  function getAdminKey() {
    const current = getAdminKeyRaw();
    if (!current) return "";
    if (isAdminKeyExpired()) {
      clearAdminKeyStorage();
      return "";
    }
    return current;
  }

  function ensureAdminKey() {
    if (getAdminKeyRaw() && isAdminKeyExpired()) {
      clearAdminKeyStorage();
      throw new Error("Admin Key 已过期，请点击“设置/更新”");
    }
    const current = getAdminKey();
    if (current) return current;
    throw new Error("未设置 Admin Key，请点击“设置/更新”");
  }

  function normalizeError(resp, data) {
    const msg = data?.error?.message || `请求失败(${resp.status})`;
    if (resp.status === 401) return "管理密钥无效，请刷新页面后重新输入";
    if (resp.status === 404) return "资源不存在或已删除，已建议刷新当前列表";
    if (resp.status >= 500) return `服务异常：${msg}`;
    return msg;
  }

  async function apiRequest(path, options = {}) {
    const key = ensureAdminKey();
    if (!key) throw new Error("未提供管理密钥，无法请求管理接口");

    const headers = Object.assign({}, options.headers || {}, { "X-Admin-Key": key });
    const resp = await fetch(path, {
      method: options.method || "GET",
      headers,
      body: options.body,
    });

    let data = {};
    try {
      data = await resp.json();
    } catch {
      data = {};
    }

    if (!resp.ok) {
      renderAdminKeyStatus();
      throw new Error(normalizeError(resp, data));
    }
    touchAdminKeyActivity();
    return data;
  }

  function buildQueryString(query) {
    const qs = new URLSearchParams();
    Object.entries(query || {}).forEach(([k, v]) => {
      if (v === "" || v === undefined || v === null) return;
      if (k === "include_deleted") {
        if (v) qs.set(k, "true");
        return;
      }
      qs.set(k, String(v));
    });
    return qs.toString();
  }

  async function apiGet(path, query) {
    const qs = buildQueryString(query);
    const url = qs ? `${path}?${qs}` : path;
    return apiRequest(url);
  }

  async function apiPost(path, body) {
    return apiRequest(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  }

  async function apiPut(path, body) {
    return apiRequest(path, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  }

  async function apiDelete(path, query) {
    const qs = buildQueryString(query);
    const url = qs ? `${path}?${qs}` : path;
    return apiRequest(url, { method: "DELETE" });
  }

  function csvEscape(value) {
    const raw = value === undefined || value === null ? "" : String(value);
    const escaped = raw.replace(/"/g, '""');
    return /[",\n]/.test(escaped) ? `"${escaped}"` : escaped;
  }

  function triggerDownload(filename, content, mimeType) {
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  function generateOperationLogID() {
    return `op_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
  }

  function readOperationLogs() {
    try {
      const raw = window.localStorage.getItem(OPLOG_STORAGE_KEY);
      const logs = raw ? JSON.parse(raw) : [];
      return Array.isArray(logs) ? logs : [];
    } catch {
      return [];
    }
  }

  function writeOperationLogs(logs) {
    const next = Array.isArray(logs) ? logs.slice(0, OPLOG_LIMIT) : [];
    window.localStorage.setItem(OPLOG_STORAGE_KEY, JSON.stringify(next));
  }

  function buildOperationLogEntry(action, status, summary, payload) {
    return {
      id: generateOperationLogID(),
      action: String(action || "unknown"),
      status: status === "error" ? "error" : "success",
      summary: String(summary || ""),
      payload: payload || {},
      created_at: new Date().toISOString(),
    };
  }

  function appendOperationLog(action, status, summary, payload) {
    const logs = readOperationLogs();
    logs.unshift(buildOperationLogEntry(action, status, summary, payload));
    writeOperationLogs(logs);
    renderOperationLogs();
  }

  function getOperationLogFilterState() {
    return {
      action: $("operation-log-action-filter")?.value || "",
      keyword: ($("operation-log-search")?.value || "").toLowerCase()
    };
  }

  function filterOperationLogs(logs, filterState) {
    return logs.filter((log) => {
      if (filterState.action && log.action !== filterState.action) {
        return false;
      }
      if (filterState.keyword) {
        const haystack = [log.action, log.summary, JSON.stringify(log.payload || {})]
          .join(" ")
          .toLowerCase();
        if (haystack.indexOf(filterState.keyword) === -1) {
          return false;
        }
      }
      return true;
    });
  }

  function renderOperationLogs() {
    const el = $("operation-log-list");
    if (!el) return;
    
    const allLogs = readOperationLogs();
    const filterState = getOperationLogFilterState();
    const filteredLogs = filterOperationLogs(allLogs, filterState);
    const logsToRender = filteredLogs.slice(0, 20);

    el.innerHTML = logsToRender.length
      ? logsToRender
          .map(
            (log) => `<div class="operation-log-item ${AdminComponents.escapeHtml(log.status || "success")}">
              <div><strong>${AdminComponents.escapeHtml(log.action || "unknown")}</strong></div>
              <div>${AdminComponents.escapeHtml(log.summary || "")}</div>
              <div class="operation-log-meta">${AdminComponents.escapeHtml(log.created_at || "")}</div>
            </div>`
          )
          .join("")
      : '<div class="operation-log-empty">暂无匹配的操作日志</div>';
  }

  function exportOperationLogs(format) {
    const logs = readOperationLogs();
    if (logs.length === 0) {
      throw new Error("导出操作日志失败：当前没有可导出的日志");
    }

    const ts = new Date().toISOString().replace(/[:.]/g, "-");
    if (format === "json") {
      triggerDownload(`admin-operation-logs-${ts}.json`, `${JSON.stringify(logs, null, 2)}\n`, "application/json;charset=utf-8");
      return;
    }

    const headers = ["id", "action", "created_at", "status", "summary", "payload"];
    const csvRows = logs.map((log) =>
      [log.id, log.action, log.created_at, log.status, log.summary, JSON.stringify(log.payload || {})]
        .map(csvEscape)
        .join(",")
    );
    triggerDownload(`admin-operation-logs-${ts}.csv`, `\uFEFF${headers.join(",")}\n${csvRows.join("\n")}\n`, "text/csv;charset=utf-8");
  }

  async function exportCurrentAssets(format) {
    const message = $("message");
    const data = await apiGet("/admin/assets", state.query);
    const rows = data.data || [];

    if (rows.length === 0) {
      throw new Error("导出失败：当前筛选无可导出资产");
    }

    const mapped = rows.map((row) => ({
      id: row.id,
      tenant_id: row.tenant_id || "",
      title: row.title || "",
      summary: row.summary || "",
      task_type: row.task_type || "",
      source_model: row.source_model || "",
      tags: Array.isArray(row.tags) ? row.tags : [],
      hit_count: row.hit_count || 0,
      is_deleted: !!row.is_deleted,
      created_at: row.created_at || "",
      updated_at: row.updated_at || "",
    }));

    const ts = new Date().toISOString().replace(/[:.]/g, "-");
    if (format === "json") {
      const content = `${JSON.stringify(mapped, null, 2)}\n`;
      triggerDownload(`assets-export-${ts}.json`, content, "application/json;charset=utf-8");
      appendOperationLog("assets.export.json", "success", `导出资产 JSON ${mapped.length} 条`, { count: mapped.length });
      AdminComponents.renderMessage(message, `导出成功：JSON ${mapped.length} 条`);
      return;
    }

    const headers = [
      "id",
      "tenant_id",
      "title",
      "summary",
      "task_type",
      "source_model",
      "tags",
      "hit_count",
      "is_deleted",
      "created_at",
      "updated_at",
    ];
    const csvRows = mapped.map((row) =>
      [
        row.id,
        row.tenant_id,
        row.title,
        row.summary,
        row.task_type,
        row.source_model,
        row.tags.join("|"),
        row.hit_count,
        row.is_deleted,
        row.created_at,
        row.updated_at,
      ]
        .map(csvEscape)
        .join(",")
    );
    const content = `\uFEFF${headers.join(",")}\n${csvRows.join("\n")}\n`;
    triggerDownload(`assets-export-${ts}.csv`, content, "text/csv;charset=utf-8");
    appendOperationLog("assets.export.csv", "success", `导出资产 CSV ${mapped.length} 条`, { count: mapped.length });
    AdminComponents.renderMessage(message, `导出成功：CSV ${mapped.length} 条`);
  }

  function generateOperationLogID() {
    return `op_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
  }

  function readOperationLogs() {
    try {
      const raw = window.localStorage.getItem(OPLOG_STORAGE_KEY);
      const logs = raw ? JSON.parse(raw) : [];
      return Array.isArray(logs) ? logs : [];
    } catch {
      return [];
    }
  }

  function writeOperationLogs(logs) {
    const next = Array.isArray(logs) ? logs.slice(0, OPLOG_LIMIT) : [];
    window.localStorage.setItem(OPLOG_STORAGE_KEY, JSON.stringify(next));
  }

  function buildOperationLogEntry(action, status, summary, payload) {
    return {
      id: generateOperationLogID(),
      action: String(action || "unknown"),
      status: status === "error" ? "error" : "success",
      summary: String(summary || ""),
      payload: payload || {},
      created_at: new Date().toISOString(),
    };
  }

  function appendOperationLog(action, status, summary, payload) {
    const logs = readOperationLogs();
    logs.unshift(buildOperationLogEntry(action, status, summary, payload));
    writeOperationLogs(logs);
    renderOperationLogs();
  }

  function renderOperationLogs() {
    const el = $("operation-log-list");
    if (!el) return;
    const logs = readOperationLogs().slice(0, 10);
    el.innerHTML = logs.length
      ? logs
          .map(
            (log) => `<div class="operation-log-item ${AdminComponents.escapeHtml(log.status || "success")}">
              <div><strong>${AdminComponents.escapeHtml(log.action || "unknown")}</strong></div>
              <div>${AdminComponents.escapeHtml(log.summary || "")}</div>
              <div class="operation-log-meta">${AdminComponents.escapeHtml(log.created_at || "")}</div>
            </div>`
          )
          .join("")
      : '<div class="operation-log-empty">暂无操作日志</div>';
  }

  function exportOperationLogs(format) {
    const logs = readOperationLogs();
    if (logs.length === 0) {
      throw new Error("导出操作日志失败：当前没有可导出的日志");
    }

    const ts = new Date().toISOString().replace(/[:.]/g, "-");
    if (format === "json") {
      triggerDownload(`admin-operation-logs-${ts}.json`, `${JSON.stringify(logs, null, 2)}\n`, "application/json;charset=utf-8");
      return;
    }

    const headers = ["id", "action", "created_at", "status", "summary", "payload"];
    const csvRows = logs.map((log) =>
      [log.id, log.action, log.created_at, log.status, log.summary, JSON.stringify(log.payload || {})]
        .map(csvEscape)
        .join(",")
    );
    triggerDownload(`admin-operation-logs-${ts}.csv`, `\uFEFF${headers.join(",")}\n${csvRows.join("\n")}\n`, "text/csv;charset=utf-8");
  }

  function readQueryFromURL() {
    const params = new URLSearchParams(window.location.search);
    state.view = params.get("view") || "assets";
    state.query.tenant_id = params.get("tenant_id") || "";
    state.query.task_type = params.get("task_type") || "";
    state.query.source_model = params.get("source_model") || "";
    state.query.tag = params.get("tag") || "";
    state.query.keyword = params.get("keyword") || "";
    state.query.include_deleted = ["1", "true", "yes", "on"].includes((params.get("include_deleted") || "").toLowerCase());
    state.query.limit = Number(params.get("limit") || 20);
    state.query.offset = Number(params.get("offset") || 0);
    if (Number.isNaN(state.query.limit) || state.query.limit <= 0) state.query.limit = 20;
    if (Number.isNaN(state.query.offset) || state.query.offset < 0) state.query.offset = 0;
  }

  function writeQueryToURL() {
    const params = new URLSearchParams();
    params.set("view", state.view);
    Object.entries(state.query).forEach(([k, v]) => {
      if (v === "" || v === undefined || v === null) return;
      if (k === "include_deleted") {
        if (v) params.set(k, "true");
        return;
      }
      params.set(k, String(v));
    });
    const next = `${window.location.pathname}?${params.toString()}`;
    window.history.replaceState({}, "", next);
  }

  function syncQueryFromForm() {
    state.query.tenant_id = $("tenant_id").value.trim();
    state.query.task_type = $("task_type").value.trim();
    state.query.source_model = $("source_model").value.trim();
    state.query.tag = $("tag").value.trim();
    state.query.keyword = $("keyword").value.trim();
    state.query.include_deleted = $("include_deleted").checked;
    state.query.limit = Number($("limit").value || 20);
    state.query.offset = 0;
  }

  function applyQueryToForm() {
    $("tenant_id").value = state.query.tenant_id;
    $("task_type").value = state.query.task_type;
    $("source_model").value = state.query.source_model;
    $("tag").value = state.query.tag;
    $("keyword").value = state.query.keyword;
    $("include_deleted").checked = !!state.query.include_deleted;
    $("limit").value = String(state.query.limit || 20);
  }

  function snapshotPresetQuery() {
    return {
      tenant_id: state.query.tenant_id || "",
      task_type: state.query.task_type || "",
      source_model: state.query.source_model || "",
      tag: state.query.tag || "",
      keyword: state.query.keyword || "",
      include_deleted: !!state.query.include_deleted,
      limit: Number(state.query.limit || 20),
    };
  }

  function readFilterPresets() {
    try {
      const raw = window.localStorage.getItem(PRESET_STORAGE_KEY);
      const list = raw ? JSON.parse(raw) : [];
      return Array.isArray(list) ? list : [];
    } catch {
      return [];
    }
  }

  function writeFilterPresets(presets) {
    window.localStorage.setItem(PRESET_STORAGE_KEY, JSON.stringify((presets || []).slice(0, 20)));
  }

  function renderFilterPresetOptions(selectedName = "") {
    const select = $("preset_list");
    if (!select) return;
    const presets = readFilterPresets();
    const options = ['<option value="">选择筛选预设</option>']
      .concat(
        presets.map((preset) => {
          const selected = preset.name === selectedName ? " selected" : "";
          return `<option value="${AdminComponents.escapeHtml(preset.name)}"${selected}>${AdminComponents.escapeHtml(preset.name)}</option>`;
        })
      )
      .join("");
    select.innerHTML = options;
  }

  function saveCurrentFilterPreset() {
    syncQueryFromForm();
    const name = $("preset_name")?.value.trim() || "";
    if (!name) {
      throw new Error("保存预设失败：请输入预设名称");
    }
    const presets = readFilterPresets().filter((preset) => preset.name !== name);
    presets.unshift({
      name,
      query: snapshotPresetQuery(),
      updated_at: new Date().toISOString(),
    });
    writeFilterPresets(presets);
    renderFilterPresetOptions(name);
    $("preset_list").value = name;
    AdminComponents.renderMessage($("message"), `筛选预设已保存：${name}`);
  }

  function loadSelectedFilterPreset() {
    const name = $("preset_list")?.value || "";
    if (!name) {
      throw new Error("加载预设失败：请先选择一个预设");
    }
    const preset = readFilterPresets().find((item) => item.name === name);
    if (!preset || !preset.query) {
      throw new Error("加载预设失败：未找到对应预设");
    }
    state.query.tenant_id = preset.query.tenant_id || "";
    state.query.task_type = preset.query.task_type || "";
    state.query.source_model = preset.query.source_model || "";
    state.query.tag = preset.query.tag || "";
    state.query.keyword = preset.query.keyword || "";
    state.query.include_deleted = !!preset.query.include_deleted;
    state.query.limit = Number(preset.query.limit || 20);
    state.query.offset = 0;
    applyQueryToForm();
    writeQueryToURL();
    AdminComponents.renderMessage($("message"), `筛选预设已加载：${name}`);
    render();
  }

  function deleteSelectedFilterPreset() {
    const name = $("preset_list")?.value || "";
    if (!name) {
      throw new Error("删除预设失败：请先选择一个预设");
    }
    const presets = readFilterPresets();
    const next = presets.filter((preset) => preset.name !== name);
    if (next.length === presets.length) {
      throw new Error("删除预设失败：未找到对应预设");
    }
    writeFilterPresets(next);
    renderFilterPresetOptions("");
    $("preset_name").value = "";
    AdminComponents.renderMessage($("message"), `筛选预设已删除：${name}`);
  }

  function emptyBatchJob() {
    return {
      visible: false,
      action: "",
      total: 0,
      completed: 0,
      failed: 0,
      current_id: 0,
      running: false,
      failures: [],
    };
  }

  function batchActionLabel(action) {
    if (action === "tags") return "批量标签";
    if (action === "rollback") return "批量回滚";
    if (action === "delete") return "批量软删除";
    return "批量任务";
  }

  function createBatchJob(action, total) {
    state.batchJob = {
      visible: true,
      action,
      total,
      completed: 0,
      failed: 0,
      current_id: 0,
      running: true,
      failures: [],
    };
    renderBatchJobPanel();
  }

  function updateBatchJobProgress(partial) {
    state.batchJob = Object.assign(emptyBatchJob(), state.batchJob || {}, partial || {});
    state.batchJob.failed = Array.isArray(state.batchJob.failures) ? state.batchJob.failures.length : 0;
    renderBatchJobPanel();
  }

  function appendBatchJobFailure(failure) {
    const failures = Array.isArray(state.batchJob?.failures) ? state.batchJob.failures.slice() : [];
    failures.push(failure);
    updateBatchJobProgress({ failures, failed: failures.length });
  }

  function finishBatchJob() {
    if (!state.batchJob) return;
    updateBatchJobProgress({
      running: false,
      current_id: 0,
      failed: Array.isArray(state.batchJob.failures) ? state.batchJob.failures.length : 0,
    });
  }

  function renderBatchJobPanel() {
    const panel = $("batch-job-panel");
    if (!panel) return;
    if (!state.batchJob || !state.batchJob.visible) {
      panel.hidden = true;
      return;
    }

    panel.hidden = false;
    const total = Number(state.batchJob.total || 0);
    const completed = Number(state.batchJob.completed || 0);
    const failed = Number(state.batchJob.failed || 0);
    const percent = total > 0 ? Math.min(100, Math.round((completed / total) * 100)) : 0;
    const current = state.batchJob.current_id ? `当前处理：asset=${state.batchJob.current_id}` : "当前处理：-";
    const failures = Array.isArray(state.batchJob.failures) ? state.batchJob.failures : [];

    $("batch-job-summary").textContent = `${batchActionLabel(state.batchJob.action)}｜总数 ${total}｜已完成 ${completed}｜失败 ${failed}`;
    $("batch-job-current").textContent = current;
    $("batch-job-progress-bar").style.width = `${percent}%`;
    $("batch-job-progress-bar").textContent = `${percent}%`;
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

    const retryBtn = $("batch-job-retry-btn");
    if (retryBtn) {
      retryBtn.disabled = state.batchJob.running || failures.length === 0;
    }
  }

  async function retryFailedBatchJobItems(targetIds = null) {
    const message = $("message");
    let allFailures = Array.isArray(state.batchJob?.failures) ? state.batchJob.failures.slice() : [];
    
    let failuresToRetry = targetIds
      ? allFailures.filter(f => targetIds.includes(f.id))
      : allFailures;

    if (failuresToRetry.length === 0) {
      throw new Error("没有可重试的失败项");
    }

    const action = state.batchJob.action || (failuresToRetry[0] && failuresToRetry[0].action) || "batch";
    
    if (!targetIds) {
      createBatchJob(action, failuresToRetry.length);
    }

    let success = 0;
    for (let i = 0; i < failuresToRetry.length; i += 1) {
      const failure = failuresToRetry[i];
      try {
        if (failure.action === "tags") {
          await apiPut("/admin/assets", failure.retry_payload);
        } else if (failure.action === "rollback") {
          const payload = Object.assign({}, failure.retry_payload || {});
          if (payload.mode === "previous") {
            const versions = await apiGet("/admin/assets/versions", {
              tenant_id: payload.tenant_id,
              asset_id: payload.asset_id,
              limit: 100,
              offset: 0,
            });
            payload.version = resolvePreviousVersionNumber(versions.data || []);
          }
          await apiPost("/admin/assets/rollback", {
            tenant_id: payload.tenant_id,
            asset_id: payload.asset_id,
            version: payload.version,
          });
        } else if (failure.action === "delete") {
          await apiDelete("/admin/assets", failure.retry_payload);
        }
        allFailures = allFailures.filter(f => f.id !== failure.id);
        success += 1;
      } catch (err) {
        const idx = allFailures.findIndex(f => f.id === failure.id);
        if (idx !== -1) {
          allFailures[idx].reason = err.message || "重试失败";
        }
      } finally {
        if (!targetIds) {
          updateBatchJobProgress({ completed: i + 1, current_id: failure.id, failures: allFailures });
        } else {
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
      if(targetIds && state.batchJob) state.batchJob.visible = false;
      renderBatchJobPanel();
    }
    await render();
  }

  function setLoading(loading) {
    state.loading = loading;
    $("refreshBtn").disabled = loading;
  }

  function setView(view) {
    state.view = view;
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.view === view);
    });
    const actionBar = $("asset-actions");
    if (actionBar) actionBar.hidden = view !== "assets";
    writeQueryToURL();
    render();
  }

  function bindNav() {
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.addEventListener("click", () => setView(btn.dataset.view));
    });
  }

  function bindFilters() {
    $("refreshBtn").addEventListener("click", () => {
      syncQueryFromForm();
      writeQueryToURL();
      render();
    });

    ["tenant_id", "task_type", "source_model", "tag", "keyword", "include_deleted", "limit"].forEach((id) => {
      const el = $(id);
      if (!el) return;
      const evt = el.tagName === "INPUT" && el.type !== "checkbox" ? "keydown" : "change";
      el.addEventListener(evt, (e) => {
        if (evt === "keydown" && e.key !== "Enter") return;
        syncQueryFromForm();
        writeQueryToURL();
        render();
      });
    });
  }

  async function renderAssets() {
    const view = $("view");
    const message = $("message");

    const data = await apiGet("/admin/assets", state.query);
    const rows = data.data || [];
    state.assetRowsByID = Object.fromEntries(rows.map((row) => [Number(row.id), row]));

    const selectableIDs = rows.filter((row) => !row.is_deleted).map((row) => Number(row.id));
    const selectableIDSet = new Set(selectableIDs);
    state.selectedAssetIDs = state.selectedAssetIDs.filter((id) => selectableIDSet.has(Number(id)));

    const selectedCountEl = $("batch-selected-count");
    const batchDeleteBtn = $("batch-delete-btn");
    const batchTagsBtn = $("batch-tags-open-btn");
    const batchRollbackBtn = $("batch-rollback-open-btn");
    if (selectedCountEl) selectedCountEl.textContent = `已选 ${state.selectedAssetIDs.length} 项`;
    if (batchDeleteBtn) batchDeleteBtn.disabled = state.selectedAssetIDs.length === 0;
    if (batchTagsBtn) batchTagsBtn.disabled = state.selectedAssetIDs.length === 0;
    if (batchRollbackBtn) batchRollbackBtn.disabled = state.selectedAssetIDs.length === 0;

    const tableHTML = `<table class="table"><thead><tr>
      <th class="check-col"><input id="asset-select-all" type="checkbox" ${selectableIDs.length === 0 ? "disabled" : ""} /></th>
      <th>ID</th><th>Tenant</th><th>Title</th><th>Task</th><th>Model</th><th>Hit</th><th>Status</th><th>Actions</th>
    </tr></thead><tbody>${rows
      .map((row) => {
        const id = Number(row.id);
        const checked = state.selectedAssetIDs.includes(id) ? "checked" : "";
        const disabled = row.is_deleted ? "disabled" : "";
        const rowClass = Number(row.id) === Number(state.lastCreatedAssetID) ? "row-highlight" : "";
        return `<tr class="${rowClass}">
          <td class="check-col"><input type="checkbox" data-action="select" data-id="${id}" ${checked} ${disabled} /></td>
          <td>${AdminComponents.escapeHtml(row.id)}</td>
          <td>${AdminComponents.escapeHtml(row.tenant_id || "")}</td>
          <td>${AdminComponents.escapeHtml(row.title || "")}</td>
          <td>${AdminComponents.escapeHtml(row.task_type || "")}</td>
          <td>${AdminComponents.escapeHtml(row.source_model || "")}</td>
          <td>${AdminComponents.escapeHtml(row.hit_count || 0)}</td>
          <td>${row.is_deleted ? '<span class="badge deleted">deleted</span>' : '<span class="badge">active</span>'}</td>
          <td class="actions">
            <button data-action="versions" data-id="${row.id}">版本</button>
            <button data-action="edit" data-id="${row.id}">编辑</button>
            ${row.is_deleted ? "" : `<button data-action="delete" data-id="${row.id}">软删除</button>`}
          </td>
        </tr>`;
      })
      .join("") || '<tr><td colspan="9">暂无数据</td></tr>'}</tbody></table>`;

    view.innerHTML = tableHTML;

    const selectAllEl = $("asset-select-all");
    if (selectAllEl) {
      const selectedCount = state.selectedAssetIDs.length;
      selectAllEl.checked = selectableIDs.length > 0 && selectedCount === selectableIDs.length;
      selectAllEl.indeterminate = selectedCount > 0 && selectedCount < selectableIDs.length;
      selectAllEl.addEventListener("change", () => {
        state.selectedAssetIDs = selectAllEl.checked ? selectableIDs.slice() : [];
        if (selectedCountEl) selectedCountEl.textContent = `已选 ${state.selectedAssetIDs.length} 项`;
        if (batchDeleteBtn) batchDeleteBtn.disabled = state.selectedAssetIDs.length === 0;
        if (batchTagsBtn) batchTagsBtn.disabled = state.selectedAssetIDs.length === 0;
        if (batchRollbackBtn) batchRollbackBtn.disabled = state.selectedAssetIDs.length === 0;
        view.querySelectorAll('input[data-action="select"]').forEach((cb) => {
          cb.checked = state.selectedAssetIDs.includes(Number(cb.dataset.id));
        });
      });
    }

    view.querySelectorAll('input[data-action="select"]').forEach((cb) => {
      cb.addEventListener("change", () => {
        const id = Number(cb.dataset.id);
        if (cb.checked) {
          if (!state.selectedAssetIDs.includes(id)) state.selectedAssetIDs.push(id);
        } else {
          state.selectedAssetIDs = state.selectedAssetIDs.filter((v) => v !== id);
        }

        const selectedCount = state.selectedAssetIDs.length;
        if (selectedCountEl) selectedCountEl.textContent = `已选 ${selectedCount} 项`;
        if (batchDeleteBtn) batchDeleteBtn.disabled = selectedCount === 0;
        if (batchTagsBtn) batchTagsBtn.disabled = selectedCount === 0;
        if (batchRollbackBtn) batchRollbackBtn.disabled = selectedCount === 0;
        if (selectAllEl) {
          selectAllEl.checked = selectableIDs.length > 0 && selectedCount === selectableIDs.length;
          selectAllEl.indeterminate = selectedCount > 0 && selectedCount < selectableIDs.length;
        }
      });
    });

    const pager = AdminComponents.renderPager({
      limit: state.query.limit,
      offset: state.query.offset,
      count: rows.length,
      onPrev: () => {
        state.query.offset = Math.max(0, state.query.offset - state.query.limit);
        writeQueryToURL();
        render();
      },
      onNext: () => {
        state.query.offset = state.query.offset + state.query.limit;
        writeQueryToURL();
        render();
      },
    });
    view.appendChild(pager);

    view.querySelectorAll('button[data-action="versions"]').forEach((btn) => {
      btn.addEventListener("click", async () => {
        state.selectedAssetID = Number(btn.dataset.id);
        try {
          await openVersions();
        } catch (err) {
          AdminComponents.renderMessage(message, err.message || "版本加载失败", "error");
        }
      });
    });

    view.querySelectorAll('button[data-action="edit"]').forEach((btn) => {
      btn.addEventListener("click", () => {
        const id = Number(btn.dataset.id);
        const row = state.assetRowsByID[id];
        if (!row) {
          AdminComponents.renderMessage(message, `未找到资产 ${id} 的当前数据`, "error");
          return;
        }
        openEditModal(row);
      });
    });

    view.querySelectorAll('button[data-action="delete"]').forEach((btn) => {
      btn.addEventListener("click", async () => {
        const id = Number(btn.dataset.id);
        const row = state.assetRowsByID[id];
        if (!row) return;

        const ok = window.confirm(`确认软删除资产 ${id} 吗？`);
        if (!ok) return;

        btn.disabled = true;
        try {
          await apiDelete("/admin/assets", {
            id,
            tenant_id: row.tenant_id || state.query.tenant_id,
          });
          appendOperationLog("asset.delete", "success", `删除资产 ${id}`, {
            asset_id: id,
            tenant_id: row.tenant_id || state.query.tenant_id
          });
          state.selectedAssetIDs = state.selectedAssetIDs.filter((v) => v !== id);
          AdminComponents.renderMessage(message, `软删除成功：asset=${id}`);
          await render();
        } catch (err) {
          appendOperationLog("asset.delete", "error", `删除资产 ${id} 失败`, {
            asset_id: id,
            tenant_id: row.tenant_id || state.query.tenant_id,
            error: err.message
          });
          AdminComponents.renderMessage(message, err.message || "软删除失败", "error");
        } finally {
          btn.disabled = false;
        }
      });
    });

    if (state.lastCreatedAssetID) {
      if (state.highlightTimer) window.clearTimeout(state.highlightTimer);
      state.highlightTimer = window.setTimeout(() => {
        state.lastCreatedAssetID = 0;
        state.highlightTimer = 0;
        if (state.view === "assets") render();
      }, 6000);
    }
  }

  function openBatchTagsModal() {
    const ids = state.selectedAssetIDs.slice();
    if (ids.length === 0) {
      throw new Error("请先选择至少 1 个未删除资产");
    }
    $("batch-tags-selected-hint").textContent = `当前已选 ${ids.length} 项`;
    $("batch-tags-mode").value = "append";
    $("batch-tags-input").value = "";
    $("batch-tags-modal").hidden = false;
  }

  function closeBatchTagsModal() {
    $("batch-tags-modal").hidden = true;
  }

  function mergeTags(existingTags, inputTags, mode) {
    const current = Array.isArray(existingTags)
      ? existingTags.map((t) => String(t || "").trim()).filter((t) => t.length > 0)
      : [];
    const incoming = Array.isArray(inputTags)
      ? inputTags.map((t) => String(t || "").trim()).filter((t) => t.length > 0)
      : [];

    if (mode === "replace") return Array.from(new Set(incoming));
    return Array.from(new Set([...current, ...incoming]));
  }

  async function batchApplyTagsSelected() {
    const message = $("message");
    const ids = state.selectedAssetIDs.slice();
    if (ids.length === 0) return;

    const mode = $("batch-tags-mode").value === "replace" ? "replace" : "append";
    const inputTags = parseTagsInput($("batch-tags-input").value);
    if (inputTags.length === 0) {
      throw new Error("批量标签失败：请至少输入 1 个 tag");
    }

    const btn = $("batch-tags-save");
    if (btn) btn.disabled = true;

    createBatchJob("tags", ids.length);
    let success = 0;

    for (let i = 0; i < ids.length; i += 1) {
      const id = ids[i];
      const row = state.assetRowsByID[id];
      if (!row || row.is_deleted) {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
        continue;
      }

      const payload = {
        id: Number(row.id),
        tenant_id: row.tenant_id || state.query.tenant_id,
        user_id: row.user_id || "",
        session_id: row.session_id || "",
        source_model: row.source_model || "",
        task_type: row.task_type || "",
        title: row.title || "",
        summary: row.summary || "",
        tags: mergeTags(row.tags, inputTags, mode),
        source_request_id: row.source_request_id || "",
      };

      try {
        await apiPut("/admin/assets", payload);
        success += 1;
      } catch (err) {
        appendBatchJobFailure({
          id,
          action: "tags",
          reason: err.message || "更新失败",
          retry_payload: payload,
        });
      } finally {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
      }
    }

    finishBatchJob();
    if ((state.batchJob?.failures || []).length > 0) {
      const detail = state.batchJob.failures.slice(0, 3).map((item) => `${item.id}:${item.reason}`).join("；");
      AdminComponents.renderMessage(
        message,
        `批量标签完成：成功 ${success} 项，失败 ${(state.batchJob.failures || []).length} 项（${detail}${(state.batchJob.failures || []).length > 3 ? " …" : ""}）`,
        "error"
      );
    } else {
      AdminComponents.renderMessage(message, `批量标签成功：共 ${success} 项（模式：${mode === "append" ? "追加" : "替换"}）`);
    }

    closeBatchTagsModal();
    await render();
  }

  function openBatchRollbackModal() {
    const ids = state.selectedAssetIDs.slice();
    if (ids.length === 0) {
      throw new Error("请先选择至少 1 个未删除资产");
    }
    $("batch-rollback-selected-hint").textContent = `当前已选 ${ids.length} 项`;
    $("batch-rollback-mode").value = "fixed";
    $("batch-rollback-version").value = "";
    $("batch-rollback-version").disabled = false;
    $("batch-rollback-modal").hidden = false;
  }

  function closeBatchRollbackModal() {
    $("batch-rollback-modal").hidden = true;
  }

  function resolvePreviousVersionNumber(versions) {
    const versionRows = (versions || [])
      .map((item) => Number(item.version || 0))
      .filter((item) => Number.isInteger(item) && item > 0)
      .sort((a, b) => b - a);
    if (versionRows.length < 2) {
      throw new Error("无上一版本可回滚");
    }
    return versionRows[1];
  }

  async function batchRollbackSelected() {
    const message = $("message");
    const ids = state.selectedAssetIDs.slice();
    if (ids.length === 0) return;

    const mode = $("batch-rollback-mode").value === "previous" ? "previous" : "fixed";
    const inputVersion = Number($("batch-rollback-version").value || 0);
    if (mode === "fixed" && (!Number.isInteger(inputVersion) || inputVersion <= 0)) {
      throw new Error("批量回滚失败：请输入大于 0 的整数版本号");
    }

    const targetText = mode === "fixed" ? `v${inputVersion}` : "自动上一版本";
    const ok = window.confirm(`确认将已选 ${ids.length} 个资产批量回滚到${targetText}吗？`);
    if (!ok) return;

    const btn = $("batch-rollback-save");
    if (btn) btn.disabled = true;

    createBatchJob("rollback", ids.length);
    let success = 0;

    for (let i = 0; i < ids.length; i += 1) {
      const id = ids[i];
      const row = state.assetRowsByID[id];
      if (!row || row.is_deleted) {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
        continue;
      }
      try {
        let version = inputVersion;
        const retryPayload = {
          tenant_id: row.tenant_id || state.query.tenant_id,
          asset_id: Number(row.id),
          version: inputVersion,
          mode,
        };
        if (mode === "previous") {
          const versions = await apiGet("/admin/assets/versions", {
            tenant_id: row.tenant_id || state.query.tenant_id,
            asset_id: Number(row.id),
            limit: 100,
            offset: 0,
          });
          version = resolvePreviousVersionNumber(versions.data || []);
          retryPayload.version = version;
        }

        await apiPost("/admin/assets/rollback", {
          tenant_id: row.tenant_id || state.query.tenant_id,
          asset_id: Number(row.id),
          version,
        });
        success += 1;
      } catch (err) {
        appendBatchJobFailure({
          id,
          action: "rollback",
          reason: err.message || "回滚失败",
          retry_payload: {
            tenant_id: row.tenant_id || state.query.tenant_id,
            asset_id: Number(row.id),
            version: inputVersion,
            mode,
          },
        });
      } finally {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
      }
    }

    finishBatchJob();
    if ((state.batchJob?.failures || []).length > 0) {
      const detail = state.batchJob.failures.slice(0, 3).map((item) => `${item.id}:${item.reason}`).join("；");
      AdminComponents.renderMessage(
        message,
        `批量回滚完成：成功 ${success} 项，失败 ${(state.batchJob.failures || []).length} 项（${detail}${(state.batchJob.failures || []).length > 3 ? " …" : ""}）`,
        "error"
      );
    } else {
      AdminComponents.renderMessage(message, `批量回滚成功：共 ${success} 项，目标${targetText}`);
    }

    closeBatchRollbackModal();
    await render();
    if (btn) btn.disabled = false;
  }

  async function batchSoftDeleteSelected() {
    const message = $("message");
    const ids = state.selectedAssetIDs.slice();
    if (ids.length === 0) return;

    const ok = window.confirm(`确认批量软删除当前页已选的 ${ids.length} 个资产吗？`);
    if (!ok) return;

    const btn = $("batch-delete-btn");
    if (btn) btn.disabled = true;

    createBatchJob("delete", ids.length);
    let success = 0;

    for (let i = 0; i < ids.length; i += 1) {
      const id = ids[i];
      const row = state.assetRowsByID[id];
      if (!row || row.is_deleted) {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
        continue;
      }
      try {
        await apiDelete("/admin/assets", {
          id,
          tenant_id: row.tenant_id || state.query.tenant_id,
        });
        success += 1;
      } catch (err) {
        appendBatchJobFailure({
          id,
          action: "delete",
          reason: err.message || "删除失败",
          retry_payload: {
            id,
            tenant_id: row.tenant_id || state.query.tenant_id,
          },
        });
      } finally {
        updateBatchJobProgress({ completed: i + 1, current_id: id });
      }
    }

    state.selectedAssetIDs = [];
    finishBatchJob();

    if ((state.batchJob?.failures || []).length > 0) {
      const detail = state.batchJob.failures.slice(0, 3).map((item) => `${item.id}:${item.reason}`).join("；");
      AdminComponents.renderMessage(
        message,
        `批量软删除完成：成功 ${success} 项，失败 ${(state.batchJob.failures || []).length} 项（${detail}${(state.batchJob.failures || []).length > 3 ? " …" : ""}）`,
        "error"
      );
    } else {
      AdminComponents.renderMessage(message, `批量软删除成功：共 ${success} 项`);
    }

    await render();
    if (btn) btn.disabled = state.selectedAssetIDs.length === 0;
  }

  async function renderStats() {
    const view = $("view");
    const stats = await apiGet("/admin/assets/stats", {
      tenant_id: state.query.tenant_id,
      include_deleted: state.query.include_deleted,
      limit: state.query.limit,
    });

    const byTask = AdminComponents.renderSimpleTable(
      [
        { key: "key", title: "task" },
        { key: "asset_count", title: "assets" },
        { key: "reuse_count", title: "reuse" },
      ],
      stats.by_task || []
    );
    const byModel = AdminComponents.renderSimpleTable(
      [
        { key: "key", title: "model" },
        { key: "asset_count", title: "assets" },
        { key: "reuse_count", title: "reuse" },
      ],
      stats.by_model || []
    );
    const byTag = AdminComponents.renderSimpleTable(
      [
        { key: "key", title: "tag" },
        { key: "asset_count", title: "assets" },
        { key: "reuse_count", title: "reuse" },
      ],
      stats.by_tag || []
    );

    view.innerHTML = `${AdminComponents.renderOverviewCards(stats.overview)}
      <h3>by_task</h3>${byTask}
      <h3>by_model</h3>${byModel}
      <h3>by_tag</h3>${byTag}`;
  }

  async function renderAudits() {
    const view = $("view");
    const data = await apiGet("/admin/assets/reuse-audits", {
      tenant_id: state.query.tenant_id,
      limit: state.query.limit,
      offset: state.query.offset,
    });

    const rows = data.data || [];
    const overview = data.stats?.overview || {};

    const table = AdminComponents.renderSimpleTable(
      [
        { key: "id", title: "ID" },
        { key: "asset_id", title: "Asset" },
        { key: "request_id", title: "Request" },
        { key: "route_model", title: "Model" },
        { key: "route_task", title: "Task" },
        { key: "hit_source", title: "Source" },
        { key: "created_at", title: "Created" },
      ],
      rows,
      "当前租户暂无复用审计记录"
    );

    const summary = `
      <div class="card audit-summary">
        <strong>审计摘要</strong>
        <div>资产总数：${AdminComponents.escapeHtml(overview.asset_count || 0)}</div>
        <div>复用次数：${AdminComponents.escapeHtml(overview.reuse_count || 0)}</div>
        <div>命中总数：${AdminComponents.escapeHtml(overview.total_hit_count || 0)}</div>
      </div>
    `;

    view.innerHTML = `${summary}${table}`;
    const pager = AdminComponents.renderPager({
      limit: state.query.limit,
      offset: state.query.offset,
      count: (data.data || []).length,
      onPrev: () => {
        state.query.offset = Math.max(0, state.query.offset - state.query.limit);
        writeQueryToURL();
        render();
      },
      onNext: () => {
        state.query.offset = state.query.offset + state.query.limit;
        writeQueryToURL();
        render();
      },
    });
    view.appendChild(pager);
  }

  function parseTagsInput(input) {
    return String(input || "")
      .split(",")
      .map((item) => item.trim())
      .filter((item) => item.length > 0);
  }

  function openCreateModal() {
    $("create_tenant_id").value = (state.query.tenant_id || "").trim();
    $("create_source_model").value = "";
    $("create_task_type").value = "";
    $("create_title").value = "";
    $("create_summary").value = "";
    $("create_tags").value = "";
    $("create_user_id").value = "";
    $("create_session_id").value = "";
    $("create_source_request_id").value = "";
    $("create-modal").hidden = false;
  }

  function closeCreateModal() {
    $("create-modal").hidden = true;
  }

  async function saveCreateAsset() {
    const tenantID = $("create_tenant_id").value.trim();
    const sourceModel = $("create_source_model").value.trim();
    const taskType = $("create_task_type").value.trim();
    const title = $("create_title").value.trim();
    const summary = $("create_summary").value.trim();

    if (!tenantID || !sourceModel || !taskType || !title || !summary) {
      throw new Error("创建失败：tenant_id、source_model、task_type、title、summary 必填");
    }

    const payload = {
      tenant_id: tenantID,
      user_id: $("create_user_id").value.trim(),
      session_id: $("create_session_id").value.trim(),
      source_model: sourceModel,
      task_type: taskType,
      title,
      summary,
      tags: parseTagsInput($("create_tags").value),
      source_request_id: $("create_source_request_id").value.trim(),
    };

    const created = await apiPost("/admin/assets", payload);
    state.lastCreatedAssetID = Number(created.id || 0);
    appendOperationLog("asset.create", "success", `创建资产 ${state.lastCreatedAssetID || "-"}`, {
      asset_id: state.lastCreatedAssetID || 0,
      tenant_id: tenantID,
      title,
    });
    AdminComponents.renderMessage($("message"), `创建成功：asset=${state.lastCreatedAssetID || "-"}`);
    closeCreateModal();
    await render();
  }

  function openEditModal(row) {
    $("edit_id").value = String(row.id || "");
    $("edit_tenant_id").value = String(row.tenant_id || "");
    $("edit_user_id").value = String(row.user_id || "");
    $("edit_session_id").value = String(row.session_id || "");
    $("edit_source_model").value = String(row.source_model || "");
    $("edit_task_type").value = String(row.task_type || "");
    $("edit_source_request_id").value = String(row.source_request_id || "");
    $("edit_title").value = String(row.title || "");
    $("edit_summary").value = String(row.summary || "");
    $("edit_tags").value = Array.isArray(row.tags) ? row.tags.join(", ") : "";
    $("edit-modal").hidden = false;
  }

  function closeEditModal() {
    $("edit-modal").hidden = true;
  }

  async function saveEditAsset() {
    const id = Number($("edit_id").value || 0);
    const tenantID = $("edit_tenant_id").value.trim();
    const sourceModel = $("edit_source_model").value.trim();
    const title = $("edit_title").value.trim();
    const summary = $("edit_summary").value.trim();

    if (!id || !tenantID || !sourceModel || !title || !summary) {
      throw new Error("编辑保存失败：id、tenant_id、source_model、title、summary 必填");
    }

    const payload = {
      id,
      tenant_id: tenantID,
      user_id: $("edit_user_id").value.trim(),
      session_id: $("edit_session_id").value.trim(),
      source_model: sourceModel,
      task_type: $("edit_task_type").value.trim(),
      title,
      summary,
      tags: parseTagsInput($("edit_tags").value),
      source_request_id: $("edit_source_request_id").value.trim(),
    };

    await apiPut("/admin/assets", payload);
    appendOperationLog("asset.edit", "success", `编辑资产 ${id}`, {
      asset_id: id,
      tenant_id: tenantID,
      title,
    });
    AdminComponents.renderMessage($("message"), `编辑成功：asset=${id}`);
    closeEditModal();

    await render();
    if (state.selectedAssetID === id && $("version-drawer") && !$("version-drawer").hidden) {
      await openVersions();
    }
  }

  async function openVersions() {
    if (!state.selectedAssetID) return;
    const drawer = $("version-drawer");
    drawer.hidden = false;

    const data = await apiGet("/admin/assets/versions", {
      tenant_id: state.query.tenant_id,
      asset_id: state.selectedAssetID,
      limit: state.query.limit,
      offset: 0,
    });

    const header = `
      <div class="card version-header">
        <div><strong>当前资产：</strong>${AdminComponents.escapeHtml(state.selectedAssetID || "")}</div>
        <button id="version-refresh">刷新版本</button>
      </div>
    `;

    const html = (data.data || [])
      .map(
        (v) => `<div class="card version-card">
          <div><strong>v${AdminComponents.escapeHtml(v.version)}</strong> ${AdminComponents.escapeHtml(v.snapshot_created_at)}</div>
          <div>${AdminComponents.escapeHtml(v.title)}</div>
          <div>${AdminComponents.escapeHtml(v.summary)}</div>
          <button data-action="rollback" data-version="${v.version}">回滚到此版本</button>
        </div>`
      )
      .join("") || "<div class=\"card\">暂无版本</div>";

    $("version-list").innerHTML = `${header}${html}`;
    $("version-refresh")?.addEventListener("click", async () => {
      await openVersions();
    });
    $("version-list").querySelectorAll('button[data-action="rollback"]').forEach((btn) => {
      btn.addEventListener("click", async () => {
        const version = Number(btn.dataset.version);
        const ok = window.confirm(`确认将资产 ${state.selectedAssetID} 回滚到版本 v${version} 吗？`);
        if (!ok) return;

        btn.disabled = true;
        try {
          await apiPost("/admin/assets/rollback", {
            tenant_id: state.query.tenant_id,
            asset_id: state.selectedAssetID,
            version,
          });
          AdminComponents.renderMessage($("message"), `回滚成功：asset=${state.selectedAssetID}, version=v${version}`);
          await render();
          await openVersions();
        } finally {
          btn.disabled = false;
        }
      });
    });
  }

  async function render() {
    const message = $("message");
    setLoading(true);
    try {
      AdminComponents.renderMessage(message, "");
      if (state.view === "assets") await renderAssets();
      else if (state.view === "stats") await renderStats();
      else if (state.view === "audits") await renderAudits();
      else await renderAssets();
    } catch (err) {
      AdminComponents.renderMessage(message, err.message || "请求失败，可稍后重试", "error");
    } finally {
      setLoading(false);
    }
  }

  function bootstrap() {
    readQueryFromURL();
    applyQueryToForm();
    renderFilterPresetOptions();
    renderOperationLogs();
    bindNav();
    bindFilters();
    $("drawer-close").addEventListener("click", () => {
      $("version-drawer").hidden = true;
    });

    renderAdminKeyStatus();
    window.setInterval(renderAdminKeyStatus, 30000);
    $("admin-key-set-btn")?.addEventListener("click", () => {
      const key = promptAdminKeyUpdate();
      if (!key) return;
      appendOperationLog("admin_key.set", "success", "设置或更新 Admin Key", {
        masked_key: maskAdminKey(key),
      });
      AdminComponents.renderMessage($("message"), "Admin Key 已更新");
    });
    $("admin-key-clear-btn")?.addEventListener("click", () => {
      const ok = window.confirm("确认清除当前 Admin Key 会话吗？");
      if (!ok) return;
      appendOperationLog("admin_key.clear", "success", "清除 Admin Key", {});
      clearAdminKeyStorage();
      AdminComponents.renderMessage($("message"), "Admin Key 已清除");
    });
    $("export-oplog-json-btn")?.addEventListener("click", () => {
      try {
        exportOperationLogs("json");
        AdminComponents.renderMessage($("message"), "操作日志已导出为 JSON");
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "导出操作日志 JSON 失败", "error");
      }
    });
    $("operation-log-action-filter")?.addEventListener("change", renderOperationLogs);
    $("operation-log-search")?.addEventListener("input", renderOperationLogs);
    $("export-oplog-csv-btn")?.addEventListener("click", () => {
      try {
        exportOperationLogs("csv");
        AdminComponents.renderMessage($("message"), "操作日志已导出为 CSV");
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "导出操作日志 CSV 失败", "error");
      }
    });

    $("preset_save_btn")?.addEventListener("click", () => {
      try {
        saveCurrentFilterPreset();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "保存预设失败", "error");
      }
    });
    $("preset_load_btn")?.addEventListener("click", () => {
      try {
        loadSelectedFilterPreset();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "加载预设失败", "error");
      }
    });
    $("preset_delete_btn")?.addEventListener("click", () => {
      try {
        deleteSelectedFilterPreset();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "删除预设失败", "error");
      }
    });
    $("create-asset-open")?.addEventListener("click", openCreateModal);
    $("export-json-btn")?.addEventListener("click", async () => {
      const btn = $("export-json-btn");
      btn.disabled = true;
      try {
        await exportCurrentAssets("json");
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "导出 JSON 失败", "error");
      } finally {
        btn.disabled = false;
      }
    });
    $("export-csv-btn")?.addEventListener("click", async () => {
      const btn = $("export-csv-btn");
      btn.disabled = true;
      try {
        await exportCurrentAssets("csv");
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "导出 CSV 失败", "error");
      } finally {
        btn.disabled = false;
      }
    });
    $("batch-delete-btn")?.addEventListener("click", async () => {
      try {
        await batchSoftDeleteSelected();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "批量软删除失败", "error");
      }
    });
    $("batch-tags-open-btn")?.addEventListener("click", () => {
      try {
        openBatchTagsModal();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "无法打开批量标签", "error");
      }
    });
    $("batch-rollback-open-btn")?.addEventListener("click", () => {
      try {
        openBatchRollbackModal();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "无法打开批量回滚", "error");
      }
    });
    $("batch-tags-close")?.addEventListener("click", closeBatchTagsModal);
    $("batch-tags-cancel")?.addEventListener("click", closeBatchTagsModal);
    $("batch-tags-save")?.addEventListener("click", async () => {
      const btn = $("batch-tags-save");
      btn.disabled = true;
      try {
        await batchApplyTagsSelected();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "批量标签失败", "error");
      } finally {
        btn.disabled = false;
      }
    });
    $("batch-rollback-mode")?.addEventListener("change", () => {
      const fixedMode = $("batch-rollback-mode").value !== "previous";
      $("batch-rollback-version").disabled = !fixedMode;
      if (!fixedMode) $("batch-rollback-version").value = "";
    });
    $("batch-rollback-close")?.addEventListener("click", closeBatchRollbackModal);
    $("batch-rollback-cancel")?.addEventListener("click", closeBatchRollbackModal);
    $("batch-rollback-save")?.addEventListener("click", async () => {
      const btn = $("batch-rollback-save");
      btn.disabled = true;
      try {
        await batchRollbackSelected();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "批量回滚失败", "error");
      } finally {
        btn.disabled = false;
      }
    });
    $("batch-job-retry-btn")?.addEventListener("click", async () => {
      const btn = $("batch-job-retry-btn");
      btn.disabled = true;
      try {
        await retryFailedBatchJobItems();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "重试失败项失败", "error");
      } finally {
        btn.disabled = false;
      }
    });

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
          btn.textContent = "重试";
          btn.disabled = false;
        }
      }
    });

    renderBatchJobPanel();
    $("create-close")?.addEventListener("click", closeCreateModal);
    $("create-cancel")?.addEventListener("click", closeCreateModal);
    $("create-save")?.addEventListener("click", async () => {
      const btn = $("create-save");
      btn.disabled = true;
      try {
        await saveCreateAsset();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "创建失败", "error");
      } finally {
        btn.disabled = false;
      }
    });

    $("edit-close").addEventListener("click", closeEditModal);
    $("edit-cancel").addEventListener("click", closeEditModal);
    $("edit-save").addEventListener("click", async () => {
      const btn = $("edit-save");
      btn.disabled = true;
      try {
        await saveEditAsset();
      } catch (err) {
        AdminComponents.renderMessage($("message"), err.message || "编辑保存失败", "error");
      } finally {
        btn.disabled = false;
      }
    });

    setView(state.view || "assets");
  }

  bootstrap();
})();
