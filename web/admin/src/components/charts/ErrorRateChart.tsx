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

interface ErrorRateData {
  date: string
  errorRate: number
  totalRequests: number
  errorRequests: number
}

export function ErrorRateChart({ data }: { data?: ErrorRateData[] }) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无错误率数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">错误率趋势</h3>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id="errorRateGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor={COLORS.red} stopOpacity={0.3} />
              <stop offset="95%" stopColor={COLORS.red} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis
            tick={{ fontSize: 12 }}
            stroke="#94a3b8"
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
            name="错误率"
            stroke={COLORS.red}
            fill="url(#errorRateGrad)"
            strokeWidth={2}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

function ChartEmptyState({ label }: { label: string }) {
  return (
    <div className="chart-empty-state">
      <p>{label}</p>
    </div>
  )
}
