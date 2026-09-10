/**
 * 把后端返回的原始英文错误翻译成中文，供前端 Toast / ErrorBoundary 使用。
 *
 * 覆盖 LLM Gateway 常见错误签名：
 *  - 502/503/504：nginx / llm-gateway 后端不可达
 *  - channel rate gate：per-channel 令牌桶限流
 *  - provider circuit open：熔断器打开
 *  - decode provider response: invalid character：上游返回非 JSON
 *  - model is not found：请求的模型在当前 channel 不存在
 *  - 429 / quota：配额耗尽
 *  - 401 / unauthorized：token 过期或无效
 */

export type ErrorTone = 'error' | 'warning' | 'info'

export interface TranslatedError {
  title: string
  body: string
  tone: ErrorTone
}

const ERROR_RULES: Array<{ pattern: RegExp; translated: TranslatedError }> = [
  {
    pattern: /\b502\s+Bad Gateway\b/i,
    translated: {
      title: '网关后端无响应',
      body: 'llm-gateway 服务未启动或正在熔断，请检查后端服务状态。',
      tone: 'error',
    },
  },
  {
    pattern: /\b503\s+Service Unavailable\b/i,
    translated: {
      title: '服务暂时不可用',
      body: '服务熔断冷却中或正在重启，稍后自动恢复。',
      tone: 'warning',
    },
  },
  {
    pattern: /\b504\s+Gateway Timeout\b/i,
    translated: {
      title: '上游超时',
      body: '上游 Provider 响应超时，可能是 Provider 侧故障。',
      tone: 'error',
    },
  },
  {
    pattern: /channel rate gate|rate limit/i,
    translated: {
      title: '渠道限流触发',
      body: '该渠道 RPM/TPM 已达上限，请稍后重试或切换渠道。',
      tone: 'warning',
    },
  },
  {
    pattern: /provider circuit open|circuit.*open/i,
    translated: {
      title: '上游熔断中',
      body: '该 Provider 熔断器处于冷却期，系统正在等待自动恢复。',
      tone: 'warning',
    },
  },
  {
    pattern: /decode provider response.*invalid character/i,
    translated: {
      title: '上游响应解析失败',
      body: 'Provider 返回了非 JSON 响应，可能是网络劫持或 Provider 故障。',
      tone: 'error',
    },
  },
  {
    pattern: /model is not found|model not found|unknown model/i,
    translated: {
      title: '模型不可用',
      body: '请求的模型在当前 Provider 不存在，请切换模型或渠道。',
      tone: 'warning',
    },
  },
  {
    pattern: /\b429\b|quota.*exceeded|rate.*exceeded/i,
    translated: {
      title: '配额耗尽',
      body: '当前 key/渠道已达日/月配额上限，等待重置或切换渠道。',
      tone: 'warning',
    },
  },
  {
    pattern: /\b401\b|unauthorized|invalid.*token|token.*expired/i,
    translated: {
      title: '认证失败',
      body: 'Token 无效或已过期，请重新登录。',
      tone: 'error',
    },
  },
  {
    pattern: /\b403\b|forbidden|access.*denied/i,
    translated: {
      title: '权限不足',
      body: '当前账号无该操作权限，请联系管理员。',
      tone: 'error',
    },
  },
  {
    pattern: /network.*unreachable|ECONNREFUSED|ENOTFOUND|fetch failed/i,
    translated: {
      title: '网络不可达',
      body: '无法连接到后端服务，请检查网络连接或代理配置。',
      tone: 'error',
    },
  },
]

export function translateError(raw: unknown): TranslatedError {
  if (!raw) {
    return { title: '请求失败', body: '未知错误', tone: 'error' }
  }

  // ApiError 已封装好的对象
  if (raw instanceof Error) {
    return matchRule(raw.message)
  }
  if (typeof raw === 'object') {
    const anyRaw = raw as { message?: string; error?: { message?: string }; status?: number }
    const msg = anyRaw.error?.message ?? anyRaw.message ?? ''
    const withStatus = anyRaw.status ? `${anyRaw.status}: ${msg}` : msg
    return matchRule(withStatus)
  }

  return matchRule(String(raw))
}

function matchRule(raw: string): TranslatedError {
  for (const rule of ERROR_RULES) {
    if (rule.pattern.test(raw)) return rule.translated
  }
  return { title: '请求失败', body: raw, tone: 'error' }
}
