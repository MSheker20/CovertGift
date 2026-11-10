import type { GroupListItem } from '@/api/types'
import { daysUntil } from './eventDate'

// Ana ekrandaki "Senden bekleyenler" listesi. Her açık grup en fazla bir görev üretir;
// gruptaki en yüksek öncelikli durum seçilir. Kurallar:
//   urgent: tarihi geçmiş ama tamamlanmamış grup; ya da ≤3 gün kala oy vermediğin fikir var.
//   high:   ≤7 gün kala oy vermediğin fikir var; ya da (yöneticiysen) ≤7 gün kala kazanan seçilmedi.
//   normal: oy vermediğin fikir var, süre baskısı yok.
//   low:    senin kurduğun ve hediye tarihi girilmemiş grup.
export type PriorityLevel = 'urgent' | 'high' | 'normal' | 'low'

export type TaskKind = 'overdue' | 'votes' | 'pickWinner' | 'addDate'

export type PriorityTask = {
  group: GroupListItem
  level: PriorityLevel
  kind: TaskKind
  // Etkinliğe kalan gün (geçmişte negatif); tarih yoksa null.
  days: number | null
  // Yönetici ise "kazananı seç" gibi eylem metinleri gösterilir.
  isAdmin: boolean
}

export const PRIORITY_ORDER: Record<PriorityLevel, number> = { urgent: 0, high: 1, normal: 2, low: 3 }

export function taskForGroup(group: GroupListItem, meId: string | undefined, now: Date = new Date()): PriorityTask | null {
  if (group.status === 'COMPLETED') return null
  const days = group.event_date ? daysUntil(group.event_date, now) : null
  // Yönetici: grubu kuran kişi (listede rol yok; kurucu her zaman admin).
  const isAdmin = meId !== undefined && group.creator_id === meId
  const pending = group.pending_votes > 0
  const task = (level: PriorityLevel, kind: TaskKind): PriorityTask => ({ group, level, kind, days, isAdmin })

  if (days !== null && days < 0) return task('urgent', 'overdue')
  if (days !== null && days <= 3 && pending) return task('urgent', 'votes')
  if (days !== null && days <= 7 && pending) return task('high', 'votes')
  if (days !== null && days <= 7 && isAdmin) return task('high', 'pickWinner')
  if (pending) return task('normal', 'votes')
  if (days === null && isAdmin) return task('low', 'addDate')
  return null
}

// Önceliğe göre (Acil → Düşük), sonra en yakın tarih önce (tarihsizler sonda), sonra grup adı.
export function prioritizedTasks(
  groups: GroupListItem[],
  meId: string | undefined,
  now: Date = new Date(),
  limit = 6,
): PriorityTask[] {
  return groups
    .map((g) => taskForGroup(g, meId, now))
    .filter((t): t is PriorityTask => t !== null)
    .sort(
      (a, b) =>
        PRIORITY_ORDER[a.level] - PRIORITY_ORDER[b.level] ||
        (a.days ?? Number.POSITIVE_INFINITY) - (b.days ?? Number.POSITIVE_INFINITY) ||
        a.group.title.localeCompare(b.group.title),
    )
    .slice(0, limit)
}

// Sıradaki sürpriz: bugün veya sonrası tarihli, en yakın açık grup.
export function nextSurprise<T extends GroupListItem>(groups: T[], now: Date = new Date()): { group: T; days: number } | null {
  let best: { group: T; days: number } | null = null
  for (const g of groups) {
    if (g.status === 'COMPLETED' || !g.event_date) continue
    const days = daysUntil(g.event_date, now)
    if (days === null || days < 0) continue
    if (!best || days < best.days) best = { group: g, days }
  }
  return best
}
