import { act, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderApp } from '@/test/render'
import { api, eventStream, group, groupDetail, http, me, ok, server } from '@/test/server'

describe('scroll position on navigation', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))
  afterEach(() => vi.restoreAllMocks())

  it('opens a new page at the top instead of keeping the previous page’s scroll offset', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const { router } = renderApp('/')
    await screen.findByRole('heading', { level: 1 })
    scrollTo.mockClear()

    // Ana sayfa aşağı kaydırılmışken bir gruba geçilir.
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 640 })
    await act(() => router.navigate(`/groups/${group.id}`))

    expect(await screen.findByRole('heading', { level: 1, name: group.title })).toBeInTheDocument()
    expect(scrollTo).toHaveBeenCalledWith(0, 0)
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 0 })
  })
})
