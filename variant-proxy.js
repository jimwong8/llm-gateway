const express = require("express");
const app = express();
const PORT = 8787;
const UPSTREAM_BASE_URL = process.env.UPSTREAM_BASE_URL;
if (!UPSTREAM_BASE_URL) {
  console.error("缺少环境变量 UPSTREAM_BASE_URL");
  process.exit(1);
}
app.use(express.json({ limit: "20mb", type: ["application/json", "application/*+json"] }));
app.use(express.text({ type: "*/*", limit: "20mb" }));
function redactHeaders(headers) {
  const out = {};
  for (const [k, v] of Object.entries(headers || {})) {
    const key = k.toLowerCase();
    if (
      key === "authorization" ||
      key === "cookie" ||
      key === "set-cookie" ||
      key === "x-api-key" ||
      key === "api-key"
    ) {
      out[k] = "***REDACTED***";
    } else {
      out[k] = v;
    }
  }
  return out;
}
function parseBody(body) {
  if (typeof body !== "string") return body;
  try {
    return JSON.parse(body);
  } catch {
    return body;
  }
}
function summarizeBody(body) {
  const b = parseBody(body);
  if (!b || typeof b !== "object") return b;
  return {
    model: b.model,
    temperature: b.temperature,
    top_p: b.top_p,
    reasoning: b.reasoning,
    reasoning_effort: b.reasoning_effort,
    thinking: b.thinking,
    extra_body: b.extra_body,
    providerOptions: b.providerOptions,
    messages_count: Array.isArray(b.messages) ? b.messages.length : undefined
  };
}
app.use(async (req, res) => {
  const upstreamUrl = UPSTREAM_BASE_URL.replace(/\/$/, "") + req.originalUrl;
  const rawBody =
    typeof req.body === "string"
      ? req.body
      : req.body && typeof req.body === "object" && Object.keys(req.body).length
      ? JSON.stringify(req.body)
      : undefined;
  console.log("\n===== REQUEST =====");
  console.log(
    JSON.stringify(
      {
        time: new Date().toISOString(),
        method: req.method,
        path: req.originalUrl,
        upstreamUrl,
        headers: redactHeaders(req.headers),
        body_summary: summarizeBody(rawBody)
      },
      null,
      2
    )
  );
  try {
    const headers = { ...req.headers };
    delete headers.host;
    delete headers["content-length"];
    const upstreamResp = await fetch(upstreamUrl, {
      method: req.method,
      headers,
      body: ["GET", "HEAD"].includes(req.method) ? undefined : rawBody
    });
    const text = await upstreamResp.text();
    console.log("\n===== RESPONSE =====");
    console.log(
      JSON.stringify(
        {
          time: new Date().toISOString(),
          status: upstreamResp.status,
          statusText: upstreamResp.statusText,
          headers: redactHeaders(Object.fromEntries(upstreamResp.headers.entries())),
          body_preview: text.slice(0, 1000)
        },
        null,
        2
      )
    );
    res.status(upstreamResp.status);
    upstreamResp.headers.forEach((value, key) => {
      if (key.toLowerCase() !== "content-length") {
        res.setHeader(key, value);
      }
    });
    res.send(text);
  } catch (err) {
    console.error("代理转发失败:", err);
    res.status(502).json({
      error: "proxy_failed",
      message: err instanceof Error ? err.message : String(err)
    });
  }
});
app.listen(PORT, "127.0.0.1", () => {
  console.log(`代理已启动: http://127.0.0.1:${PORT}`);
  console.log(`上游地址: ${UPSTREAM_BASE_URL}`);
});
