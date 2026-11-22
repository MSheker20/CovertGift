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
  // MULTI havuz olaylarında hediye alacak kişinin adı da gelir; metin onu anar. Eski (SINGLE)
  // bildirimlerde alan yoktur ve eski metinler kullanılır.
  const recipient = p.recipient_name
  switch (n.type) {
    case 'idea_added':
      return recipient
        ? i18n.t('notifications.ideaAddedFor', { actor, recipient, title: p.idea_title ?? '' })
        : i18n.t('notifications.ideaAdded', { actor, title: p.idea_title ?? '' })
    case 'ai_ideas_added':
      return recipient
        ? i18n.t('notifications.aiIdeasAddedFor', { actor, recipient, count: p.count ?? 1 })
        : i18n.t('notifications.aiIdeasAdded', { actor, count: p.count ?? 1 })
    case 'added_to_group':
      return i18n.t('notifications.addedToGroup', { actor })
    case 'member_joined':
      return i18n.t('notifications.memberJoined', { actor, member: p.member_name ?? i18n.t('notifications.someone') })
    case 'winner_selected':
      return recipient
        ? i18n.t('notifications.winnerSelectedFor', { actor, recipient, count: p.count ?? 1 })
        : i18n.t('notifications.winnerSelected', { actor, count: p.count ?? 1 })
    case 'group_reopened':
      return recipient
        ? i18n.t('notifications.groupReopenedFor', { actor, recipient })
        : i18n.t('notifications.groupReopened', { actor })
    case 'event_date_changed':
      if (recipient) {
        return p.event_date
          ? i18n.t('notifications.eventDateChangedFor', { actor, recipient, date })
          : i18n.t('notifications.eventDateRemovedFor', { actor, recipient })
      }
      return p.event_date
        ? i18n.t('notifications.eventDateChanged', { actor, date })
        : i18n.t('notifications.eventDateRemoved', { actor })
    case 'budget_changed':
      return recipient
        ? i18n.t('notifications.budgetChangedFor', { actor, recipient, amount: formatMoney(p.total_budget ?? '') })
        : i18n.t('notifications.budgetChanged', { actor, amount: formatMoney(p.total_budget ?? '') })
    case 'event_reminder': {
      const days = p.days_left ?? 0
      if (recipient) {
        if (days <= 0) return i18n.t('notifications.reminderTodayFor', { recipient, date })
        if (days === 1) return i18n.t('notifications.reminderTomorrowFor', { recipient, date })
        return i18n.t('notifications.reminderDaysFor', { recipient, count: days, date })
      }
      if (days <= 0) return i18n.t('notifications.reminderToday', { date })
      if (days === 1) return i18n.t('notifications.reminderTomorrow', { date })
      return i18n.t('notifications.reminderDays', { count: days, date })
    }
    case 'winner_overdue':
      return recipient
        ? i18n.t('notifications.winnerOverdueFor', { recipient, date })
        : i18n.t('notifications.winnerOverdue', { date })
    case 'pool_added':
      return i18n.t('notifications.poolAdded', { actor, recipient: recipient ?? i18n.t('notifications.someone') })
    case 'pool_manager_assigned':
      return i18n.t('notifications.poolManagerAssigned', {
        actor,
        recipient: recipient ?? i18n.t('notifications.someone'),
      })
    default:
      return i18n.t('notifications.unknown')
  }
}

// Bildirime tıklayınca gidilecek adres: havuz olaylarında grup sayfası o havuz sekmesiyle açılır.
export function notificationPath(n: AppNotification): string {
  return `/groups/${n.group_id}${n.pool_id ? `?pool=${encodeURIComponent(n.pool_id)}` : ''}`
}

// Rozet: 9'dan fazlası "9+" olarak gösterilir (dar barda taşmasın).
export function badgeLabel(count: number): string {
  return count > 9 ? '9+' : String(count)
}

// Paneldeki ikonun renk tonu: kobalt fikir, hardal para, altın kazanan, turuncu hatırlatma,
// kırmızı gecikme, yeşil üye. Bilinmeyen türler nötr.
export type NotificationTone = 'idea' | 'money' | 'winner' | 'reminder' | 'overdue' | 'member' | 'neutral'

export function notificationTone(type: string): NotificationTone {
  switch (type) {
    case 'idea_added':
    case 'ai_ideas_added':
    case 'pool_added':
    case 'pool_manager_assigned':
      return 'idea'
    case 'budget_changed':
      return 'money'
    case 'winner_selected':
      return 'winner'
    case 'event_reminder':
    case 'event_date_changed':
      return 'reminder'
    case 'winner_overdue':
      return 'overdue'
    case 'added_to_group':
    case 'member_joined':
      return 'member'
    default:
      return 'neutral'
  }
}
