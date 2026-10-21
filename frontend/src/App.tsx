import { Navigate, createBrowserRouter, type RouteObject } from 'react-router'

import { GuestOnly, RequireAuth } from '@/routes/guards'

// Sayfalar rota açıldığında yüklenir (kod bölme): giriş ekranı uygulama kabuğunu,
// grup listesi grup sayfasını beklemez.
export const routes: RouteObject[] = [
  {
    element: <GuestOnly />,
    children: [
      { path: '/login', lazy: () => import('@/pages/LoginPage').then((m) => ({ Component: m.LoginPage })) },
      { path: '/register', lazy: () => import('@/pages/RegisterPage').then((m) => ({ Component: m.RegisterPage })) },
    ],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        lazy: () => import('@/pages/AppShell').then((m) => ({ Component: m.AppShell })),
        children: [
          { index: true, lazy: () => import('@/pages/GroupsPage').then((m) => ({ Component: m.GroupsPage })) },
          { path: '/groups/:id', lazy: () => import('@/pages/GroupPage').then((m) => ({ Component: m.GroupPage })) },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]

export const router = createBrowserRouter(routes)
