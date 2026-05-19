import { lazy } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { ProtectedRoute } from './components/auth/ProtectedRoute'

const AccountPage = lazy(() => import('./pages/AccountPage').then(m => ({ default: m.AccountPage })))
const ApprovalsPage = lazy(() => import('./pages/ApprovalsPage').then(m => ({ default: m.ApprovalsPage })))
const ChatPage = lazy(() => import('./pages/ChatPage').then(m => ({ default: m.ChatPage })))
const AuditRuntimePage = lazy(() => import('./pages/AuditRuntimePage').then(m => ({ default: m.AuditRuntimePage })))
const ConfigCenterPage = lazy(() => import('./pages/ConfigCenterPage').then(m => ({ default: m.ConfigCenterPage })))
const DashboardPage = lazy(() => import('./pages/DashboardPage').then(m => ({ default: m.DashboardPage })))
const DriftDashboardPage = lazy(() => import('./pages/DriftDashboardPage').then(m => ({ default: m.DriftDashboardPage })))
const MemoryGovernancePage = lazy(() => import('./pages/MemoryGovernancePage').then(m => ({ default: m.MemoryGovernancePage })))
const ApiKeysPage = lazy(() => import('./pages/ApiKeysPage').then(m => ({ default: m.ApiKeysPage })))
const SignupPage = lazy(() => import('./pages/SignupPage').then(m => ({ default: m.SignupPage })))
const LoginPage = lazy(() => import('./pages/LoginPage').then(m => ({ default: m.LoginPage })))
const ObservabilityPage = lazy(() => import('./pages/ObservabilityPage').then(m => ({ default: m.ObservabilityPage })))
const PlaygroundPage = lazy(() => import('./pages/PlaygroundPage').then(m => ({ default: m.PlaygroundPage })))
const RecommendationCenterPage = lazy(() => import('./pages/RecommendationCenterPage').then(m => ({ default: m.RecommendationCenterPage })))
const PoliciesPage = lazy(() => import('./pages/PoliciesPage').then(m => ({ default: m.PoliciesPage })))
const PolicyVersionsPage = lazy(() => import('./pages/PolicyVersionsPage').then(m => ({ default: m.PolicyVersionsPage })))
const QuotaPage = lazy(() => import('./pages/QuotaPage').then(m => ({ default: m.QuotaPage })))
const ReleasesPage = lazy(() => import('./pages/ReleasesPage').then(m => ({ default: m.ReleasesPage })))
const RolloutsPage = lazy(() => import('./pages/RolloutsPage').then(m => ({ default: m.RolloutsPage })))
const RuntimeObserverPage = lazy(() => import('./pages/RuntimeObserverPage').then(m => ({ default: m.RuntimeObserverPage })))
const SystemPage = lazy(() => import('./pages/SystemPage').then(m => ({ default: m.SystemPage })))
const ChannelsPage = lazy(() => import('./pages/ChannelsPage').then(m => ({ default: m.ChannelsPage })))
const AssetsPage = lazy(() => import('./pages/AssetsPage').then(m => ({ default: m.AssetsPage })))
const TenantKeysPage = lazy(() => import('./pages/TenantKeysPage').then(m => ({ default: m.TenantKeysPage })))
const AuditExportPage = lazy(() => import('./pages/AuditExportPage').then(m => ({ default: m.AuditExportPage })))
const BillingPage = lazy(() => import('./pages/BillingPage').then(m => ({ default: m.BillingPage })))
const PricingPage = lazy(() => import('./pages/admin/PricingPage').then(m => ({ default: m.PricingPage })))
const BroadcastPage = lazy(() => import('./pages/admin/BroadcastPage').then(m => ({ default: m.BroadcastPage })))
const PresetsPage = lazy(() => import('./pages/PresetsPage').then(m => ({ default: m.PresetsPage })))
const WsChatPage = lazy(() => import('./pages/WsChatPage').then(m => ({ default: m.WsChatPage })))
const OAuthCallbackPage = lazy(() => import('./pages/OAuthCallback').then(m => ({ default: m.OAuthCallbackPage })))
const ForgotPasswordPage = lazy(() => import('./pages/ForgotPassword').then(m => ({ default: m.ForgotPasswordPage })))
const ResetPasswordPage = lazy(() => import('./pages/ResetPassword').then(m => ({ default: m.ResetPasswordPage })))

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
        path: 'ws-chat',
        element: <WsChatPage />,
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
