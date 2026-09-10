import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { TokenUsageChart, ModelDistributionChart, CacheHitRateChart, ChannelStatusChart, LatencyChart, ErrorRateChart } from '../charts'
import { CHART_TAB_CONFIG, type ChartTab } from './ChartTabConfig'
export type { ChartTab }
import { getTokenUsage, getModelDistribution, getCacheHitRate, getChannelStatus, getLatencyTrend, getErrorRateTrend } from '../../lib/api/dashboard'

type ChartPanelProps = {
  activeTab: ChartTab
  onTabChange: (tab: ChartTab) => void
}

/**
 * 独立的图表面板组件。
 *
 * 之前 6 个 useQuery 全部注册在 DashboardAdminView 顶层，即使 enabled: false
 * 也会保留订阅者；且每次父组件重渲染会重新创建 6 个 query config 对象。
 *
 * 现在每个 query 只在自己的组件挂载时才创建。切 tab 时未激活 tab 的
 * useQuery 根本不注册，refetchInterval 也不会启动。
 */
function useActiveChartQuery(tab: ChartTab) {
  const tokenUsageQuery = useQuery({
    queryKey: ['dashboard-charts', 'token-usage'],
    queryFn: () => getTokenUsage(7),
    refetchInterval: 30_000,
    enabled: tab === 'tokens',
  })
  const modelDistributionQuery = useQuery({
    queryKey: ['dashboard-charts', 'model-distribution'],
    queryFn: getModelDistribution,
    refetchInterval: 30_000,
    enabled: tab === 'models',
  })
  const cacheHitRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'cache-hit-rate'],
    queryFn: () => getCacheHitRate(7),
    refetchInterval: 30_000,
    enabled: tab === 'cache',
  })
  const channelStatusQuery = useQuery({
    queryKey: ['dashboard-charts', 'channel-status'],
    queryFn: getChannelStatus,
    refetchInterval: 30_000,
    enabled: tab === 'channels',
  })
  const latencyQuery = useQuery({
    queryKey: ['dashboard-charts', 'latency'],
    queryFn: () => getLatencyTrend(7),
    refetchInterval: 30_000,
    enabled: tab === 'latency',
  })
  const errorRateQuery = useQuery({
    queryKey: ['dashboard-charts', 'error-rate'],
    queryFn: () => getErrorRateTrend(7),
    refetchInterval: 30_000,
    enabled: tab === 'errorRate',
  })

  return {
    tokenData: tokenUsageQuery.data?.data,
    modelData: modelDistributionQuery.data?.data,
    cacheData: cacheHitRateQuery.data?.data,
    channelData: channelStatusQuery.data?.data,
    latencyData: latencyQuery.data?.data,
    errorRateData: errorRateQuery.data?.data,
    isLoading:
      tokenUsageQuery.isLoading ||
      modelDistributionQuery.isLoading ||
      cacheHitRateQuery.isLoading ||
      channelStatusQuery.isLoading ||
      latencyQuery.isLoading ||
      errorRateQuery.isLoading,
    error:
      tokenUsageQuery.error ||
      modelDistributionQuery.error ||
      cacheHitRateQuery.error ||
      channelStatusQuery.error ||
      latencyQuery.error ||
      errorRateQuery.error,
  }
}

export function DashboardChartPanel({ activeTab, onTabChange }: ChartPanelProps) {
  const { t } = useTranslation()
  const { tokenData, modelData, cacheData, channelData, latencyData, errorRateData, isLoading, error } =
    useActiveChartQuery(activeTab)

  return (
    <div className="dashboard-charts-panel">
      <div className="dashboard-charts-panel__header">
        <h2 className="dashboard-charts-panel__title">📈 数据分析</h2>
        <p className="dashboard-charts-panel__subtitle">实时监控与趋势分析</p>
      </div>

      <div className="dashboard-charts-panel__tabs" role="tablist">
        {CHART_TAB_CONFIG.map((tab) => {
          const Icon = tab.icon
          const isActive = activeTab === tab.key
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              className={`dashboard-chart-tab ${isActive ? 'dashboard-chart-tab--active' : ''}`}
              onClick={() => onTabChange(tab.key)}
            >
              <Icon className="dashboard-chart-tab__icon" size={16} aria-hidden="true" />
              <span className="dashboard-chart-tab__label">{tab.label}</span>
            </button>
          )
        })}
      </div>

      <div className="dashboard-charts-panel__content">
        {isLoading ? (
          <div className="dashboard-chart-loading" role="status">
            <div className="dashboard-chart-loading__spinner" />
            <span>{t('dashboard.chartLoading')}</span>
          </div>
        ) : null}
        {error ? (
          <div className="dashboard-chart-error" role="alert">
            <span>{t('dashboard.chartError')}</span>
          </div>
        ) : null}
        {!isLoading && !error ? (
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
  )
}
