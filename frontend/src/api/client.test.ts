import { describe, expect, it, vi } from 'vitest'

import { api as path, fail, http, ok, server } from '@/test/server'
import { api, ApiError, tokenStore, UNAUTHORIZED_EVENT } from './client'

describe('api client', () => {
  it('returns data from the success envelope and sends the bearer token', async () => {
    tokenStore.set('tok-1')
    let auth: string | null = null
    server.use(
      http.get(path('/me'), ({ request }) => {
        auth = request.headers.get('authorization')
        return ok({ id: 'u1' })
      }),
    )
    await expect(api('/me')).resolves.toEqual({ id: 'u1' })
    expect(auth).toBe('Bearer tok-1')
  })

  it('throws ApiError with the backend message and status', async () => {
    server.use(http.post(path('/auth/register'), () => fail(409, 'Bu e-posta zaten kayıtlı')))
    const err = await api('/auth/register', { method: 'POST', body: {} }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 409, message: 'Bu e-posta zaten kayıtlı' })
  })

  it('clears the token and announces the logout on 401', async () => {
    tokenStore.set('expired')
    const listener = vi.fn()
    window.addEventListener(UNAUTHORIZED_EVENT, listener)
    server.use(http.get(path('/groups'), () => fail(401, 'Oturum açmanız gerekiyor')))

    await expect(api('/groups')).rejects.toMatchObject({ status: 401 })
    expect(tokenStore.get()).toBeNull()
    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener(UNAUTHORIZED_EVENT, listener)
  })

  it('reports a network failure as status 0', async () => {
    server.use(http.get(path('/groups'), () => Response.error()))
    await expect(api('/groups')).rejects.toMatchObject({ status: 0, message: 'network' })
  })
})
