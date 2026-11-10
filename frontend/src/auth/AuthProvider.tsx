import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { ApiError, tokenStore, UNAUTHORIZED_EVENT } from '@/api/client'
import { authApi, queryKeys } from '@/api/endpoints'
import type { UpdateProfileResponse, User } from '@/api/types'

type AuthState = {
  token: string | null
  user: User | undefined
  isLoadingUser: boolean
  sessionExpired: boolean
  login: (email: string, password: string) => Promise<void>
  register: (name: string, email: string, password: string) => Promise<void>
  logout: () => void
  // PATCH /me yanıtını uygular: kullanıcıyı önbelleğe yazar, token geldiyse saklı token'ı değiştirir.
  updateUser: (res: UpdateProfileResponse) => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [token, setToken] = useState<string | null>(() => tokenStore.get())
  const [sessionExpired, setSessionExpired] = useState(false)

  const me = useQuery({
    queryKey: queryKeys.me,
    queryFn: async ({ signal }) => {
      try {
        return await authApi.me(signal)
      } catch (err) {
        // Token geçerli ama kullanıcı artık yok (hesap silinmiş): oturumu 401 gibi kapat.
        if (err instanceof ApiError && err.status === 404) {
          tokenStore.clear()
          window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
        }
        throw err
      }
    },
    retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
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

  const updateUser = useCallback(
    ({ user, token: nextToken }: UpdateProfileResponse) => {
      if (nextToken) {
        tokenStore.set(nextToken)
        setToken(nextToken)
      }
      queryClient.setQueryData(queryKeys.me, user)
      // Üye listeleri ve fikir sahipleri adı/e-postayı gömülü taşır; gruplar yeniden çekilsin.
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    [queryClient],
  )

  const value = useMemo<AuthState>(
    () => ({
      token,
      user: me.data,
      isLoadingUser: token !== null && me.isPending,
      sessionExpired,
      login,
      register,
      logout,
      updateUser,
    }),
    [token, me.data, me.isPending, sessionExpired, login, register, logout, updateUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
