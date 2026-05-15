export function formatDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function formatPercent(value: number | undefined): string {
  return `${((value ?? 0) * 100).toFixed(1)}%`
}

export function formatLatency(value: number | undefined): string {
  if (value === undefined) return '—'
  return `${value.toFixed(1)} ms`
}

export function formatCost(value: number | undefined): string {
  if (value === undefined) return '—'
  return `$${value.toFixed(4)}`
}

export function buildQuery(path: string, params: Record<string, string>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v.trim()) sp.set(k, v.trim())
  }
  const qs = sp.toString()
  return qs ? `${path}?${qs}` : path
}

export function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '…'
}
