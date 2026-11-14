import { Navigate, Outlet, useLocation } from 'react-router'

import { useAuth } from '@/auth/AuthProvider'
import { returnPath } from '@/lib/returnPath'

export function RequireAuth() {
  const { token } = useAuth()
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}

export function GuestOnly() {
  const { token } = useAuth()
  const location = useLocation()
  if (token) return <Navigate to={returnPath(location.state)} replace />
  return <Outlet />
}
