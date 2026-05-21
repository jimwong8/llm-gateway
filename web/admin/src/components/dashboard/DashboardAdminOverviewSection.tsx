import { HomeTopStatusStrip } from './HomeTopStatusStrip'
import { mapTopMetrics } from './dashboard-home.mappers'
import type { AdminHealth, AdminSummary } from '../../types/dashboard'
import type { Channel } from '../../types/channel'

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

  return (
    <section className="admin-overview-section">
      <HomeTopStatusStrip
        metrics={metrics}
        onClickMetric={(metric) => {
          // Handle metric click - for now just log
          console.log('Metric clicked:', metric)
        }}
      />
    </section>
  )
}
