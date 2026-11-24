import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { CreateGroupInput, Friend, FriendList, MyGroupInvite } from '@/api/types'
import { renderApp } from '@/test/render'
import { api, eventStream, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

const friend = (id: string, name: string): Friend => ({ friendship_id: `fs-${id}`, id, name, since: '2026-11-20T09:00:00Z' })
const ayse = friend('u-ayse', 'Ayşe Demir')
const can = friend('u-can', 'Can Yılmaz')
const list = (overrides: Partial<FriendList> = {}): FriendList => ({ friends: [], incoming: [], outgoing: [], ...overrides })

function authed() {
  server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([])))
}

describe('friends page', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('accepts an incoming request and shows the badge until then', async () => {
    authed()
    let accepted = false
    server.use(
      http.get(api('/friends'), () => ok(accepted ? list({ friends: [ayse] }) : list({ incoming: [ayse] }))),
      http.get(api('/friends/link'), () => ok({ code: 'A'.repeat(22) })),
      http.post(api('/friends/requests/fs-u-ayse/accept'), () => {
        accepted = true
        return ok({ message: 'ok' })
      }),
    )
    const user = userEvent.setup()
    renderApp('/friends')

    expect(await screen.findByRole('button', { name: 'Arkadaşlar, 1 yeni istek' })).toBeInTheDocument()
    expect(screen.getByDisplayValue(`${window.location.origin}/f/${'A'.repeat(22)}`)).toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: 'Kabul et' }))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Arkadaşlar' })).toBeInTheDocument())
    const mine = screen.getByRole('heading', { name: /Arkadaşlarım/ }).closest('section') as HTMLElement
    expect(within(mine).getByText('Ayşe Demir')).toBeInTheDocument()
  })

  it('sends a request by email and explains an unknown email', async () => {
    authed()
    let sent = ''
    server.use(
      http.get(api('/friends/link'), () => ok({ code: 'A'.repeat(22) })),
      http.post(api('/friends/requests'), async ({ request }) => {
        const body = (await request.json()) as { email: string }
        if (body.email === 'yok@ornek.com') return fail(404, 'Bu e-postayla kayıtlı bir kullanıcı yok')
        sent = body.email
        return ok({ status: 'PENDING' }, 201)
      }),
    )
    const user = userEvent.setup()
    renderApp('/friends')

    const input = await screen.findByLabelText('E-postayla')
    await user.type(input, 'yok@ornek.com')
    await user.click(screen.getByRole('button', { name: 'İstek gönder' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Bu e-postayla kayıtlı bir kullanıcı yok.')

    await user.clear(input)
    await user.type(input, 'ayse@ornek.com')
    await user.click(screen.getByRole('button', { name: 'İstek gönder' }))
    await waitFor(() => expect(sent).toBe('ayse@ornek.com'))
    expect(input).toHaveValue('')
  })
})

describe('friends popover', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('accepts a request without leaving the page and links to the full list', async () => {
    authed()
    let accepted = false
    server.use(
      http.get(api('/friends'), () => ok(accepted ? list({ friends: [ayse] }) : list({ friends: [can], incoming: [ayse] }))),
      http.post(api('/friends/requests/fs-u-ayse/accept'), () => {
        accepted = true
        return ok({ message: 'ok' })
      }),
    )
    const user = userEvent.setup()
    const { router } = renderApp('/')

    await user.click(await screen.findByRole('button', { name: 'Arkadaşlar, 1 yeni istek' }))
    const popup = await screen.findByRole('dialog', { name: 'Arkadaşlar' })
    expect(within(popup).getByText('1 arkadaş')).toBeInTheDocument()
    await user.click(within(popup).getByRole('button', { name: 'Kabul et' }))

    await waitFor(() => expect(accepted).toBe(true))
    expect(router.state.location.pathname).toBe('/')
    await waitFor(() => expect(within(popup).queryByText('Gelen istekler')).not.toBeInTheDocument())
    await user.click(within(popup).getByRole('link', { name: 'Tüm arkadaşlar ve istekler' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/friends'))
  })
})

describe('friend link page', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('befriends the link owner and opens the friends page', async () => {
    authed()
    const code = 'B'.repeat(22)
    let accepted = false
    server.use(
      http.get(api(`/friend-links/${code}`), () =>
        ok({ user_id: ayse.id, name: ayse.name, is_self: false, already_friend: false }),
      ),
      http.post(api(`/friend-links/${code}/accept`), () => {
        accepted = true
        return ok({ message: 'ok' })
      }),
      http.get(api('/friends/link'), () => ok({ code: 'A'.repeat(22) })),
    )
    const user = userEvent.setup()
    const { router } = renderApp(`/f/${code}`)

    expect(await screen.findByRole('heading', { name: 'Ayşe Demir seni arkadaş olarak eklemek istiyor' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Arkadaş ol' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/friends'))
    expect(accepted).toBe(true)
  })

  it('shows one message for an unknown or reset link', async () => {
    authed()
    server.use(http.get(api('/friend-links/:code'), () => fail(404, 'arkadaşlık linki geçersiz')))
    renderApp(`/f/${'C'.repeat(22)}`)
    expect(await screen.findByRole('alert')).toHaveTextContent('Bu link geçersiz')
  })
})

describe('new group with friends', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('picks the target from friends, hides them from the invite list and sends member_ids', async () => {
    authed()
    let body: CreateGroupInput | null = null
    server.use(
      http.get(api('/friends'), () => ok(list({ friends: [ayse, can] }))),
      http.post(api('/groups'), async ({ request }) => {
        body = (await request.json()) as CreateGroupInput
        return ok(group, 201)
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const user = userEvent.setup()
    renderApp('/')

    await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
    await user.type(screen.getByLabelText('Grup adı'), 'Ayşe’ye sürpriz')
    await user.selectOptions(await screen.findByLabelText('Kime hediye? (arkadaşların)'), ayse.id)
    // Hedef seçilince isim alanı dolar ve e-posta alanı kalkar.
    expect(screen.getByLabelText('Hediye kimin için?')).toHaveValue('Ayşe')
    expect(screen.queryByLabelText(/Hedef kişinin e-postası/)).not.toBeInTheDocument()

    const picker = screen.getByRole('group', { name: /Kimlerle\?/ })
    expect(within(picker).queryByText('Ayşe Demir')).not.toBeInTheDocument()
    await user.click(within(picker).getByRole('checkbox', { name: 'Can Yılmaz' }))
    await user.click(screen.getByRole('button', { name: 'Grubu kur' }))

    await waitFor(() => expect(body).not.toBeNull())
    expect(body).toMatchObject({ target_user_id: ayse.id, member_ids: [can.id] })
    expect(body!.target_email).toBeUndefined()
  })
})

describe('group invites on the home page', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('joins the group from an invite', async () => {
    authed()
    const invite: MyGroupInvite = {
      id: 'inv-1',
      group_id: group.id,
      group_title: group.title,
      group_mode: 'SINGLE',
      inviter_name: 'Elif Kara',
      member_count: 3,
      created_at: '2026-11-24T09:00:00Z',
    }
    let accepted = false
    server.use(
      http.get(api('/me/group-invites'), () => ok(accepted ? [] : [invite])),
      http.post(api('/me/group-invites/inv-1/accept'), () => {
        accepted = true
        return ok({ group_id: group.id })
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ my_role: 'member' }))),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const user = userEvent.setup()
    const { router } = renderApp('/')

    const section = (await screen.findByRole('heading', { name: 'Bir grup davetin var' })).closest('section') as HTMLElement
    expect(within(section).getByText('Elif Kara davet etti · 3 üye')).toBeInTheDocument()
    await user.click(within(section).getByRole('button', { name: 'Katıl' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/groups/${group.id}`))
    expect(accepted).toBe(true)
  })
})
