import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Group, GroupDetail, Idea, User } from '@/api/types'

// Testler gerçek API yerine bu MSW sunucusuna konuşur. Her test kendi yanıtlarını
// server.use(...) ile ekler; tanımsız bir istek testi hata ile durdurur.
export const server = setupServer()

export const ok = <T>(data: T, status = 200) => HttpResponse.json({ success: true, data }, { status })
export const fail = (status: number, error: string) => HttpResponse.json({ success: false, error }, { status })

export const api = (path: string) => `/api/v1${path}`
export { http }

export const me: User = {
  id: 'u-deniz',
  name: 'Deniz Aksoy',
  email: 'deniz@ornek.com',
  created_at: '2026-10-21T09:00:00Z',
}

export const group: Group = {
  id: 'g-kaan',
  title: 'Kaan’ın 30. yaşı',
  target_person_name: 'Kaan',
  total_budget: '3000',
  creator_id: me.id,
  created_at: '2026-10-21T09:00:00Z',
  status: 'ACTIVE',
}

export const groupDetail = (overrides: Partial<GroupDetail> = {}): GroupDetail => ({
  group,
  member_count: 2,
  per_person_budget: '1500',
  my_role: 'admin',
  interests: ['Kamp', 'Filtre kahve'],
  members: [
    { id: 'm1', group_id: group.id, user_id: me.id, role: 'admin', joined_at: group.created_at, user: me },
    {
      id: 'm2',
      group_id: group.id,
      user_id: 'u-elif',
      role: 'member',
      joined_at: group.created_at,
      user: { id: 'u-elif', name: 'Elif Kara', email: 'elif@ornek.com', created_at: group.created_at },
    },
  ],
  ...overrides,
})

export const idea = (overrides: Partial<Idea> = {}): Idea => ({
  id: 'i-termos',
  group_id: group.id,
  title: 'Stanley termos 1 L',
  estimated_price: '1450.00',
  source_type: 'MANUAL',
  status: 'ACTIVE',
  is_over_budget: false,
  search_query: 'Stanley termos 1 lt',
  created_by: me.id,
  created_at: '2026-10-21T09:00:00Z',
  creator: me,
  score: 0,
  my_vote: 0,
  ...overrides,
})
