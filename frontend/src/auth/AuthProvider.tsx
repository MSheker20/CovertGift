import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { tokenStore, UNAUTHORIZED_EVENT } from '@/api/client'
import { authApi, queryKeys } from '@/api/endpoints'
import type { User } from '@/api/types'

type AuthState = {
  token: string | null
  user: User | undefined
  isLoadingUser: boolean
  sessionExpired: boolean
  login: (email: string, password: string) => Promise<void>
  register: (name: string, email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [token, setToken] = useState<string | null>(() => tokenStore.get())
  const [sessionExpired, setSessionExpired] = useState(false)

  const me = useQuery({
    queryKey: queryKeys.me,
    queryFn: ({ signal }) => authApi.me(signal),
    enabled: token !== null,
    staleTime: 5 * 60_000,
  })

  // API katmanı 401 aldığında token'ı siler ve bu olayı yayar.
  useEffect(() => {
    const onUnauthorized = () => {
      setToken(null)
      setSessionExpired(true)
      queryClient.clear()
    }
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [queryClient])

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await authApi.login(email, password)
      queryClient.clear()
      tokenStore.set(res.token)
      queryClient.setQueryData(queryKeys.me, res.user)
      setSessionExpired(false)
      setToken(res.token)
    },
    [queryClient],
  )

  const register = useCallback(
    async (name: string, email: string, password: string) => {
      await authApi.register(name, email, password)
      await login(email, password)
    },
    [login],
  )

  const logout = useCallback(() => {
    tokenStore.clear()
    queryClient.clear()
    setSessionExpired(false)
    setToken(null)
  }, [queryClient])

  const value = useMemo<AuthState>(
    () => ({
      token,
      user: me.data,
      isLoadingUser: token !== null && me.isPending,
      sessionExpired,
      login,
      register,
      logout,
    }),
    [token, me.data, me.isPending, sessionExpired, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
