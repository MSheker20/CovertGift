import { describe, expect, it } from 'vitest'

import type { LockedPool, OpenPool } from '@/api/types'
import { poolScope } from './scope'

// Derleme zamanı denetimi: kendi (kilitli) havuzum için kapsam kurulamaz, dolayısıyla o havuza
// istek atılamaz. Bu dosyadaki @ts-expect-error satırı tsc -b'de kaybolursa (hata kalkarsa) derleme kırılır.
describe('poolScope', () => {
  it('accepts an open pool and rejects a locked pool at compile time', () => {
    const open = { id: 'p1', locked: false } as OpenPool
    const locked = { id: 'p2', locked: true } as LockedPool
    expect(poolScope('g', open).kind).toBe('pool')
    // @ts-expect-error LockedPool bir PoolRef değildir (locked: true).
    poolScope('g', locked)
  })
})
