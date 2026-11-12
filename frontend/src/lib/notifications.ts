import type { AppNotification } from '@/api/types'
import i18n from '@/i18n'

import { formatEventDate } from './eventDate'
import { formatMoney } from './format'

// Bildirim metni sunucudan gelmez; tür ve parametrelerden aktif dilde oluşturulur.
// Bilinmeyen bir tür (yeni backend, eski arayüz) genel bir metinle gösterilir.
export function notificationText(n: AppNotification): string {
  const p = n.params ?? {}
  const actor = p.actor_name || i18n.t('notifications.someone')
  const date = p.event_date ? formatEventDate(p.event_date) : ''
  switch (n.type) {
    case 'idea_added':
      return i18n.t('notifications.ideaAdded', { actor, title: p.idea_title ?? '' })
    case 'ai_ideas_added':
      return i18n.t('notifications.aiIdeasAdded', { actor, count: p.count ?? 1 })
    case 'added_to_group':
      return i18n.t('notifications.addedToGroup', { actor })
    case 'member_joined':
      return i18n.t('notifications.memberJoined', { actor, member: p.member_name ?? i18n.t('notifications.someone') })
    case 'winner_selected':
      return i18n.t('notifications.winnerSelected', { actor, count: p.count ?? 1 })
    case 'group_reopened':
      return i18n.t('notifications.groupReopened', { actor })
    case 'event_date_changed':
      return p.event_date
        ? i18n.t('notifications.eventDateChanged', { actor, date })
        : i18n.t('notifications.eventDateRemoved', { actor })
    case 'budget_changed':
      return i18n.t('notifications.budgetChanged', { actor, amount: formatMoney(p.total_budget ?? '') })
    case 'event_reminder': {
      const days = p.days_left ?? 0
      if (days <= 0) return i18n.t('notifications.reminderToday', { date })
      if (days === 1) return i18n.t('notifications.reminderTomorrow', { date })
      return i18n.t('notifications.reminderDays', { count: days, date })
    }
    case 'winner_overdue':
      return i18n.t('notifications.winnerOverdue', { date })
    default:
      return i18n.t('notifications.unknown')
  }
}

// Rozet: 9'dan fazlası "9+" olarak gösterilir (dar barda taşmasın).
export function badgeLabel(count: number): string {
  return count > 9 ? '9+' : String(count)
}
