import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { DashboardAdminOverviewSection } from '../components/dashboard/DashboardAdminOverviewSection'
import { DashboardSessionOpsSection } from '../components/dashboard/DashboardSessionOpsSection'
import { DashboardChartPanel, type ChartTab } from '../components/dashboard/DashboardChartPanel'
import { UserDashboardView } from '../components/dashboard/UserDashboardView'
import { DetailDrawer } from '../components/dashboard/DetailDrawer'
import { apiRequest } from '../lib/http'
import { getUserToken } from '../lib/api/identity'
import { getToken } from '../lib/auth'
import { listChannels } from '../lib/channels'
import { mapChannelNodes } from '../components/dashboard/dashboard-home.mappers'
import { ChannelHealthMatrix } from '../components/dashboard/ChannelHealthMatrix'
import type { AdminHealth, AdminSummary } from '../types/dashboard'
import type { Channel } from '../types/channel'
import type { SessionAdminDashboard } from '../types/sessionDashboard'
import type { DrawerPayload } from '../components/dashboard/dashboard-home.types'

function DashboardAdminView() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<ChartTab>('tokens')
  const [drawerPayload, setDrawerPayload] = useState<DrawerPayload>(null)

  // 基础状态查询（3 个）
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

  const loading = healthQuery.isLoading || summaryQuery.isLoading || channelsQuery.isLoading

  // 部分失败不再整体隐藏数据：只要有一个 API 成功就渲染已加载的部分。
  // 全部失败才显示 dashboard-error banner；部分失败显示顶部 warning 条。
  const anyError = !!(healthQuery.error || summaryQuery.error || channelsQuery.error)
  const allFailed = !healthQuery.data && !summaryQuery.data && !channelsQuery.data && anyError

  const channelNodes = mapChannelNodes(channelsQuery.data ?? [])

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
        <div className="dashboard-loading" role="status">
          <div className="dashboard-loading__spinner" />
          <span>{t('dashboard.loading')}</span>
        </div>
      ) : null}

      {allFailed ? (
        <div className="dashboard-error" role="alert">
          <span className="dashboard-error__icon">⚠️</span>
          <span>{t('dashboard.loadError')}</span>
        </div>
      ) : null}

      {!allFailed && anyError && !loading ? (
        <div className="dashboard-warning" role="status">
          <span>⚠️</span>
          <span>
            {healthQuery.error ? '服务状态' : ''}
            {healthQuery.error && summaryQuery.error ? ' · ' : ''}
            {summaryQuery.error ? '监控摘要' : ''}
            {summaryQuery.error && channelsQuery.error ? ' · ' : ''}
            {channelsQuery.error ? '渠道列表' : ''}
            加载失败，其他数据仍正常显示。
          </span>
        </div>
      ) : null}

      {!loading && !allFailed ? (
        <>
          <DashboardAdminOverviewSection
            health={healthQuery.data}
            summary={summaryQuery.data}
            channels={channelsQuery.data ?? []}
            sessionData={sessionQuery.data ?? undefined}
          />
          <ChannelHealthMatrix
            channels={channelNodes}
            onSelect={(node) => {
              setDrawerPayload({ kind: 'channel', channelId: node.id })
            }}
          />
        </>
      ) : null}

      <DashboardSessionOpsSection
        loading={sessionQuery.isLoading}
        hasError={!!sessionQuery.error}
        data={sessionQuery.data}
      />

      {/* 图表面板：6 个 useQuery 移到内部，切 tab 时未激活 tab 的 query 根本不注册 */}
      <DashboardChartPanel activeTab={activeTab} onTabChange={setActiveTab} />

      {/* 详情抽屉 */}
      <DetailDrawer
        open={!!drawerPayload}
        title={
          drawerPayload?.kind === 'channel'
            ? '渠道详情'
            : drawerPayload?.kind === 'security-event'
              ? '安全事件'
              : drawerPayload?.kind === 'cost-item'
                ? '费用项目'
                : '详情'
        }
        subtitle={drawerPayload?.kind === 'channel' ? `查看渠道 ${drawerPayload.channelId}` : undefined}
        onClose={() => setDrawerPayload(null)}
      >
        {drawerPayload?.kind === 'channel' && (
          <div>
            <p>渠道 ID: {drawerPayload.channelId}</p>
            <p>此处应显示渠道详细信息</p>
          </div>
        )}
        {drawerPayload?.kind === 'security-event' && (
          <div>
            <p>事件 ID: {drawerPayload.eventId}</p>
            <p>此处应显示安全事件详细信息</p>
          </div>
        )}
        {drawerPayload?.kind === 'cost-item' && (
          <div>
            <p>项目 ID: {drawerPayload.itemId}</p>
            <p>此处应显示费用项目详细信息</p>
          </div>
        )}
      </DetailDrawer>
    </>
  )
}

function useUserRole(): 'admin' | 'user' | null {
  // 优先检查 admin token（Bearer llm_gateway_admin_token）
  // 有 admin token = admin 模式（后端直接对比 key 值，不走 JWT role）
  const adminToken = getToken()
  if (adminToken && adminToken.trim().length > 4) {
    return 'admin'
  }
  const userToken = getUserToken()
  if (!userToken) return null
  // 尝试解析 JWT role；兼容 role 是字符串或整数两种编码
  try {
    const parts = userToken.split('.')
    if (parts.length !== 3) return 'user'
    const payload = JSON.parse(atob(parts[1]))
    const role = payload.role
    // 后端 JWT Claims.Role 是 string，user 表 Role 是 int
    // 常见映射：0=disabled, 1=user, 2=operator, 3=admin, 4=readonly
    const roleStr = typeof role === 'string' ? role : String(role)
    if (roleStr === 'admin' || roleStr === 'operator' || roleStr === 'readonly' || roleStr === '2' || roleStr === '3' || roleStr === '4') {
      return 'admin'
    }
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