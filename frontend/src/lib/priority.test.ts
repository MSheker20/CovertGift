import { describe, expect, it } from 'vitest'

import type { GroupListItem } from '@/api/types'
import { group, me } from '@/test/server'
import { nextSurprise, prioritizedTasks, taskForGroup } from './priority'

// Yerel takvimde 10 Kasım 2026 öğleden sonra.
const now = new Date(2026, 10, 10, 15, 30)

const item = (overrides: Partial<GroupListItem> = {}): GroupListItem => ({
  ...group,
  member_count: 3,
  idea_count: 2,
  pending_votes: 0,
  ...overrides,
})
const other = 'u-elif'

describe('taskForGroup', () => {
  it.each([
    ['overdue open group', { event_date: '2026-11-08', creator_id: other }, 'urgent', 'overdue'],
    ['votes due in 3 days', { event_date: '2026-11-13', pending_votes: 1, creator_id: other }, 'urgent', 'votes'],
    ['votes due today', { event_date: '2026-11-10', pending_votes: 2, creator_id: other }, 'urgent', 'votes'],
    ['votes due in 7 days', { event_date: '2026-11-17', pending_votes: 1, creator_id: other }, 'high', 'votes'],
    ['admin, 5 days, no pending votes', { event_date: '2026-11-15' }, 'high', 'pickWinner'],
    ['votes without time pressure', { event_date: '2026-12-20', pending_votes: 3, creator_id: other }, 'normal', 'votes'],
    ['votes, no date', { pending_votes: 1, creator_id: other }, 'normal', 'votes'],
    ['admin, no date', {}, 'low', 'addDate'],
  ] as const)('%s', (_name, overrides, level, kind) => {
    expect(taskForGroup(item(overrides), me.id, now)).toMatchObject({ level, kind })
  })

  it.each([
    ['completed group', { status: 'COMPLETED', event_date: '2026-11-01', pending_votes: 4 }],
    ['member, 5 days, nothing to vote on', { event_date: '2026-11-15', creator_id: other }],
    ['member, no date, nothing to vote on', { creator_id: other }],
    ['admin, far date, nothing to vote on', { event_date: '2027-01-01' }],
  ] as const)('no task: %s', (_name, overrides) => {
    expect(taskForGroup(item(overrides), me.id, now)).toBeNull()
  })

  it('treats nobody as admin without a signed-in user', () => {
    expect(taskForGroup(item({ event_date: '2026-11-15' }), undefined, now)).toBeNull()
  })
})

describe('prioritizedTasks', () => {
  it('sorts by level, then by nearest date, and caps the list', () => {
    const groups = [
      item({ id: 'low', title: 'Low' }),
      item({ id: 'normal', title: 'Normal', pending_votes: 1, creator_id: other }),
      item({ id: 'high-later', title: 'High later', event_date: '2026-11-17', pending_votes: 1 }),
      item({ id: 'urgent', title: 'Urgent', event_date: '2026-11-09' }),
      item({ id: 'high-sooner', title: 'High sooner', event_date: '2026-11-15' }),
      item({ id: 'done', title: 'Done', status: 'COMPLETED' }),
    ]
    expect(prioritizedTasks(groups, me.id, now).map((t) => t.group.id)).toEqual([
      'urgent',
      'high-sooner',
      'high-later',
      'normal',
      'low',
    ])
    expect(prioritizedTasks(groups, me.id, now, 2)).toHaveLength(2)
  })

  it('orders undated tasks of the same level by title', () => {
    const groups = [item({ id: 'b', title: 'Beta' }), item({ id: 'a', title: 'Alfa' })]
    expect(prioritizedTasks(groups, me.id, now).map((t) => t.group.id)).toEqual(['a', 'b'])
  })
})

describe('nextSurprise', () => {
  it('picks the nearest open group dated today or later', () => {
    const groups = [
      item({ id: 'past', event_date: '2026-11-01' }),
      item({ id: 'later', event_date: '2026-12-01' }),
      item({ id: 'done', event_date: '2026-11-11', status: 'COMPLETED' }),
      item({ id: 'soon', event_date: '2026-11-12' }),
      item({ id: 'undated' }),
    ]
    expect(nextSurprise(groups, now)).toMatchObject({ group: { id: 'soon' }, days: 2 })
    expect(nextSurprise([item({ id: 'today', event_date: '2026-11-10' })], now)?.days).toBe(0)
    expect(nextSurprise([item(), item({ event_date: '2026-11-01' })], now)).toBeNull()
  })
})
