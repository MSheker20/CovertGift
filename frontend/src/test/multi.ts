import type { Group, GroupDetail, GroupMember, LockedPool, OpenPool, PoolDetail, PoolView } from '@/api/types'
import { me } from './server'

// MULTI grup ve havuz örnekleri (docs: multi-api-contract). Ben (me) varsayılan olarak yöneticiyim.
export const multiGroup: Group = {
  id: 'g-ofis',
  title: 'Ofis yılbaşı',
  target_person_name: '',
  total_budget: '0',
  creator_id: me.id,
  created_at: '2026-10-21T09:00:00Z',
  status: 'ACTIVE',
  mode: 'MULTI',
}

export const elif = { id: 'u-elif', name: 'Elif Kara' }
export const can = { id: 'u-can', name: 'Can Er' }

const member = (id: string, name: string, role: 'admin' | 'member'): GroupMember => ({
  id: `m-${id}`,
  group_id: multiGroup.id,
  user_id: id,
  role,
  joined_at: multiGroup.created_at,
  user: { id, name, email: `${id}@ornek.com`, created_at: multiGroup.created_at },
})

export const multiMembers = (role: 'admin' | 'member' = 'admin'): GroupMember[] => [
  member(me.id, me.name, role === 'admin' ? 'admin' : 'member'),
  member(elif.id, elif.name, role === 'admin' ? 'member' : 'admin'),
  member(can.id, can.name, 'member'),
]

export const lockedPool = (overrides: Partial<LockedPool> = {}): LockedPool => ({
  id: 'p-me',
  recipient: { id: me.id, name: me.name },
  is_mine: true,
  locked: true,
  ...overrides,
})

export const openPool = (overrides: Partial<OpenPool> = {}): OpenPool => ({
  id: 'p-elif',
  recipient: elif,
  is_mine: false,
  locked: false,
  manager: { id: me.id, name: me.name },
  can_manage: true,
  status: 'ACTIVE',
  event_date: '2031-01-15',
  total_budget: '300',
  per_person_budget: '100',
  no_contributors: false,
  contributor_count: 3,
  my_contribution: true,
  idea_count: 1,
  pending_votes: 0,
  ...overrides,
})

export const poolDetail = (overrides: Partial<PoolDetail> = {}): PoolDetail => ({
  ...openPool(),
  interests: ['Kamp'],
  contributors: [
    { user_id: me.id, name: me.name, contributing: true },
    { user_id: can.id, name: can.name, contributing: true },
    { user_id: 'u-extra', name: 'Ece Su', contributing: true },
  ],
  ...overrides,
})

export const multiDetail = (pools: PoolView[], overrides: Partial<GroupDetail> = {}): GroupDetail => ({
  group: multiGroup,
  member_count: 3,
  per_person_budget: '0',
  my_role: 'admin',
  interests: [],
  members: multiMembers(overrides.my_role ?? 'admin'),
  pools,
  ...overrides,
})
