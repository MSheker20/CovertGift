import type { Group } from '@/api/types'
import i18n from '@/i18n'

// Hediye tarihi "YYYY-MM-DD" takvim günüdür; saat dilimi taşımaz. Hesaplar kullanıcının
// yerel takvim gününe göre yapılır, böylece gece yarısı kayması olmaz.

function parts(date: string): [number, number, number] | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date)
  return m ? [Number(m[1]), Number(m[2]) - 1, Number(m[3])] : null
}

// Bugünden hedef güne kalan gün; geçmiş tarihlerde negatif.
export function daysUntil(date: string, now: Date = new Date()): number | null {
  const p = parts(date)
  if (!p) return null
  const target = Date.UTC(p[0], p[1], p[2])
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((target - today) / 86_400_000)
}

export function formatEventDate(date: string): string {
  const p = parts(date)
  if (!p) return '—'
  return new Intl.DateTimeFormat(i18n.language === 'en' ? 'en-GB' : 'tr-TR', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(p[0], p[1], p[2])))
}

// "12 gün kaldı", "Yarın", "Bugün", "3 gün geçti".
export function countdownLabel(date: string, now: Date = new Date()): string | null {
  const days = daysUntil(date, now)
  if (days === null) return null
  if (days === 0) return i18n.t('countdown.today')
  if (days === 1) return i18n.t('countdown.tomorrow')
  if (days > 1) return i18n.t('countdown.left', { count: days })
  return i18n.t('countdown.passed', { count: -days })
}

// <input type="date"> için yerel "YYYY-MM-DD".
export function isoDay(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// Backend kuralıyla aynı aralık: bugünden en fazla 1 yıl önce, en fazla 5 yıl sonra.
export function eventDateBounds(now: Date = new Date()) {
  const min = new Date(now.getFullYear() - 1, now.getMonth(), now.getDate())
  const max = new Date(now.getFullYear() + 5, now.getMonth(), now.getDate())
  return { min: isoDay(min), max: isoDay(max) }
}

// Yaklaşan tarihi olan açık gruplar en üstte (en yakın önce); ardından diğer açık gruplar,
// en sonda tamamlananlar. Grup içindeki sıra sunucunun sırasıdır.
export function sortByUpcoming<T extends Group>(groups: T[], now: Date = new Date()): T[] {
  const bucket = (g: Group) => {
    if (g.status === 'COMPLETED') return 2
    const d = g.event_date ? daysUntil(g.event_date, now) : null
    return d !== null && d >= 0 ? 0 : 1
  }
  return groups
    .map((g, i) => ({ g, i, b: bucket(g), d: g.event_date ? (daysUntil(g.event_date, now) ?? 0) : 0 }))
    .sort((x, y) => x.b - y.b || (x.b === 0 ? x.d - y.d : 0) || x.i - y.i)
    .map((x) => x.g)
}
