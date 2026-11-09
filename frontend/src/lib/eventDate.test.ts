import { describe, expect, it } from 'vitest'

import type { Group } from '@/api/types'
import { group } from '@/test/server'
import { countdownLabel, daysUntil, eventDateBounds, sortByUpcoming } from './eventDate'

// Yerel takvimde 9 Kasım 2026 öğleden sonra.
const now = new Date(2026, 10, 9, 15, 30)

describe('event date helpers', () => {
  it('counts calendar days in the local calendar', () => {
    expect(daysUntil('2026-11-09', now)).toBe(0)
    expect(daysUntil('2026-11-10', now)).toBe(1)
    expect(daysUntil('2026-11-21', now)).toBe(12)
    expect(daysUntil('2026-11-06', now)).toBe(-3)
    expect(daysUntil('09.11.2026', now)).toBeNull()
  })

  it('labels the countdown', () => {
    expect(countdownLabel('2026-11-09', now)).toBe('Bugün')
    expect(countdownLabel('2026-11-10', now)).toBe('Yarın')
    expect(countdownLabel('2026-11-21', now)).toBe('12 gün kaldı')
    expect(countdownLabel('2026-11-06', now)).toBe('3 gün geçti')
  })

  it('matches the backend date bounds', () => {
    expect(eventDateBounds(now)).toEqual({ min: '2025-11-09', max: '2031-11-09' })
  })

  it('sorts upcoming groups first, then other open groups, then completed ones', () => {
    const g = (id: string, extra: Partial<Group> = {}): Group => ({ ...group, id, ...extra })
    const sorted = sortByUpcoming(
      [
        g('done', { status: 'COMPLETED', event_date: '2026-11-10' }),
        g('no-date'),
        g('far', { event_date: '2026-12-31' }),
        g('passed', { event_date: '2026-11-01' }),
        g('soon', { event_date: '2026-11-12' }),
      ],
      now,
    )
    expect(sorted.map((x) => x.id)).toEqual(['soon', 'far', 'no-date', 'passed', 'done'])
  })
})
