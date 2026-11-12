import { api } from './client'
import type {
  CreateGroupInput,
  CreateIdeaInput,
  Group,
  GroupListItem,
  GroupDetail,
  GroupMember,
  Idea,
  LinkPreview,
  LoginResponse,
  NotificationList,
  UpdateProfileResponse,
  User,
} from './types'

export const authApi = {
  login: (email: string, password: string) =>
    api<LoginResponse>('/auth/login', { method: 'POST', body: { email, password } }),
  register: (name: string, email: string, password: string) =>
    api<User>('/auth/register', { method: 'POST', body: { name, email, password } }),
  me: (signal?: AbortSignal) => api<User>('/me', { signal }),
  // İsim ve e-posta birlikte gönderilir. Başka kullanıcının e-postası 409 döner.
  updateProfile: (name: string, email: string) =>
    api<UpdateProfileResponse>('/me', { method: 'PATCH', body: { name, email } }),
  // Yanlış mevcut şifre veya eskisiyle aynı yeni şifre 400 döner (401 değil; oturum kapanmaz).
  changePassword: (currentPassword: string, newPassword: string) =>
    api<{ message: string }>('/me/password', {
      method: 'PUT',
      body: { current_password: currentPassword, new_password: newPassword },
    }),
  // Hesabı ve kullanıcının tüm verisini kalıcı olarak siler.
  deleteMe: () => api<{ message: string }>('/me', { method: 'DELETE' }),
}

export const groupsApi = {
  list: (signal?: AbortSignal) => api<GroupListItem[]>('/groups', { signal }),
  detail: (id: string, signal?: AbortSignal) => api<GroupDetail>(`/groups/${id}`, { signal }),
  create: (input: CreateGroupInput) => api<Group>('/groups', { method: 'POST', body: input }),
  addMember: (id: string, email: string) =>
    api<GroupMember>(`/groups/${id}/members`, { method: 'POST', body: { email } }),
  addInterest: (id: string, interest: string) =>
    api<{ interests: string[] }>(`/groups/${id}/interests`, { method: 'POST', body: { interest } }),
  removeInterest: (id: string, interest: string) =>
    api<{ interests: string[] }>(`/groups/${id}/interests`, { method: 'DELETE', body: { interest } }),
  updateBudget: (id: string, totalBudget: string) =>
    api<unknown>(`/groups/${id}/budget`, { method: 'PATCH', body: { total_budget: totalBudget } }),
  linkTarget: (id: string, email: string) =>
    api<Group>(`/groups/${id}/target`, { method: 'PATCH', body: { email } }),
  remove: (id: string) => api<unknown>(`/groups/${id}`, { method: 'DELETE' }),
  // null tarihi kaldırır.
  updateEventDate: (id: string, eventDate: string | null) =>
    api<unknown>(`/groups/${id}/event-date`, { method: 'PATCH', body: { event_date: eventDate } }),
  // Bir ya da daha fazla (en çok 20) aktif fikir kazanan olur ve grup tamamlanır.
  selectWinners: (id: string, ideaIds: string[]) =>
    api<unknown>(`/groups/${id}/winner`, { method: 'POST', body: { idea_ids: ideaIds } }),
  reopen: (id: string) => api<unknown>(`/groups/${id}/winner`, { method: 'DELETE' }),
}

export const ideasApi = {
  list: (groupId: string, signal?: AbortSignal) => api<Idea[]>(`/groups/${groupId}/ideas`, { signal }),
  create: (groupId: string, input: CreateIdeaInput) =>
    api<Idea>(`/groups/${groupId}/ideas`, { method: 'POST', body: input }),
  previewLink: (groupId: string, url: string, signal?: AbortSignal) =>
    api<LinkPreview>(`/groups/${groupId}/ideas/preview-link`, { method: 'POST', body: { url }, signal }),
  generateAI: (groupId: string) => api<Idea[]>(`/groups/${groupId}/ideas/generate-ai`, { method: 'POST' }),
  vote: (ideaId: string, voteType: 1 | -1) =>
    api<unknown>(`/ideas/${ideaId}/vote`, { method: 'POST', body: { vote_type: voteType } }),
  unvote: (ideaId: string) => api<unknown>(`/ideas/${ideaId}/vote`, { method: 'DELETE' }),
  reject: (ideaId: string) => api<unknown>(`/ideas/${ideaId}/reject`, { method: 'POST' }),
  remove: (ideaId: string) => api<unknown>(`/ideas/${ideaId}`, { method: 'DELETE' }),
}

export const notificationsApi = {
  // En yeni 50 bildirim ve okunmamış sayısı. Hedefi olduğum grupların bildirimleri hiç gelmez.
  list: (signal?: AbortSignal) => api<NotificationList>('/notifications', { signal }),
  markRead: (id: string) => api<unknown>(`/notifications/${id}/read`, { method: 'POST' }),
  markAllRead: () => api<unknown>('/notifications/read-all', { method: 'POST' }),
}

export const queryKeys = {
  me: ['me'] as const,
  notifications: ['notifications'] as const,
  groups: ['groups'] as const,
  group: (id: string) => ['groups', id] as const,
  ideas: (groupId: string) => ['groups', groupId, 'ideas'] as const,
}

// Fikir mutasyonlarının ortak anahtarı: biri sürerken periyodik yenileme durur, böylece
// iyimser güncelleme eski bir yanıtla ezilmez.
export const mutationKeys = {
  ideas: (groupId: string) => ['ideas', groupId] as const,
}

// Grup sayfası açıkken değişiklikler SSE ile anında gelir. Bağlantı yoksa (proxy, ağ)
// başkalarının oyları ve yeni fikirler bu aralıkla çekilir; bağlantı varken yalnızca
// seyrek bir güvenlik yenilemesi yapılır. Sekme arka plandayken TanStack Query yenilemeyi durdurur.
export const LIVE_REFRESH_MS = 10_000
export const LIVE_FALLBACK_MS = 60_000

export const liveInterval = (connected: boolean) => (connected ? LIVE_FALLBACK_MS : LIVE_REFRESH_MS)

// Bildirimler SSE ile anında gelir; bağlantı yoksa bu aralıkla çekilir.
export const NOTIFICATIONS_REFRESH_MS = 30_000
export const NOTIFICATIONS_FALLBACK_MS = 5 * 60_000
