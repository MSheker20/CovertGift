import { act, render, renderHook, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useCountUp, useIntroWindow, useNewIds } from '@/lib/motion'
import { AiThinking } from '@/pages/group/AiThinking'
import { renderApp } from '@/test/render'
import { api, group, groupDetail, http, me, ok, server } from '@/test/server'

describe('useNewIds', () => {
  afterEach(() => vi.useRealTimers())

  it('treats nothing as new on the first load, then flags ids that appear later and clears them', () => {
    vi.useFakeTimers()
    const { result, rerender } = renderHook(({ ids, ready }) => useNewIds(ids, ready, 1000), {
      initialProps: { ids: [] as string[], ready: false },
    })
    rerender({ ids: ['a', 'b'], ready: true })
    expect(result.current.size).toBe(0)

    rerender({ ids: ['a', 'b', 'c'], ready: true })
    expect([...result.current]).toEqual(['c'])

    act(() => vi.advanceTimersByTime(1000))
    expect(result.current.size).toBe(0)

    // Sıra değişikliği ya da çıkarma "yeni" sayılmaz.
    rerender({ ids: ['c', 'a'], ready: true })
    expect(result.current.size).toBe(0)
  })
})

describe('useIntroWindow', () => {
  afterEach(() => vi.useRealTimers())

  it('stays on until the data is ready, then turns off after the window', () => {
    vi.useFakeTimers()
    const { result, rerender } = renderHook(({ ready }) => useIntroWindow(ready, 500), { initialProps: { ready: false } })
    act(() => vi.advanceTimersByTime(2000))
    expect(result.current).toBe(true)
    rerender({ ready: true })
    act(() => vi.advanceTimersByTime(500))
    expect(result.current).toBe(false)
  })
})

describe('GroupLink', () => {
  it('prefetches the group detail when the pointer reaches a group card', async () => {
    localStorage.setItem('covertgift.token', 'tok')
    let detailCalls = 0
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([group])),
      http.get(api(`/groups/${group.id}`), () => {
        detailCalls++
        return ok(groupDetail())
      }),
    )
    renderApp('/')
    const user = userEvent.setup()
    const links = await screen.findAllByRole('link', { name: new RegExp(group.title) })
    expect(detailCalls).toBe(0)
    await user.hover(links[0])
    await waitFor(() => expect(detailCalls).toBe(1))
  })
})

describe('useCountUp', () => {
  it('shows the target straight away when reduced motion is preferred', () => {
    const original = window.matchMedia
    window.matchMedia = ((query: string) => ({ ...original(query), matches: query.includes('reduce') })) as typeof window.matchMedia
    try {
      const { result } = renderHook(() => useCountUp(12))
      expect(result.current).toBe(12)
    } finally {
      window.matchMedia = original
    }
  })

  it('counts up to the target and stops there', async () => {
    const { result } = renderHook(() => useCountUp(5, 50))
    await waitFor(() => expect(result.current).toBe(5))
  })
})

describe('AiThinking', () => {
  afterEach(() => vi.useRealTimers())

  it('announces the long explanation once and rotates the visual step text', () => {
    vi.useFakeTimers()
    render(<AiThinking />)
    expect(screen.getByRole('status')).toHaveTextContent('Gemini ilgi alanlarına ve bütçeye göre ürün arıyor.')
    expect(screen.getByText('İlgi alanlarını inceliyor…')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(3500))
    expect(screen.getByText('Bütçeye uyan ürünlere bakıyor…')).toBeInTheDocument()
  })
})
