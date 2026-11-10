import type { ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'
import { authApi, queryKeys } from '@/api/endpoints'
import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { api, fail, http, me, ok, server } from '@/test/server'

// AuthProvider'ı gerçek sorgu istemcisiyle çizer; istemci testte önbelleği okumak için döner.
function setup(token = 'tok-eski') {
  localStorage.setItem('covertgift.token', token)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{children}</AuthProvider>
    </QueryClientProvider>
  )
  const hook = renderHook(() => useAuth(), { wrapper })
  return { ...hook, queryClient }
}

afterEach(() => localStorage.clear())

describe('profile update', () => {
  it('sends name and email together and stores the new user and token', async () => {
    const updated = { ...me, name: 'Deniz Yıldız', email: 'deniz.yeni@ornek.com' }
    let body: unknown
    let auth: string | null = null
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.patch(api('/me'), async ({ request }) => {
        body = await request.json()
        auth = request.headers.get('Authorization')
        return ok({ user: updated, token: 'tok-yeni' })
      }),
    )
    const { result, queryClient } = setup()
    await waitFor(() => expect(result.current.user).toEqual(me))
    queryClient.setQueryData(queryKeys.groups, [])

    const res = await authApi.updateProfile(updated.name, updated.email)
    act(() => result.current.updateUser(res))

    expect(body).toEqual({ name: 'Deniz Yıldız', email: 'deniz.yeni@ornek.com' })
    expect(auth).toBe('Bearer tok-eski')
    await waitFor(() => expect(result.current.user).toEqual(updated))
    expect(result.current.token).toBe('tok-yeni')
    expect(localStorage.getItem('covertgift.token')).toBe('tok-yeni')
    expect(queryClient.getQueryState(queryKeys.groups)?.isInvalidated).toBe(true)
  })

  it('keeps the stored token when the response has none (email unchanged)', async () => {
    const renamed = { ...me, name: 'Deniz A.' }
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.patch(api('/me'), () => ok({ user: renamed })),
    )
    const { result } = setup()
    await waitFor(() => expect(result.current.user).toEqual(me))

    const res = await authApi.updateProfile(renamed.name, renamed.email)
    act(() => result.current.updateUser(res))

    // Sorgu gözlemcileri bildirimi bir sonraki tick'te yapar.
    await waitFor(() => expect(result.current.user).toEqual(renamed))
    expect(result.current.token).toBe('tok-eski')
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')
  })

  it('surfaces a taken email as a 409 ApiError without touching the session', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.patch(api('/me'), () => fail(409, 'bu e-posta adresi zaten kullanımda')),
    )
    const { result } = setup()
    await waitFor(() => expect(result.current.user).toEqual(me))

    await expect(authApi.updateProfile(me.name, 'elif@ornek.com')).rejects.toMatchObject({ status: 409 })
    expect(result.current.user).toEqual(me)
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')
  })
})

describe('password change', () => {
  it('sends current and new password', async () => {
    let body: unknown
    server.use(
      http.put(api('/me/password'), async ({ request }) => {
        body = await request.json()
        return ok({ message: 'Şifreniz başarıyla değiştirildi' })
      }),
    )
    localStorage.setItem('covertgift.token', 'tok-eski')

    await authApi.changePassword('eski-sifre', 'yeni-sifre')

    expect(body).toEqual({ current_password: 'eski-sifre', new_password: 'yeni-sifre' })
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')
  })

  it('a wrong current password (400) does not end the session', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.put(api('/me/password'), () => fail(400, 'mevcut şifre hatalı')),
    )
    const { result } = setup()
    await waitFor(() => expect(result.current.user).toEqual(me))

    const err = await authApi.changePassword('yanlis', 'yeni-sifre').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(400)
    expect(result.current.token).toBe('tok-eski')
    expect(result.current.sessionExpired).toBe(false)
  })
})
