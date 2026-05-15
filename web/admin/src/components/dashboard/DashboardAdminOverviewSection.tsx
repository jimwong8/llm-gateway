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
      <SummaryMetricCard label="Service" value={service} />
      <SummaryMetricCard label="Admin Auth" value={adminAuth} />
      <SummaryMetricCard label="Requests" value={requests} />
      <SummaryMetricCard label="Cache Hit Rate" value={cacheHitRate} />
      <SummaryMetricCard label="Provider Error Rate" value={providerErrorRate} />
      <SummaryMetricCard label="Total Tokens" value={totalTokens} />
    </div>
  )
}
