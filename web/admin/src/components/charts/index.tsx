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
  if (!data || data.length === 0) return <ChartEmptyState label="暂无 Token 使用数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">Token 使用趋势</h3>
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
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <Tooltip />
          <Legend />
          <Area
            type="monotone"
            dataKey="prompt"
            stroke={COLORS.blue}
            fill="url(#promptGrad)"
            name="Prompt Tokens"
            stackId="1"
          />
          <Area
            type="monotone"
            dataKey="completion"
            stroke={COLORS.indigo}
            fill="url(#completionGrad)"
            name="Completion Tokens"
            stackId="1"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

export function ModelDistributionChart({
  data,
}: {
  data?: ModelDistributionData[]
}) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无模型分布数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">模型分布</h3>
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

export function CacheHitRateChart({
  data,
}: {
  data?: CacheHitData[]
}) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无缓存数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">缓存命中率</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis
            tick={{ fontSize: 12 }}
            stroke="#94a3b8"
            domain={[0, 100]}
            tickFormatter={(v) => `${v}%`}
          />
          <Tooltip formatter={(v: unknown) => `${Number(v).toFixed(1)}%`} />
          <Legend />
          <Bar dataKey="hitRate" name="命中率" fill={COLORS.green} radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export function ChannelStatusChart({
  data,
}: {
  data?: ChannelStatusData[]
}) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无渠道数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">渠道状态概览</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data} layout="vertical">
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis type="number" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis dataKey="name" type="category" tick={{ fontSize: 12 }} stroke="#94a3b8" width={80} />
          <Tooltip />
          <Legend />
          <Bar dataKey="healthy" name="健康" stackId="a" fill={COLORS.green} />
          <Bar dataKey="degraded" name="降级" stackId="a" fill={COLORS.amber} />
          <Bar dataKey="down" name="宕机" stackId="a" fill={COLORS.red} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export function DailyRequestsChart({
  data,
}: {
  data?: DailyRequestData[]
}) {
  if (!data || data.length === 0) return <ChartEmptyState label="暂无请求数据" />

  return (
    <div className="chart-container">
      <h3 className="chart-title">每日请求量</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="date" tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <YAxis tick={{ fontSize: 12 }} stroke="#94a3b8" />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="requests"
            name="请求数"
            stroke={COLORS.blue}
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="errors"
            name="错误数"
            stroke={COLORS.red}
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
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
