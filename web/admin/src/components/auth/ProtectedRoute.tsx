import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { hasToken } from '../../lib/auth'
import { hasUserToken } from '../../lib/api/identity'

export function ProtectedRoute() {
  const location = useLocation()

  if (!hasToken() && !hasUserToken()) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <Outlet />
}
