import { Navigate, Outlet, createBrowserRouter, useLocation } from 'react-router'

import { useAuth } from '@/auth/AuthProvider'
import { AppShell } from '@/pages/AppShell'
import { GroupPage } from '@/pages/GroupPage'
import { GroupsPage } from '@/pages/GroupsPage'
import { LoginPage } from '@/pages/LoginPage'
import { RegisterPage } from '@/pages/RegisterPage'

function RequireAuth() {
  const { token } = useAuth()
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}

function GuestOnly() {
  const { token } = useAuth()
  if (token) return <Navigate to="/" replace />
  return <Outlet />
}

export const router = createBrowserRouter([
  {
    element: <GuestOnly />,
    children: [
      { path: '/login', element: <LoginPage /> },
      { path: '/register', element: <RegisterPage /> },
    ],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { index: true, element: <GroupsPage /> },
          { path: '/groups/:id', element: <GroupPage /> },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
])
