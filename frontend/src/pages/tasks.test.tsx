import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { GroupListItem } from '@/api/types'
import { renderApp } from '@/test/render'
import { api, group, http, me, ok, server } from '@/test/server'

// Tarihsiz, oy bekleyen gruplar: her biri "Normal" öncelikli tek bir görev üretir.
function pendingGroups(count: number): GroupListItem[] {
  return Array.from({ length: count }, (_, i) => ({
    ...group,
    id: `g-${i}`,
    title: `Grup ${i + 1}`,
    member_count: 3,
    idea_count: 2,
    pending_votes: 1,
  }))
}

async function tasksPanel(count: number) {
  server.use(http.get(api('/groups'), () => ok(pendingGroups(count))))
  renderApp('/')
  return screen.findByRole('region', { name: 'Senden bekleyenler' })
}

// Panelde başta 3 görev işlenir; 3. satır yalnızca masaüstünde CSS ile gizlenir (JSDOM'da görünür).
describe('Senden bekleyenler paneli', () => {
  beforeEach(() => {
    localStorage.setItem('covertgift.token', 'tok')
    server.use(http.get(api('/me'), () => ok(me)))
  })

  it('shows every task and no button when there are two', async () => {
    const panel = await tasksPanel(2)
    expect(await within(panel).findAllByRole('link')).toHaveLength(2)
    expect(within(panel).queryByRole('button')).toBeNull()
  })

  it('offers the full list once tasks exceed the desktop limit', async () => {
    const panel = await tasksPanel(3)
    expect(await within(panel).findAllByRole('link')).toHaveLength(3)
    expect(within(panel).getByRole('button', { name: 'Tümünü gör (3)' })).toHaveAttribute('aria-expanded', 'false')
  })

  it('expands in place and collapses again', async () => {
    const user = userEvent.setup()
    const panel = await tasksPanel(7)
    expect(await within(panel).findAllByRole('link')).toHaveLength(3)

    const toggle = within(panel).getByRole('button', {
      name: 'Tümünü gör (7)',
    })
    expect(document.getElementById(toggle.getAttribute('aria-controls') ?? '')).not.toBeNull()
    await user.click(toggle)

    expect(within(panel).getAllByRole('link')).toHaveLength(7)
    expect(within(panel).getByRole('link', { name: /Grup 7/ })).toBeVisible()
    const less = within(panel).getByRole('button', { name: 'Daha az göster' })
    expect(less).toHaveAttribute('aria-expanded', 'true')
    expect(less).toHaveFocus()

    await user.click(less)
    expect(within(panel).getAllByRole('link')).toHaveLength(3)
  })

  it('keeps the empty state when nothing is waiting', async () => {
    const panel = await tasksPanel(0)
    expect(await within(panel).findByText('Her şey yolunda')).toBeInTheDocument()
    expect(within(panel).queryByRole('button')).toBeNull()
  })
})
