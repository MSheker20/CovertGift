import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { Group } from '@/api/types'
import { renderApp } from '@/test/render'
import { api, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

const PATH = `/groups/${group.id}`

async function fillNewGroup(email: string) {
  const user = userEvent.setup()
  await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
  const dialog = await screen.findByRole('dialog')
  await user.type(within(dialog).getByLabelText('Grup adı'), 'Kaan’ın 30. yaşı')
  await user.type(within(dialog).getByLabelText('Hediye kimin için?'), 'Kaan')
  if (email) await user.type(within(dialog).getByLabelText(/Hedef kişinin e-postası/), email)
  return { user, dialog }
}

describe('new group target email', () => {
  beforeEach(() => {
    localStorage.setItem('covertgift.token', 'tok')
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
    )
  })

  it('sends the email so the server can link the recipient, then opens the group', async () => {
    let posted: Record<string, unknown> | null = null
    server.use(
      http.post(api('/groups'), async ({ request }) => {
        posted = (await request.json()) as Record<string, unknown>
        return ok({ ...group, target_user_id: 'u-kaan' }, 201)
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    )
    const { router } = renderApp('/')
    const { user, dialog } = await fillNewGroup('  kaan@ornek.com ')
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(posted).toMatchObject({ target_person_name: 'Kaan', target_email: 'kaan@ornek.com' })
  })

  it('stops on an unregistered email and keeps the name-only fallback', async () => {
    const bodies: Record<string, unknown>[] = []
    server.use(
      http.post(api('/groups'), async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        bodies.push(body)
        return body.target_email ? fail(404, 'hedef kişinin e-postası sistemde kayıtlı değil') : ok(group, 201)
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    )
    const { router } = renderApp('/')
    const { user, dialog } = await fillNewGroup('yok@ornek.com')
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))

    const field = within(dialog).getByLabelText(/Hedef kişinin e-postası/)
    expect(await within(dialog).findByText(/Bu e-posta kayıtlı değil/)).toBeInTheDocument()
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(router.state.location.pathname).toBe('/')

    await user.clear(field)
    expect(within(dialog).queryByText(/Bu e-posta kayıtlı değil/)).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(bodies).toHaveLength(2)
    expect(bodies[1]).not.toHaveProperty('target_email')
  })

  it('rejects the user’s own email without calling the API', async () => {
    let called = false
    server.use(
      http.post(api('/groups'), () => {
        called = true
        return ok(group, 201)
      }),
    )
    renderApp('/')
    const { user, dialog } = await fillNewGroup('Deniz@Ornek.com')
    expect(within(dialog).getByText('Kendin için sürpriz grubu kuramazsın.')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))
    expect(called).toBe(false)
  })
})

describe('link target from the admin menu', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  function handlers(current: () => Group) {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([current()])),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ group: current() }))),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    )
  }

  it('links the recipient by email, then shows the linked badge and hides the action', async () => {
    let current: Group = group
    let sent: unknown = null
    handlers(() => current)
    server.use(
      http.patch(api(`/groups/${group.id}/target`), async ({ request }) => {
        sent = await request.json()
        current = { ...group, target_user_id: 'u-kaan' }
        return ok(current)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    expect(await screen.findByText('Kaan için sürpriz', { exact: false })).toBeInTheDocument()
    expect(screen.queryByText('hesabı bağlı')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hedef kişiyi bağla' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('değiştirilemez')
    await user.type(within(dialog).getByLabelText('E-posta'), 'kaan@ornek.com')
    await user.click(within(dialog).getByRole('button', { name: 'Bağla' }))

    expect(await screen.findByText(/Kaan artık bu grubu göremez/)).toBeInTheDocument()
    expect(sent).toEqual({ email: 'kaan@ornek.com' })
    expect(await screen.findByText('hesabı bağlı')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    expect(await screen.findByRole('menuitem', { name: 'Grubu sil' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Hedef kişiyi bağla' })).not.toBeInTheDocument()
  })

  it.each([
    [404, 'hedef kişinin e-postası sistemde kayıtlı değil', 'Bu e-posta kayıtlı değil.'],
    [409, 'grubun üyesi hedef kişi olarak bağlanamaz', 'grubun üyesi hedef kişi olarak bağlanamaz'],
  ])('keeps the dialog open and explains a %i', async (status, serverMessage, shown) => {
    handlers(() => group)
    server.use(http.patch(api(`/groups/${group.id}/target`), () => fail(status, serverMessage)))
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hedef kişiyi bağla' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('E-posta'), 'elif@ornek.com')
    await user.click(within(dialog).getByRole('button', { name: 'Bağla' }))

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(shown)
    expect(within(dialog).getByLabelText('E-posta')).toHaveAttribute('aria-invalid', 'true')
  })

  it('hides the link action from a regular member', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([group])),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ my_role: 'member' }))),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    )
    renderApp(PATH)
    expect(await screen.findByText('Kaan için sürpriz', { exact: false })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Grup işlemleri' })).not.toBeInTheDocument()
  })
})
