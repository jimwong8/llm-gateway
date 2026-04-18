window.AdminComponents = {
  escapeHtml(input) {
    return String(input ?? "")
      .replaceAll("&", "&")
      .replaceAll("<", "<")
      .replaceAll(">", ">")
      .replaceAll('"', """)
      .replaceAll("'", "'");
  },

  renderMessage(el, text, level = "warn") {
    if (!el) return;
    if (!text) {
      el.innerHTML = "";
      return;
    }
    const cls = level === "error" ? "alert alert-error" : "alert";
    el.innerHTML = `<div class="${cls}">${this.escapeHtml(text)}</div>`;
  },

  renderOverviewCards(overview = {}) {
    const items = [
      ["asset_count", "资产总数"],
      ["active_count", "活跃资产"],
      ["deleted_count", "软删除"],
      ["version_count", "版本总数"],
      ["reuse_count", "复用次数"],
      ["total_hit_count", "命中总数"],
    ];
    return `<div class="card-grid">${items
      .map(([k, label]) => {
        const value = overview[k] ?? 0;
        return `<div class="card stat-card"><div class="card-label">${this.escapeHtml(label)}</div><strong class="card-value">${this.escapeHtml(value)}</strong></div>`;
      })
      .join("")}</div>`;
  },

  renderSimpleTable(columns, rows, emptyText = "暂无数据") {
    const thead = `<tr>${columns.map((c) => `<th>${this.escapeHtml(c.title)}</th>`).join("")}</tr>`;
    const tbody = (rows || [])
      .map((row) => {
        return `<tr>${columns
          .map((c) => {
            const raw = row?.[c.key];
            const val = raw === undefined || raw === null ? "" : raw;
            return `<td>${this.escapeHtml(val)}</td>`;
          })
          .join("")}</tr>`;
      })
      .join("");
    return `<table class="table"><thead>${thead}</thead><tbody>${tbody || `<tr><td colspan="${columns.length}">${this.escapeHtml(emptyText)}</td></tr>`}</tbody></table>`;
  },

  renderPager({ limit, offset, count, onPrev, onNext }) {
    const hasPrev = offset > 0;
    const hasNext = count >= limit;
    const from = offset + 1;
    const to = offset + count;

    const container = document.createElement("div");
    container.className = "pager";
    container.innerHTML = `
      <span class="pager-info">显示 ${this.escapeHtml(from)} - ${this.escapeHtml(to)}</span>
      <button class="pager-btn" data-action="prev" ${hasPrev ? "" : "disabled"}>上一页</button>
      <button class="pager-btn" data-action="next" ${hasNext ? "" : "disabled"}>下一页</button>
    `;

    container.querySelector('[data-action="prev"]')?.addEventListener("click", onPrev);
    container.querySelector('[data-action="next"]')?.addEventListener("click", onNext);
    return container;
  },
};
