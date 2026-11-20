import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { api, resetServerActivity } from '@/api/client'
import { ServerWakingNotice } from '@/components/ServerWakingNotice'
import { api as url, http, me, ok, server } from '@/test/server'

// Yanıtı testin elle bıraktığı bir istek: sunucunun uyanmasını taklit eder.
function heldMe() {
  let release!: () => void
  const gate = new Promise<void>((r) => (release = r))
  server.use(
    http.get(url('/me'), async () => {
      await gate
      return ok(me)
    }),
  )
  return () => release()
}

describe('ServerWakingNotice', () => {
  afterEach(() => resetServerActivity())

  it('tells the user the server is waking up when a request to an idle server is slow', async () => {
    const release = heldMe()
    render(<ServerWakingNotice delayMs={30} />)
    let done!: Promise<unknown>
    act(() => {
      done = api('/me')
    })

    const note = await screen.findByText(/Sunucu uyanıyor/)
    expect(note.closest('[role="status"]')).not.toBeNull()
    await act(async () => {
      release()
      await done
    })
    await waitFor(() => expect(screen.queryByText(/Sunucu uyanıyor/)).toBeNull())
  })

  it('stays quiet for a slow request once the server has answered recently', async () => {
    server.use(http.get(url('/me'), () => ok(me)))
    await api('/me')

    const release = heldMe()
    render(<ServerWakingNotice delayMs={30} />)
    let done!: Promise<unknown>
    act(() => {
      done = api('/me')
    })
    await new Promise((r) => setTimeout(r, 120))
    expect(screen.queryByText(/Sunucu uyanıyor/)).toBeNull()
    await act(async () => {
      release()
      await done
    })
  })

  it('does not flash for a fast request to an idle server', async () => {
    server.use(http.get(url('/me'), () => ok(me)))
    render(<ServerWakingNotice delayMs={80} />)
    await act(async () => {
      await api('/me')
    })
    await new Promise((r) => setTimeout(r, 150))
    expect(screen.queryByText(/Sunucu uyanıyor/)).toBeNull()
  })
})
