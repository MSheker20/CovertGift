import type { PoolView } from '@/api/types'
import { daysUntil } from './eventDate'

// MULTI grubun kişi sekmelerinin sırası: tarihi bugün ya da ileride olan açık havuzlar en yakın
// tarihten başlar; ardından tarihsizler, sonra tarihi geçmiş ya da tamamlanmış olanlar (en yenisi
// önce). Kendi (kilitli) havuzum her zaman en sondadır; zaten tarih taşımaz (Anti-Spoiler).
// Eşitlikte sunucunun sırası korunur. Girdi değiştirilmez.
export function orderPools<T extends PoolView>(pools: T[], now: Date = new Date()): T[] {
  const rank = (p: T): [bucket: number, key: number] => {
    if (p.locked) return [3, 0]
    const d = p.event_date ? daysUntil(p.event_date, now) : null
    if (d === null) return [p.status === 'COMPLETED' ? 2 : 1, 0]
    if (p.status === 'COMPLETED' || d < 0) return [2, -d]
    return [0, d]
  }
  return pools
    .map((p, i) => ({ p, i, r: rank(p) }))
    .sort((x, y) => x.r[0] - y.r[0] || x.r[1] - y.r[1] || x.i - y.i)
    .map((x) => x.p)
}
