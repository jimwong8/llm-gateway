import { HomeTopStatusStrip } from './HomeTopStatusStrip'
import { mapTopMetrics } from './dashboard-home.mappers'
import { SecurityRadarPanel } from './SecurityRadarPanel'
import { OperationsActionPanel } from './OperationsActionPanel'
import { CostQuotaPanel } from './CostQuotaPanel'
import type { AdminHealth, AdminSummary } from '../../types/dashboard'
import type { Channel } from '../../types/channel'
import type { SecurityEventItem } from './dashboard-home.types'

type DashboardAdminOverviewSectionProps = {
  health: AdminHealth | undefined
  summary: AdminSummary | undefined
  channels: Channel[]
}

export function DashboardAdminOverviewSection({
  health,
  summary,
  channels,
}: DashboardAdminOverviewSectionProps) {
  const metrics = mapTopMetrics(health, summary, channels)

  // Mock security events for now - in real implementation this would come from API
  const mockSecurityEvents: SecurityEventItem[] = [
    {
      id: 'evt-1',
      level: 'critical',
      title: 'Key brute-force suspected',
      summary: 'IP 1.2.3.4 attempted 500+ keys in 5 minutes',
      timestamp: '14:23:11',
      source: '1.2.3.4',
    },
    {
      id: 'evt-2',
      level: 'warning',
      title: 'Unusual token usage spike',
      summary: 'User user-123 exceeded daily quota by 300%',
      timestamp: '14:20:05',
      source: 'user-123',
    }
  ];

  return (
    <section className="admin-overview-section">
      <HomeTopStatusStrip
        metrics={metrics}
        onClickMetric={(metric) => {
          // Handle metric click - for now just log
          console.log('Metric clicked:', metric)
        }}
      />
      <CostQuotaPanel
        todayCost={'--'}
        monthCost={'--'}
        cacheHitRate={`${(summary?.cache_hit_rate ?? 0).toFixed(1)}%`}
        providerErrorRate={`${(summary?.provider_error_rate ?? 0).toFixed(1)}%`}
        totalTokens={String(summary?.total_tokens ?? 0)}
      />
      <SecurityRadarPanel events={mockSecurityEvents} onSelectEvent={(event) => {
        console.log('Security event selected:', event);
      }} />
      <OperationsActionPanel />
    </section>
  )
}