import { expect, type APIRequestContext } from '@playwright/test'

// E-postayla ekleme davettir: kişi kabul edene kadar üye olmaz. Testler daveti API'den kabul eder.
export async function acceptGroupInvite(request: APIRequestContext, api: string, token: string, groupId: string) {
  const headers = { Authorization: token.startsWith('Bearer ') ? token : `Bearer ${token}` }
  const list = await request.get(`${api}/me/group-invites`, { headers })
  expect(list.status(), await list.text()).toBe(200)
  const invite = ((await list.json()).data as { id: string; group_id: string }[]).find((i) => i.group_id === groupId)
  expect(invite, `${groupId} için davet yok`).toBeTruthy()
  const accepted = await request.post(`${api}/me/group-invites/${invite!.id}/accept`, { headers })
  expect(accepted.status(), await accepted.text()).toBe(200)
}
