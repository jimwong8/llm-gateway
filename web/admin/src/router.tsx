import { createBrowserRouter, Navigate } from 'react-router-dom'
import { ProtectedRoute } from './components/auth/ProtectedRoute'
import { ConfigCenterPage } from './pages/ConfigCenterPage'
import { AuditRuntimePage } from './pages/AuditRuntimePage'
import { DashboardPage } from './pages/DashboardPage'
import { LoginPage } from './pages/LoginPage'
import { ObservabilityPage } from './pages/ObservabilityPage'
import { PlaygroundPage } from './pages/PlaygroundPage'
import { PoliciesPage } from './pages/PoliciesPage'
import { QuotaPage } from './pages/QuotaPage'
import { ReleasesPage } from './pages/ReleasesPage'
import { SystemPage } from './pages/SystemPage'

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
    ],
  },
], {
  basename: '/admin/ui',
})
