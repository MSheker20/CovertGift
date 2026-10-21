import { Navigate, Outlet, useLocation } from 'react-router'

import { useAuth } from '@/auth/AuthProvider'

export function RequireAuth() {
  const { token } = useAuth()
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}

export function GuestOnly() {
  const { token } = useAuth()
  if (token) return <Navigate to="/" replace />
  return <Outlet />
}
