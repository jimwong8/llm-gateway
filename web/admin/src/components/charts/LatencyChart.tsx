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
import { ChartEmptyState } from './ChartEmptyState'
import { useTranslation } from 'react-i18next'

interface LatencyData {
  date: string
  p50: number
  p95: number
  p99: number
}

export const LatencyChart = React.memo(function LatencyChart({ data }: { data?: LatencyData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyLatency')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleLatency')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis tick={{ fontSize: 12 }} stroke="#64748b" tickFormatter={(v) => `${v}ms`} />
          <Tooltip formatter={(v: unknown) => `${Number(v).toFixed(1)} ms`} />
          <Legend />
          <Line
            type="monotone"
            dataKey="p50"
            name={t('charts.chartP50')}
            stroke={COLORS.green}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="p95"
            name={t('charts.chartP95')}
            stroke={COLORS.amber}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="p99"
            name={t('charts.chartP99')}
            stroke={COLORS.red}
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
})
