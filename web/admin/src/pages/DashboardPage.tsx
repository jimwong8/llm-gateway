import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { TokenUsageChart, ModelDistributionChart, CacheHitRateChart, ChannelStatusChart } from '../components/charts'
import { apiRequest } from '../lib/http'
import type { BillingSummary } from '../types/observability'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import { formatPercent } from '../lib/format'

type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

type ChartTab = 'tokens' | 'models' | 'cache' | 'channels'

const CHART_TABS: { key: ChartTab; label: string }[] = [
  { key: 'tokens', label: 'Token 趋势' },
  { key: 'models', label: '模型分布' },
  { key: 'cache', label: '缓存命中' },
  { key: 'channels', label: '渠道状态' },
]

export function DashboardPage() {
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

  const loading = healthQuery.isLoading || summaryQuery.isLoading
  const hasError = healthQuery.error || summaryQuery.error

  const mockTokenData = useMemo(() => [
    { date: '04/10', prompt: 120000, completion: 80000, total: 200000 },
    { date: '04/11', prompt: 150000, completion: 95000, total: 245000 },
    { date: '04/12', prompt: 110000, completion: 70000, total: 180000 },
    { date: '04/13', prompt: 180000, completion: 120000, total: 300000 },
    { date: '04/14', prompt: 200000, completion: 140000, total: 340000 },
    { date: '04/15', prompt: 165000, completion: 110000, total: 275000 },
    { date: '04/16', prompt: 220000, completion: 160000, total: 380000 },
  ], [])

  const mockModelData = useMemo(() => [
    { name: 'GPT-4', value: 35 },
    { name: 'GPT-3.5', value: 25 },
    { name: 'Claude-3', value: 20 },
    { name: 'Gemini', value: 12 },
    { name: 'Other', value: 8 },
  ], [])

  const mockCacheData = useMemo(() => [
    { date: '04/10', hitRate: 72, requests: 15000 },
    { date: '04/11', hitRate: 78, requests: 18000 },
    { date: '04/12', hitRate: 65, requests: 12000 },
    { date: '04/13', hitRate: 81, requests: 22000 },
    { date: '04/14', hitRate: 85, requests: 25000 },
    { date: '04/15', hitRate: 74, requests: 16000 },
    { date: '04/16', hitRate: 88, requests: 28000 },
  ], [])

  const mockChannelData = useMemo(() => [
    { name: 'OpenAI', healthy: 95, degraded: 3, down: 2 },
    { name: 'Anthropic', healthy: 90, degraded: 7, down: 3 },
    { name: 'Azure', healthy: 88, degraded: 8, down: 4 },
    { name: 'Google', healthy: 92, degraded: 5, down: 3 },
  ], [])

  return (
    <AppShell
      title="仪表盘"
      description="聚合展示服务状态、请求量、缓存命中率与 Provider 错误率，作为控制台首页。"
    >
      <div className="events-page">
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
          <div className="chart-tabs">
            <div className="tab-strip">
              {CHART_TABS.map((tab) => (
                <button
                  key={tab.key}
                  type="button"
                  className={activeTab === tab.key ? 'tab active' : 'tab'}
                  onClick={() => setActiveTab(tab.key)}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ marginTop: '1rem' }}>
            {activeTab === 'tokens' && <TokenUsageChart data={mockTokenData} />}
            {activeTab === 'models' && <ModelDistributionChart data={mockModelData} />}
            {activeTab === 'cache' && <CacheHitRateChart data={mockCacheData} />}
            {activeTab === 'channels' && <ChannelStatusChart data={mockChannelData} />}
          </div>
        </div>
      </div>
    </AppShell>
  )
}
