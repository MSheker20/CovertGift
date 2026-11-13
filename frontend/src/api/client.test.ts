import { afterEach, describe, expect, it, vi } from 'vitest'

import { api as path, fail, http, ok, server } from '@/test/server'
import { api, apiBlob, ApiError, tokenStore, UNAUTHORIZED_EVENT } from './client'
import { groupsApi } from './endpoints'

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

describe('cover requests', () => {
  afterEach(() => vi.restoreAllMocks())

  it('uploads the image as multipart field "file" and lets the browser set the boundary', async () => {
    tokenStore.set('tok-1')
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(Response.json({ success: true, data: { cover_version: 'v1', width: 1600, height: 1200 } }))
    const image = new Blob(['jpeg'], { type: 'image/jpeg' })

    await expect(groupsApi.uploadCover('g1', image)).resolves.toEqual({ cover_version: 'v1', width: 1600, height: 1200 })

    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/groups/g1/cover')
    expect(init.method).toBe('PUT')
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer tok-1')
    // Content-Type elle yazılmaz: sınır değerini (boundary) tarayıcı ekler.
    expect(headers['Content-Type']).toBeUndefined()
    const body = init.body as FormData
    expect(body).toBeInstanceOf(FormData)
    expect([...body.keys()]).toEqual(['file'])
    const file = body.get('file') as File
    expect(file).toMatchObject({ name: 'cover.jpg', type: 'image/jpeg', size: image.size })
  })

  it('still sends JSON bodies as JSON', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(Response.json({ success: true, data: { cover_admin_only: true } }))
    await groupsApi.setCoverPolicy('g1', true)
    const init = fetchSpy.mock.calls[0][1] as RequestInit
    expect(init.method).toBe('PATCH')
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"cover_admin_only":true}')
  })

  it('reads an image as a blob with the bearer token and never puts the token in the URL', async () => {
    tokenStore.set('tok-1')
    let auth: string | null = null
    let url = ''
    server.use(
      http.get(path('/groups/g1/cover'), ({ request }) => {
        auth = request.headers.get('authorization')
        url = request.url
        return new Response(new Uint8Array([0xff, 0xd8, 0xff]), { headers: { 'Content-Type': 'image/jpeg' } })
      }),
    )
    const blob = await groupsApi.cover('g1', 'v 1')
    expect(blob.size).toBe(3)
    expect(blob.type).toBe('image/jpeg')
    expect(auth).toBe('Bearer tok-1')
    expect(url).toMatch(/\/groups\/g1\/cover\?v=v%201$/)
    expect(url).not.toContain('tok-1')
  })

  it('turns image errors into ApiError with the backend message (Anti-Spoiler 403, missing photo 404)', async () => {
    tokenStore.set('tok-1')
    server.use(
      http.get(path('/groups/g1/cover'), () =>
        fail(403, 'Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.'),
      ),
      http.get(path('/groups/g2/cover'), () => fail(404, 'bu grubun kapak fotoğrafı yok')),
    )
    await expect(apiBlob('/groups/g1/cover')).rejects.toMatchObject({ status: 403, message: expect.stringContaining('sürpriz') })
    await expect(apiBlob('/groups/g2/cover')).rejects.toMatchObject({ status: 404 })
    expect(tokenStore.get()).toBe('tok-1')
  })

  it('logs out on 401 for image requests too', async () => {
    tokenStore.set('expired')
    const listener = vi.fn()
    window.addEventListener(UNAUTHORIZED_EVENT, listener)
    server.use(http.get(path('/groups/g1/cover'), () => fail(401, 'Oturum açmanız gerekiyor')))
    await expect(apiBlob('/groups/g1/cover')).rejects.toMatchObject({ status: 401 })
    expect(tokenStore.get()).toBeNull()
    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener(UNAUTHORIZED_EVENT, listener)
  })
})
