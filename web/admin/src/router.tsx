import { createBrowserRouter, Navigate } from 'react-router-dom'
import { ProtectedRoute } from './components/auth/ProtectedRoute'
import { ApprovalsPage } from './pages/ApprovalsPage'
import { AuditRuntimePage } from './pages/AuditRuntimePage'
import { ConfigCenterPage } from './pages/ConfigCenterPage'
import { DashboardPage } from './pages/DashboardPage'
import { DriftDashboardPage } from './pages/DriftDashboardPage'
import { MemoryGovernancePage } from './pages/MemoryGovernancePage'
import { LoginPage } from './pages/LoginPage'
import { ObservabilityPage } from './pages/ObservabilityPage'
import { PlaygroundPage } from './pages/PlaygroundPage'
import { RecommendationCenterPage } from './pages/RecommendationCenterPage'
import { PoliciesPage } from './pages/PoliciesPage'
import { PolicyVersionsPage } from './pages/PolicyVersionsPage'
import { QuotaPage } from './pages/QuotaPage'
import { ReleasesPage } from './pages/ReleasesPage'
import { RolloutsPage } from './pages/RolloutsPage'
import { RuntimeObserverPage } from './pages/RuntimeObserverPage'
import { SystemPage } from './pages/SystemPage'
import { ChannelsPage } from './pages/ChannelsPage'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Navigate to="dashboard" replace />,
  },
  {
    path: 'login',
    element: <LoginPage />,
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        path: 'dashboard',
        element: <DashboardPage />,
      },
      {
        path: 'config-center',
        element: <ConfigCenterPage />,
      },
      {
        path: 'releases',
        element: <ReleasesPage />,
      },
      {
        path: 'audit-runtime',
        element: <AuditRuntimePage />,
      },
      {
        path: 'channels',
        element: <ChannelsPage />,
      },
      {
        path: 'system',
        element: <SystemPage />,
      },
      {
        path: 'playground',
        element: <PlaygroundPage />,
      },
      {
        path: 'observability',
        element: <ObservabilityPage />,
      },
      {
        path: 'quota',
        element: <QuotaPage />,
      },
      {
        path: 'policies',
        element: <PoliciesPage />,
      },
      {
        path: 'memory-governance',
        element: <MemoryGovernancePage />,
      },
      {
        path: 'recommendations',
        element: <RecommendationCenterPage />,
      },
      {
        path: 'approvals',
        element: <ApprovalsPage />,
      },
      {
        path: 'policy-versions',
        element: <PolicyVersionsPage />,
      },
      {
        path: 'rollouts',
        element: <RolloutsPage />,
      },
      {
        path: 'runtime-observer',
        element: <RuntimeObserverPage />,
      },
      {
        path: 'drifts',
        element: <DriftDashboardPage />,
      },
      {
        path: '*',
        element: <Navigate to="/dashboard" replace />,
      },
    ],
  },
], {
  basename: '/admin/ui',
})
