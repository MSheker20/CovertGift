import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router'

import './i18n'
import './index.css'
import { router } from './App'
import { AuthProvider } from '@/auth/AuthProvider'
import { ServerWakingNotice } from '@/components/ServerWakingNotice'
import { Toaster } from '@/components/ui/sonner'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, refetchOnWindowFocus: false },
  },
})

// index.html'deki açılış ekranı, yönlendirici ilk sayfanın kodunu yükleyip çizdiğinde kaldırılır.
function hideSplash() {
  const splash = document.getElementById('splash')
  if (!splash) return
  splash.classList.add('is-done')
  window.setTimeout(() => splash.remove(), 300)
}
if (router.state.initialized) {
  requestAnimationFrame(hideSplash)
} else {
  const unsubscribe = router.subscribe((state) => {
    if (!state.initialized) return
    unsubscribe()
    requestAnimationFrame(hideSplash)
  })
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
        <Toaster />
        <ServerWakingNotice />
      </AuthProvider>
    </QueryClientProvider>
  </StrictMode>,
)
