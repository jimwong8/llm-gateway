/**
 * k6 HTTP 压测脚本 — llm-gateway
 *
 * 场景流程：
 *   1. 登录（POST /auth/login）获取 token
 *   2. 查询 preset 列表（GET /api/memory/presets）
 *   3. 创建 preset（POST /api/memory/presets）
 *   4. 查询 preset 详情（GET /api/memory/presets/:id）
 *
 * 环境变量（均可通过 -e 或 .env 覆盖）：
 *   BASE_URL    默认 http://localhost:8080
 *   VUS         默认 10
 *   DURATION    默认 30s
 *   ADMIN_TOKEN 可选，跳过登录步骤
 *
 * 阈值：
 *   - P95 < 500ms
 *   - 错误率 < 1%
 *
 * 用法：
 *   k6 run scripts/loadtest/k6-http.js
 *   k6 run -e VUS=50 -e DURATION=60s scripts/loadtest/k6-http.js
 *   k6 run -e BASE_URL=https://staging.example.com scripts/loadtest/k6-http.js
 */

import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend } from "k6/metrics";

// ── 自定义指标 ──────────────────────────────────────────────
const errorRate = new Rate("errors");
const loginTrend = new Trend("duration_login");
const listTrend = new Trend("duration_list_presets");
const createTrend = new Trend("duration_create_preset");
const detailTrend = new Trend("duration_preset_detail");

// ── 配置 ────────────────────────────────────────────────────
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const VUS = parseInt(__ENV.VUS || "10", 10);
const DURATION = __ENV.DURATION || "30s";
const ADMIN_TOKEN = __ENV.ADMIN_TOKEN || "";

export const options = {
  vus: VUS,
  duration: DURATION,
  thresholds: {
    // P95 < 500ms
    http_req_duration: ["p(95)<500"],
    // 错误率 < 1%
    errors: ["rate<0.01"],
    // 各阶段 P95 阈值
    duration_login: ["p(95)<500"],
    duration_list_presets: ["p(95)<500"],
    duration_create_preset: ["p(95)<500"],
    duration_preset_detail: ["p(95)<500"],
  },
};

// ── 辅助函数 ────────────────────────────────────────────────
function buildHeaders(token) {
  const h = { "Content-Type": "application/json" };
  if (token) {
    h["Authorization"] = `Bearer ${token}`;
  }
  return h;
}

// ── 默认执行函数 ────────────────────────────────────────────
export default function () {
  let token = ADMIN_TOKEN;

  // ── 步骤 1：登录获取 token ───────────────────────────────
  group("1. Login", function () {
    if (!token) {
      const loginRes = http.post(
        `${BASE_URL}/auth/login`,
        JSON.stringify({
          username: "admin",
          password: "admin123",
        }),
        { headers: buildHeaders(), tags: { name: "login" } }
      );

      const ok = check(loginRes, {
        "login status is 200": (r) => r.status === 200,
        "login returns token": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.token && body.token.length > 0;
          } catch {
            return false;
          }
        },
      });

      errorRate.add(!ok);
      loginTrend.add(loginRes.timings.duration);

      if (ok) {
        try {
          const body = JSON.parse(loginRes.body);
          token = body.token;
        } catch {
          // 解析失败，后续请求会失败并计入错误率
        }
      }
    }
  });

  if (!token) {
    // 无 token 则跳过后续步骤
    return;
  }

  sleep(0.1);

  // ── 步骤 2：查询 preset 列表 ─────────────────────────────
  let presetId = null;

  group("2. List Presets", function () {
    const listRes = http.get(`${BASE_URL}/api/memory/presets`, {
      headers: buildHeaders(token),
      tags: { name: "list_presets" },
    });

    const ok = check(listRes, {
      "list status is 200": (r) => r.status === 200,
      "list returns array": (r) => {
        try {
          const body = JSON.parse(r.body);
          return Array.isArray(body) || Array.isArray(body.data);
        } catch {
          return false;
        }
      },
    });

    errorRate.add(!ok);
    listTrend.add(listRes.timings.duration);

    // 尝试从列表中取一个 preset ID 用于后续详情查询
    if (ok) {
      try {
        const body = JSON.parse(listRes.body);
        const items = Array.isArray(body) ? body : body.data;
        if (items && items.length > 0 && items[0].id) {
          presetId = items[0].id;
        }
      } catch {
        // 忽略
      }
    }
  });

  sleep(0.1);

  // ── 步骤 3：创建 preset ──────────────────────────────────
  group("3. Create Preset", function () {
    const createRes = http.post(
      `${BASE_URL}/api/memory/presets`,
      JSON.stringify({
        name: `loadtest-preset-${__VU}-${__ITER}`,
        description: "Auto-created by k6 load test",
        config: {
          model: "gpt-4o",
          temperature: 0.7,
          max_tokens: 2048,
        },
      }),
      { headers: buildHeaders(token), tags: { name: "create_preset" } }
    );

    const ok = check(createRes, {
      "create status is 201 or 200": (r) =>
        r.status === 201 || r.status === 200,
      "create returns id": (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.id !== undefined;
        } catch {
          return false;
        }
      },
    });

    errorRate.add(!ok);
    createTrend.add(createRes.timings.duration);

    if (ok) {
      try {
        const body = JSON.parse(createRes.body);
        if (body.id) {
          presetId = body.id;
        }
      } catch {
        // 忽略
      }
    }
  });

  sleep(0.1);

  // ── 步骤 4：查询 preset 详情 ─────────────────────────────
  if (presetId) {
    group("4. Preset Detail", function () {
      const detailRes = http.get(
        `${BASE_URL}/api/memory/presets/${presetId}`,
        { headers: buildHeaders(token), tags: { name: "preset_detail" } }
      );

      const ok = check(detailRes, {
        "detail status is 200": (r) => r.status === 200,
        "detail returns object": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body && typeof body === "object";
          } catch {
            return false;
          }
        },
      });

      errorRate.add(!ok);
      detailTrend.add(detailRes.timings.duration);
    });
  }

  sleep(0.5);
}
