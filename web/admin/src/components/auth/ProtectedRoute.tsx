import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { hasToken } from '../../lib/auth'

export function ProtectedRoute() {
  const location = useLocation()

  if (!hasToken()) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <Outlet />
}
