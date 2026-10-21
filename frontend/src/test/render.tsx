import type { ReactElement } from 'react'
import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryRouter } from 'react-router'

import { routes } from '@/App'
import { AuthProvider } from '@/auth/AuthProvider'
import { Toaster } from '@/components/ui/sonner'

function client() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 }, mutations: { retry: false } } })
}

// Uygulamanın gerçek rota ağacını bellek yönlendiricisiyle verilen adreste açar.
export function renderApp(path: string, { token }: { token?: string } = {}) {
  if (token) localStorage.setItem('covertgift.token', token)
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const result = render(
    <QueryClientProvider client={client()}>
      <AuthProvider>
        <RouterProvider router={router} />
        <Toaster />
      </AuthProvider>
    </QueryClientProvider>,
  )
  return { ...result, router }
}

// Tek bir bileşeni sorgu istemcisi ve yönlendirici bağlamıyla çizer.
export function renderWithProviders(ui: ReactElement) {
  const router = createMemoryRouter([{ path: '*', element: ui }])
  return render(
    <QueryClientProvider client={client()}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  )
}
