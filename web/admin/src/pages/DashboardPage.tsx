import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { UserDashboardView } from '../components/dashboard/UserDashboardView'
import { TokenUsageChart, ModelDistributionChart, CacheHitRateChart, ChannelStatusChart } from '../components/charts'
import { Tabs } from '../components/ui'
import { apiRequest } from '../lib/http'
import { getUserToken } from '../lib/api/identity'
import type { BillingSummary } from '../types/observability'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import { formatPercent } from '../lib/format'

type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

type ChartTab = 'tokens' | 'models' | 'cache' | 'channels'

const CHART_TABS = [
  { key: 'tokens', label: 'Token 趋势' },
  { key: 'models', label: '模型分布' },
  { key: 'cache', label: '缓存命中' },
  { key: 'channels', label: '渠道状态' },
]

type TokenUsagePoint = { date: string; prompt: number; completion: number; total: number }
type ModelDistributionPoint = { name: string; value: number }
type CacheHitPoint = { date: string; hitRate: number; requests: number }
type ChannelStatusPoint = { name: string; healthy: number; degraded: number; down: number }

function DashboardAdminView() {
  const [activeTab, setActiveTab] = useState<ChartTab>('tokens')

  const healthQuery = useQuery({
    queryKey: ['dashboard-health'],
    queryFn: () => apiRequest<AdminHealth>('/admin/health'),
    refetchInterval: 30_000,
  })

  const summaryQuery = useQuery({
    queryKey: ['dashboard-observability-summary'],
    queryFn: () => apiRequest<BillingSummary>('/admin/observability/summary'),
    refetchInterval: 30_000,
  })

  const sessionQuery = useQuery({
    queryKey: ['dashboard-session'],
    queryFn: () => apiRequest<SessionAdminDashboard>('/admin/dashboard'),
    retry: false,
  })

  const tokenUsageQuery = useQuery({
    queryKey: ['dashboard-charts', 'token-usage'],
    queryFn: () => apiRequest<{ data: TokenUsagePoint[] }>('/admin/dashboard/charts/token-usage'),
    refetchInterval: 30_000,
  })

  const modelDistributionQuery = useQuery({
    queryKey: ['dashboard-charts', 'model-distribution'],
    queryFn: () => apiRequest<{ data: ModelDistributionPoint[] }>('/admin/dashboard/charts/model-distribution'),
    refetchInterval: 30_000,
  })

  const cacheHitRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'cache-hit-rate'],
    queryFn: () => apiRequest<{ data: CacheHitPoint[] }>('/admin/dashboard/charts/cache-hit-rate'),
    refetchInterval: 30_000,
  })

  const channelStatusQuery = useQuery({
    queryKey: ['dashboard-charts', 'channel-status'],
    queryFn: () => apiRequest<{ data: ChannelStatusPoint[] }>('/admin/dashboard/charts/channel-status'),
    refetchInterval: 30_000,
  })

  const loading = healthQuery.isLoading || summaryQuery.isLoading
  const hasError = healthQuery.error || summaryQuery.error

  const chartLoading = tokenUsageQuery.isLoading || modelDistributionQuery.isLoading || cacheHitRateQuery.isLoading || channelStatusQuery.isLoading
  const chartError = tokenUsageQuery.error || modelDistributionQuery.error || cacheHitRateQuery.error || channelStatusQuery.error

  const tokenData = tokenUsageQuery.data?.data
  const modelData = modelDistributionQuery.data?.data
  const cacheData = cacheHitRateQuery.data?.data
  const channelData = channelStatusQuery.data?.data

  return (
    <>
      {loading ? <div className="event-state">正在加载首页概览…</div> : null}
      {hasError ? <div className="config-error" role="alert">首页概览加载失败，请检查后端接口状态。</div> : null}

      {!loading && !hasError ? (
        <DashboardAdminOverviewSection
          service={healthQuery.data?.service ?? '—'}
          adminAuth={healthQuery.data?.admin_auth ?? '—'}
          requests={summaryQuery.data?.requests ?? 0}
          cacheHitRate={formatPercent(summaryQuery.data?.cache_hit_rate)}
          providerErrorRate={formatPercent(summaryQuery.data?.provider_error_rate)}
          totalTokens={summaryQuery.data?.total_tokens ?? 0}
        />
      ) : null}

      <DashboardSessionOpsSection
        loading={sessionQuery.isLoading}
        hasError={!!sessionQuery.error}
        data={sessionQuery.data}
      />

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <Tabs tabs={CHART_TABS} activeKey={activeTab} onChange={(key) => setActiveTab(key as ChartTab)} />

        <div style={{ marginTop: '1rem' }}>
          {chartLoading ? <div className="event-state">加载图表数据中…</div> : null}
          {chartError ? <div className="config-error" role="alert">图表数据加载失败</div> : null}
          {!chartLoading && !chartError ? (
            <>
              {activeTab === 'tokens' && <TokenUsageChart data={tokenData} />}
              {activeTab === 'models' && <ModelDistributionChart data={modelData} />}
              {activeTab === 'cache' && <CacheHitRateChart data={cacheData} />}
              {activeTab === 'channels' && <ChannelStatusChart data={channelData} />}
            </>
          ) : null}
        </div>
      </div>
    </>
  )
}

export function DashboardPage() {
  const isUser = !!getUserToken()

  return (
    <AppShell
      title={isUser ? '我的面板' : '仪表盘'}
      description={isUser ? '查看您的配额使用、API Keys 和调用统计。' : '聚合展示服务状态、请求量、缓存命中率与 Provider 错误率，作为控制台首页。'}
    >
      <div className="events-page">
        {isUser ? <UserDashboardView /> : <DashboardAdminView />}
      </div>
    </AppShell>
  )
}
