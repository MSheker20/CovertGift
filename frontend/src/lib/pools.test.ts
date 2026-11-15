import { describe, expect, it } from 'vitest'

import { lockedPool, openPool } from '@/test/multi'
import { orderPools } from './pools'

// Yerel takvimde 9 Kasım 2026 öğleden sonra.
const now = new Date(2026, 10, 9, 15, 30)

describe('orderPools', () => {
  it('puts the nearest upcoming date first, then undated, then past or completed, and my own pool last', () => {
    const pools = [
      lockedPool(),
      openPool({ id: 'undated', event_date: undefined }),
      openPool({ id: 'far', event_date: '2027-03-01' }),
      openPool({ id: 'done', event_date: '2027-01-01', status: 'COMPLETED' }),
      openPool({ id: 'past', event_date: '2026-11-01' }),
      openPool({ id: 'today', event_date: '2026-11-09' }),
      openPool({ id: 'soon', event_date: '2026-11-20' }),
    ]
    expect(orderPools(pools, now).map((p) => p.id)).toEqual(['today', 'soon', 'far', 'undated', 'done', 'past', 'p-me'])
  })

  it('keeps the server order for ties and does not mutate the input', () => {
    const pools = [openPool({ id: 'a' }), openPool({ id: 'b' }), openPool({ id: 'c', event_date: undefined })]
    const before = pools.map((p) => p.id)
    expect(orderPools(pools, now).map((p) => p.id)).toEqual(['a', 'b', 'c'])
    expect(pools.map((p) => p.id)).toEqual(before)
  })
})
