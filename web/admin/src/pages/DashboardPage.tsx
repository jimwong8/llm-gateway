import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { UserDashboardView } from '../components/dashboard/UserDashboardView'
import { TokenUsageChart, ModelDistributionChart, CacheHitRateChart, ChannelStatusChart, LatencyChart, ErrorRateChart } from '../components/charts'
import { SimpleTabs } from '../components/ui/simple-tabs'
import { apiRequest } from '../lib/http'
import { getUserToken } from '../lib/api/identity'
import { getTokenUsage, getModelDistribution, getCacheHitRate, getChannelStatus, getLatencyTrend, getErrorRateTrend } from '../lib/api/dashboard'
import { listChannels } from '../lib/channels'
import type { AdminHealth, AdminSummary, TokenUsagePoint, ModelDistributionPoint, CacheHitPoint, ChannelStatusPoint, LatencyPoint, ErrorRatePoint } from '../types/dashboard'
import type { Channel } from '../types/channel'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import { formatPercent } from '../lib/format'

type ChartTab = 'tokens' | 'models' | 'cache' | 'channels' | 'latency' | 'errorRate'

const CHART_TAB_CONFIG: { key: ChartTab; label: string; icon: string }[] = [
  { key: 'tokens', label: 'Token 趋势', icon: '📊' },
  { key: 'models', label: '模型分布', icon: '🧩' },
  { key: 'cache', label: '缓存命中', icon: '⚡' },
  { key: 'channels', label: '渠道状态', icon: '🔗' },
  { key: 'latency', label: '延迟趋势', icon: '⏱️' },
  { key: 'errorRate', label: '错误率', icon: '🛡️' },
]

function DashboardAdminView() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<ChartTab>('tokens')

  const healthQuery = useQuery({
    queryKey: ['dashboard-health'],
    queryFn: () => apiRequest<AdminHealth>('/admin/health'),
    refetchInterval: 30_000,
  })

  const summaryQuery = useQuery({
    queryKey: ['dashboard-observability-summary'],
    queryFn: () => apiRequest<AdminSummary>('/admin/observability/summary'),
    refetchInterval: 30_000,
  })

  const channelsQuery = useQuery({
    queryKey: ['dashboard-channels'],
    queryFn: listChannels,
    refetchInterval: 30_000,
  })

  const sessionQuery = useQuery({
    queryKey: ['dashboard-session'],
    queryFn: () => apiRequest<SessionAdminDashboard>('/admin/dashboard'),
    retry: false,
  })

  const tokenUsageQuery = useQuery({
    queryKey: ['dashboard-charts', 'token-usage'],
    queryFn: () => getTokenUsage(7),
    refetchInterval: 30_000,
    enabled: activeTab === 'tokens',
  })

  const modelDistributionQuery = useQuery({
    queryKey: ['dashboard-charts', 'model-distribution'],
    queryFn: getModelDistribution,
    refetchInterval: 30_000,
    enabled: activeTab === 'models',
  })

  const cacheHitRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'cache-hit-rate'],
    queryFn: () => getCacheHitRate(7),
    refetchInterval: 30_000,
    enabled: activeTab === 'cache',
  })

  const channelStatusQuery = useQuery({
    queryKey: ['dashboard-charts', 'channel-status'],
    queryFn: getChannelStatus,
    refetchInterval: 30_000,
    enabled: activeTab === 'channels',
  })

  const latencyQuery = useQuery({
    queryKey: ['dashboard-charts', 'latency'],
    queryFn: () => getLatencyTrend(7),
    refetchInterval: 30_000,
    enabled: activeTab === 'latency',
  })

  const errorRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'error-rate'],
    queryFn: () => getErrorRateTrend(7),
    refetchInterval: 30_000,
    enabled: activeTab === 'errorRate',
  })

  const loading = healthQuery.isLoading || summaryQuery.isLoading || channelsQuery.isLoading
  const hasError = healthQuery.error || summaryQuery.error || channelsQuery.error
  const chartLoading = tokenUsageQuery.isLoading || modelDistributionQuery.isLoading || cacheHitRateQuery.isLoading || channelStatusQuery.isLoading || latencyQuery.isLoading || errorRateQuery.isLoading
  const chartError = tokenUsageQuery.error || modelDistributionQuery.error || cacheHitRateQuery.error || channelStatusQuery.error || latencyQuery.error || errorRateQuery.error

  const tokenData = tokenUsageQuery.data?.data
  const modelData = modelDistributionQuery.data?.data
  const cacheData = cacheHitRateQuery.data?.data
  const channelData = channelStatusQuery.data?.data
  const latencyData = latencyQuery.data?.data
  const errorRateData = errorRateQuery.data?.data

  return (
    <>
      {/* 背景装饰 */}
      <div className="dashboard-bg-decoration" aria-hidden="true">
        <div className="dashboard-bg-decoration__orb dashboard-bg-decoration__orb--1" />
        <div className="dashboard-bg-decoration__orb dashboard-bg-decoration__orb--2" />
        <div className="dashboard-bg-decoration__orb dashboard-bg-decoration__orb--3" />
        <div className="dashboard-bg-decoration__grid" />
      </div>

      {loading ? (
        <div className="dashboard-loading">
          <div className="dashboard-loading__spinner" />
          <span>{t('dashboard.loading')}</span>
        </div>
      ) : null}

      {hasError ? (
        <div className="dashboard-error" role="alert">
          <span className="dashboard-error__icon">⚠️</span>
          <span>{t('dashboard.loadError')}</span>
        </div>
      ) : null}

      {!loading && !hasError ? (
        <DashboardAdminOverviewSection
          health={healthQuery.data}
          summary={summaryQuery.data}
          channels={channelsQuery.data ?? []}
        />
      ) : null}

      <DashboardSessionOpsSection
        loading={sessionQuery.isLoading}
        hasError={!!sessionQuery.error}
        data={sessionQuery.data}
      />

      {/* 图表面板 */}
      <div className="dashboard-charts-panel">
        <div className="dashboard-charts-panel__header">
          <h2 className="dashboard-charts-panel__title">📈 数据分析</h2>
          <p className="dashboard-charts-panel__subtitle">实时监控与趋势分析</p>
        </div>

        <div className="dashboard-charts-panel__tabs">
          {CHART_TAB_CONFIG.map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={`dashboard-chart-tab ${activeTab === tab.key ? 'dashboard-chart-tab--active' : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              <span className="dashboard-chart-tab__icon">{tab.icon}</span>
              <span className="dashboard-chart-tab__label">{t(`dashboard.chart${tab.key.charAt(0).toUpperCase() + tab.key.slice(1)}`) || tab.label}</span>
            </button>
          ))}
        </div>

        <div className="dashboard-charts-panel__content">
          {chartLoading ? (
            <div className="dashboard-chart-loading">
              <div className="dashboard-chart-loading__spinner" />
              <span>{t('dashboard.chartLoading')}</span>
            </div>
          ) : null}
          {chartError ? (
            <div className="dashboard-chart-error" role="alert">
              <span>{t('dashboard.chartError')}</span>
            </div>
          ) : null}
          {!chartLoading && !chartError ? (
            <div className="dashboard-chart-container">
              {activeTab === 'tokens' && <TokenUsageChart data={tokenData} />}
              {activeTab === 'models' && <ModelDistributionChart data={modelData} />}
              {activeTab === 'cache' && <CacheHitRateChart data={cacheData} />}
              {activeTab === 'channels' && <ChannelStatusChart data={channelData} />}
              {activeTab === 'latency' && <LatencyChart data={latencyData} />}
              {activeTab === 'errorRate' && <ErrorRateChart data={errorRateData} />}
            </div>
          ) : null}
        </div>
      </div>
    </>
  )
}

function useUserRole(): 'admin' | 'user' | null {
  const userToken = getUserToken()
  if (!userToken) return null
  try {
    const parts = userToken.split('.')
    if (parts.length !== 3) return 'user'
    const payload = JSON.parse(atob(parts[1]))
    const role = payload.role
    if (role === 'admin' || role === 'operator' || role === 'readonly') return 'admin'
    return 'user'
  } catch {
    return 'user'
  }
}

export function DashboardPage() {
  const { t } = useTranslation()
  const role = useUserRole()
  const isAdmin = role === 'admin'
  const isUser = !!getUserToken()

  return (
    <AppShell
      title={isUser && !isAdmin ? t('dashboard.myPanel') : t('dashboard.title')}
      description={isUser && !isAdmin ? t('dashboard.userDescription') : t('dashboard.description')}
    >
      <div className="dashboard-page">
        {isUser && !isAdmin ? <UserDashboardView /> : <DashboardAdminView />}
      </div>
    </AppShell>
  )
}
