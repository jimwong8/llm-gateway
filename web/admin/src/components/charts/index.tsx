import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  LineChart,
  Line,
} from 'recharts'

import { ChartEmptyState } from './ChartEmptyState'
import { useTranslation } from 'react-i18next'

const COLORS = {
  blue: '#3b82f6',
  indigo: '#6366f1',
  violet: '#8b5cf6',
  green: '#22c55e',
  red: '#ef4444',
  amber: '#f59e0b',
  cyan: '#06b6d4',
  pink: '#ec4899',
  slate: '#64748b',
}

const CHART_COLORS = [
  COLORS.blue,
  COLORS.indigo,
  COLORS.violet,
  COLORS.green,
  COLORS.amber,
  COLORS.cyan,
  COLORS.pink,
  COLORS.red,
]

interface TokenUsageData {
  date: string
  prompt: number
  completion: number
  total: number
}

interface ModelDistributionData {
  name: string
  value: number
  color?: string
}

interface CacheHitData {
  date: string
  hitRate: number
  requests: number
}

interface ChannelStatusData {
  name: string
  healthy: number
  degraded: number
  down: number
}

interface DailyRequestData {
  date: string
  requests: number
  tokens: number
  errors: number
}

export function TokenUsageChart({ data }: { data?: TokenUsageData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyToken')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleTokenUsage')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id="promptGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor={COLORS.blue} stopOpacity={0.3} />
              <stop offset="95%" stopColor={COLORS.blue} stopOpacity={0} />
            </linearGradient>
            <linearGradient id="completionGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor={COLORS.indigo} stopOpacity={0.3} />
              <stop offset="95%" stopColor={COLORS.indigo} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis tick={{ fontSize: 12 }} stroke="#64748b" />
          <Tooltip />
          <Legend />
          <Area
            type="monotone"
            dataKey="prompt"
            stroke={COLORS.blue}
            fill="url(#promptGrad)"
            name={t('charts.chartPrompt')}
            stackId="1"
          />
          <Area
            type="monotone"
            dataKey="completion"
            stroke={COLORS.indigo}
            fill="url(#completionGrad)"
            name={t('charts.chartCompletion')}
            stackId="1"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

export function ModelDistributionChart({ data }: { data?: ModelDistributionData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyModelDist')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleModelDist')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={60}
            outerRadius={100}
            dataKey="value"
            nameKey="name"
            label={({ name, percent }: { name?: string; percent?: number }) => `${name ?? ''} ${((percent ?? 0) * 100).toFixed(0)}%`}
          >
            {data.map((_, i) => (
              <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
            ))}
          </Pie>
          <Tooltip />
          <Legend />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

export function CacheHitRateChart({ data }: { data?: CacheHitData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyCacheHit')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleCacheHit')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis
            tick={{ fontSize: 12 }}
            stroke="#64748b"
            domain={[0, 100]}
            tickFormatter={(v) => `${v}%`}
          />
          <Tooltip formatter={(v: unknown) => `${Number(v).toFixed(1)}%`} />
          <Legend />
          <Bar dataKey="hitRate" name={t('charts.chartHitRate')} fill={COLORS.green} radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export function ChannelStatusChart({ data }: { data?: ChannelStatusData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyChannelStatus')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleChannelStatus')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data} layout="vertical">
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis type="number" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis dataKey="name" type="category" tick={{ fontSize: 12 }} stroke="#64748b" width={80} />
          <Tooltip />
          <Legend />
          <Bar dataKey="healthy" name={t('charts.chartHealthy')} stackId="a" fill={COLORS.green} />
          <Bar dataKey="degraded" name={t('charts.chartDegraded')} stackId="a" fill={COLORS.amber} />
          <Bar dataKey="down" name={t('charts.chartDown')} stackId="a" fill={COLORS.red} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export function DailyRequestsChart({ data }: { data?: DailyRequestData[] }) {
  const { t } = useTranslation()
  if (!data || data.length === 0) return <ChartEmptyState label={t('charts.emptyDailyRequests')} />

  return (
    <div className="chart-container">
      <h3 className="chart-title">{t('charts.titleDailyRequests')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#64748b" />
          <YAxis tick={{ fontSize: 12 }} stroke="#64748b" />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="requests"
            name={t('charts.chartRequests')}
            stroke={COLORS.blue}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="errors"
            name={t('charts.chartErrors')}
            stroke={COLORS.red}
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

export { LatencyChart } from './LatencyChart'
export { ErrorRateChart } from './ErrorRateChart'
export { COLORS, CHART_COLORS }
export type {
  TokenUsageData,
  ModelDistributionData,
  CacheHitData,
  ChannelStatusData,
  DailyRequestData,
}
