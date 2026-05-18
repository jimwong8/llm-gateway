import { createBrowserRouter, Navigate } from 'react-router-dom'
import { ProtectedRoute } from './components/auth/ProtectedRoute'
import { AccountPage } from './pages/AccountPage'
import { ApprovalsPage } from './pages/ApprovalsPage'
import { ChatPage } from './pages/ChatPage'
import { AuditRuntimePage } from './pages/AuditRuntimePage'
import { ConfigCenterPage } from './pages/ConfigCenterPage'
import { DashboardPage } from './pages/DashboardPage'
import { DriftDashboardPage } from './pages/DriftDashboardPage'
import { MemoryGovernancePage } from './pages/MemoryGovernancePage'
import { ApiKeysPage } from './pages/ApiKeysPage'
import { SignupPage } from './pages/SignupPage'
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
import { AssetsPage } from './pages/AssetsPage'
import { TenantKeysPage } from './pages/TenantKeysPage'
import { AuditExportPage } from './pages/AuditExportPage'
import { BillingPage } from './pages/BillingPage'
import { PricingPage } from './pages/admin/PricingPage'
import { BroadcastPage } from './pages/admin/BroadcastPage'
import { PresetsPage } from './pages/PresetsPage'
import { OAuthCallbackPage } from './pages/OAuthCallback'
import { ForgotPasswordPage } from './pages/ForgotPassword'
import { ResetPasswordPage } from './pages/ResetPassword'

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
    path: 'signup',
    element: <SignupPage />,
  },
  {
    path: 'oauth/callback',
    element: <OAuthCallbackPage />,
  },
  {
    path: 'forgot-password',
    element: <ForgotPasswordPage />,
  },
  {
    path: 'reset-password',
    element: <ResetPasswordPage />,
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
        path: 'api-keys',
        element: <ApiKeysPage />,
      },
      {
        path: 'chat',
        element: <ChatPage />,
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
        path: 'assets',
        element: <AssetsPage />,
      },
      {
        path: 'tenant-keys',
        element: <TenantKeysPage />,
      },
      {
        path: 'audit-export',
        element: <AuditExportPage />,
      },
      {
        path: 'system',
        element: <SystemPage />,
      },
      {
        path: 'system/settings',
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
        path: 'presets',
        element: <PresetsPage />,
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
        path: 'billing',
        element: <BillingPage />,
      },
      {
        path: 'account',
        element: <AccountPage />,
      },
      {
        path: 'billing-pricing',
        element: <PricingPage />,
      },
      {
        path: 'broadcasts',
        element: <BroadcastPage />,
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
