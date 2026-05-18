import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { UserDashboardView } from '../components/dashboard/UserDashboardView'
import { TokenUsageChart, ModelDistributionChart, CacheHitRateChart, ChannelStatusChart } from '../components/charts'
import { Tabs } from '../components/ui'
import { apiRequest } from '../lib/http'
import { getUserToken } from '../lib/api/identity'
import { getTokenUsage, getModelDistribution, getCacheHitRate, getChannelStatus } from '../lib/api/dashboard'
import type { AdminSummary } from '../types/dashboard'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import { formatPercent } from '../lib/format'

type AdminHealth = {
  service?: string
  admin_auth?: string
  time?: string
}

type ChartTab = 'tokens' | 'models' | 'cache' | 'channels'



type TokenUsagePoint = { date: string; prompt: number; completion: number; total: number }
type ModelDistributionPoint = { name: string; value: number }
type CacheHitPoint = { date: string; hitRate: number; requests: number }
type ChannelStatusPoint = { name: string; healthy: number; degraded: number; down: number }

function DashboardAdminView() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<ChartTab>('tokens')

  const CHART_TABS = [
    { key: 'tokens', label: t('dashboard.chartTokens') },
    { key: 'models', label: t('dashboard.chartModels') },
    { key: 'cache', label: t('dashboard.chartCache') },
    { key: 'channels', label: t('dashboard.chartChannels') },
  ]

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

  const sessionQuery = useQuery({
    queryKey: ['dashboard-session'],
    queryFn: () => apiRequest<SessionAdminDashboard>('/admin/dashboard'),
    retry: false,
  })

  const tokenUsageQuery = useQuery({
    queryKey: ['dashboard-charts', 'token-usage'],
    queryFn: () => getTokenUsage(7),
    refetchInterval: 30_000,
  })

  const modelDistributionQuery = useQuery({
    queryKey: ['dashboard-charts', 'model-distribution'],
    queryFn: getModelDistribution,
    refetchInterval: 30_000,
  })

  const cacheHitRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'cache-hit-rate'],
    queryFn: () => getCacheHitRate(7),
    refetchInterval: 30_000,
  })

  const channelStatusQuery = useQuery({
    queryKey: ['dashboard-charts', 'channel-status'],
    queryFn: getChannelStatus,
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
      {loading ? <div className="event-state">{t('dashboard.loading')}</div> : null}
      {hasError ? <div className="config-error" role="alert">{t('dashboard.loadError')}</div> : null}

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
          {chartLoading ? <div className="event-state">{t('dashboard.chartLoading')}</div> : null}
          {chartError ? <div className="config-error" role="alert">{t('dashboard.chartError')}</div> : null}
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
      <div className="events-page">
        {isUser && !isAdmin ? <UserDashboardView /> : <DashboardAdminView />}
      </div>
    </AppShell>
  )
}
