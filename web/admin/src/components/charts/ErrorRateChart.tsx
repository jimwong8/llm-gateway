import React from 'react'
import {
  AreaChart,
  Area,
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

interface ErrorRateData {
  date: string
  errorRate: number
  totalRequests: number
  errorRequests: number
}

export const ErrorRateChart = React.memo(function ErrorRateChart({ data }: { data?: ErrorRateData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyErrorRate')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleErrorRate')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id="errorRateGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor={COLORS.red} stopOpacity={0.3} />
              <stop offset="95%" stopColor={COLORS.red} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis
            tick={{ fontSize: 12 }}
            stroke="#64748b"
            tickFormatter={(v) => `${v}%`}
          />
          <Tooltip formatter={(v: unknown, name: unknown) => {
            if (name === 'errorRate') return `${Number(v).toFixed(2)}%`
            return String(v)
          }} />
          <Legend />
          <Area
            type="monotone"
            dataKey="errorRate"
            name={t('charts.chartErrorRate')}
            stroke={COLORS.red}
            fill="url(#errorRateGrad)"
            strokeWidth={2}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
})
