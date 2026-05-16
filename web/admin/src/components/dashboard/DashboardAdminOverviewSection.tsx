import { SummaryMetricCard } from './SummaryMetricCard'

type DashboardAdminOverviewSectionProps = {
  service: string | number
  adminAuth: string | number
  requests: string | number
  cacheHitRate: string | number
  providerErrorRate: string | number
  totalTokens: string | number
}

export function DashboardAdminOverviewSection({
  service,
  adminAuth,
  requests,
  cacheHitRate,
  providerErrorRate,
  totalTokens,
}: DashboardAdminOverviewSectionProps) {
  return (
    <div className="summary-card-grid">
      <SummaryMetricCard label="服务" value={service} />
      <SummaryMetricCard label="管理员认证" value={adminAuth} />
      <SummaryMetricCard label="请求量" value={requests} />
      <SummaryMetricCard label="缓存命中率" value={cacheHitRate} />
      <SummaryMetricCard label="Provider 错误率" value={providerErrorRate} />
      <SummaryMetricCard label="总 Token 数" value={totalTokens} />
    </div>
  )
}
