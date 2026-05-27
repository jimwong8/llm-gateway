import React from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { COLORS } from './index'

interface LatencyData {
  date: string
  p50: number
  p95: number
  p99: number
}

export const LatencyChart = React.memo(function LatencyChart({ data }: { data?: LatencyData[] }) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无延迟数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">延迟趋势 (P50/P95/P99)</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis tick={{ fontSize: 12 }} stroke="#94a3b8" tickFormatter={(v) => `${v}ms`} />
          <Tooltip formatter={(v: unknown) => `${Number(v).toFixed(1)} ms`} />
          <Legend />
          <Line
            type="monotone"
            dataKey="p50"
            name="P50"
            stroke={COLORS.green}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="p95"
            name="P95"
            stroke={COLORS.amber}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="p99"
            name="P99"
            stroke={COLORS.red}
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
})

function ChartEmptyState({ label }: { label: string }) {
  return (
    <div className="chart-empty-state">
      <p>{label}</p>
    </div>
  )
}
