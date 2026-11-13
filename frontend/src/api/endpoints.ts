import { api, apiBlob } from './client'
import type {
  ContributionResult,
  CoverUploadResult,
  CreateGroupInput,
  CreateIdeaInput,
  Group,
  GroupListItem,
  GroupDetail,
  GroupMember,
  Idea,
  InviteCreated,
  InviteDays,
  InvitePreview,
  InviteStatus,
  LinkPreview,
  LoginResponse,
  NotificationList,
  OpenPool,
  PoolDetail,
  PoolView,
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
  // Yalnızca yönetici. Sunucu metni kırpar; boş değer açıklamayı siler. En çok 1000 karakter.
  updateDescription: (id: string, description: string) =>
    api<Group>(`/groups/${id}/description`, { method: 'PATCH', body: { description } }),
  // null tarihi kaldırır.
  updateEventDate: (id: string, eventDate: string | null) =>
    api<unknown>(`/groups/${id}/event-date`, { method: 'PATCH', body: { event_date: eventDate } }),
  // Bir ya da daha fazla (en çok 20) aktif fikir kazanan olur ve grup tamamlanır.
  selectWinners: (id: string, ideaIds: string[]) =>
    api<unknown>(`/groups/${id}/winner`, { method: 'POST', body: { idea_ids: ideaIds } }),
  reopen: (id: string) => api<unknown>(`/groups/${id}/winner`, { method: 'DELETE' }),
  // Kapak fotoğrafı (JPEG). Hedef kişi ve üye olmayan 403, fotoğraf yoksa 404 alır.
  // Sürüm yalnızca tarayıcı önbelleğini ayırmak içindir; sunucu yok sayar.
  cover: (id: string, version: string, signal?: AbortSignal) =>
    apiBlob(`/groups/${id}/cover?v=${encodeURIComponent(version)}`, signal),
  // Sunucu görseli yeniden işler (4:3, en çok 1600x1200, metadata'sız JPEG).
  uploadCover: (id: string, image: Blob) => {
    const form = new FormData()
    form.append('file', image, 'cover.jpg')
    return api<CoverUploadResult>(`/groups/${id}/cover`, { method: 'PUT', body: form })
  },
  removeCover: (id: string) => api<unknown>(`/groups/${id}/cover`, { method: 'DELETE' }),
  // Yalnızca yönetici: kapağı yalnızca yöneticinin değiştirebilmesini açar/kapatır.
  setCoverPolicy: (id: string, adminOnly: boolean) =>
    api<{ cover_admin_only: boolean }>(`/groups/${id}/cover-policy`, {
      method: 'PATCH',
      body: { cover_admin_only: adminOnly },
    }),
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

// MULTI havuzlar. Hepsi OpenPool ister: kendi havuzum (LockedPool) için istek yapmak tür düzeyinde
// engellenir, çünkü sunucu o istekleri zaten 403 + sürpriz mesajıyla reddeder.
// Yalnızca açık (kilitli olmayan) havuz: LockedPool derleme hatası verir.
export type PoolRef = Pick<OpenPool, 'id' | 'locked'>

const poolPath = (groupId: string, pool: PoolRef) => `/groups/${groupId}/pools/${pool.id}`

export const poolsApi = {
  detail: (groupId: string, pool: PoolRef, signal?: AbortSignal) =>
    api<PoolDetail>(poolPath(groupId, pool), { signal }),
  ideas: (groupId: string, pool: PoolRef, signal?: AbortSignal) =>
    api<Idea[]>(`${poolPath(groupId, pool)}/ideas`, { signal }),
  createIdea: (groupId: string, pool: PoolRef, input: CreateIdeaInput) =>
    api<Idea>(`${poolPath(groupId, pool)}/ideas`, { method: 'POST', body: input }),
  previewLink: (groupId: string, url: string, signal?: AbortSignal) =>
    api<LinkPreview>(`/groups/${groupId}/ideas/preview-link`, { method: 'POST', body: { url }, signal }),
  generateAI: (groupId: string, pool: PoolRef) =>
    api<Idea[]>(`${poolPath(groupId, pool)}/ideas/generate-ai`, { method: 'POST' }),
  addInterest: (groupId: string, pool: PoolRef, interest: string) =>
    api<{ interests: string[] }>(`${poolPath(groupId, pool)}/interests`, { method: 'POST', body: { interest } }),
  removeInterest: (groupId: string, pool: PoolRef, interest: string) =>
    api<{ interests: string[] }>(`${poolPath(groupId, pool)}/interests`, { method: 'DELETE', body: { interest } }),
  updateBudget: (groupId: string, pool: PoolRef, totalBudget: string) =>
    api<unknown>(`${poolPath(groupId, pool)}/budget`, { method: 'PATCH', body: { total_budget: totalBudget } }),
  updateEventDate: (groupId: string, pool: PoolRef, eventDate: string | null) =>
    api<unknown>(`${poolPath(groupId, pool)}/event-date`, { method: 'PATCH', body: { event_date: eventDate } }),
  selectWinners: (groupId: string, pool: PoolRef, ideaIds: string[]) =>
    api<unknown>(`${poolPath(groupId, pool)}/winner`, { method: 'POST', body: { idea_ids: ideaIds } }),
  reopen: (groupId: string, pool: PoolRef) =>
    api<unknown>(`${poolPath(groupId, pool)}/winner`, { method: 'DELETE' }),
  // Yalnızca kendi bayrağım. Havuz tamamlanmışken 409.
  setContribution: (groupId: string, pool: PoolRef, contributing: boolean) =>
    api<ContributionResult>(`${poolPath(groupId, pool)}/contribution`, { method: 'PUT', body: { contributing } }),
}

// Yalnızca grup yöneticisi. userId hediye alacak kişinin hesap kimliğidir.
export const recipientsApi = {
  add: (groupId: string, userId: string, managerUserId?: string) =>
    api<{ pool: PoolView }>(`/groups/${groupId}/recipients`, {
      method: 'POST',
      body: { user_id: userId, ...(managerUserId ? { manager_user_id: managerUserId } : {}) },
    }),
  // Yalnızca yöneticinin kendi havuzu için anlamlıdır (başkası için yalnızca null kabul edilir).
  setManager: (groupId: string, userId: string, managerUserId: string | null) =>
    api<{ pool_id: string }>(`/groups/${groupId}/recipients/${userId}/manager`, {
      method: 'PATCH',
      body: { manager_user_id: managerUserId },
    }),
  // Havuzda fikir varsa (ya da yönetici kendi havuzunu kaldırıyorsa) confirm olmadan 409 döner.
  remove: (groupId: string, userId: string, confirm: boolean) =>
    api<{ message: string }>(`/groups/${groupId}/recipients/${userId}${confirm ? '?confirm=true' : ''}`, {
      method: 'DELETE',
    }),
}

export const notificationsApi = {
  // En yeni 50 bildirim ve okunmamış sayısı. Hedefi olduğum grupların bildirimleri hiç gelmez.
  list: (signal?: AbortSignal) => api<NotificationList>('/notifications', { signal }),
  markRead: (id: string) => api<unknown>(`/notifications/${id}/read`, { method: 'POST' }),
  markAllRead: () => api<unknown>('/notifications/read-all', { method: 'POST' }),
}

export const invitesApi = {
  // Yalnızca yönetici. Token'ı asla döndürmez.
  status: (groupId: string, signal?: AbortSignal) => api<InviteStatus>(`/groups/${groupId}/invite`, { signal }),
  // Yeni link oluşturur; varsa öncekini iptal eder. Ham token yalnızca bu yanıtta gelir.
  create: (groupId: string, days: InviteDays) =>
    api<InviteCreated>(`/groups/${groupId}/invite`, { method: 'POST', body: { expires_in_days: days } }),
  revoke: (groupId: string) => api<unknown>(`/groups/${groupId}/invite`, { method: 'DELETE' }),
  // Giriş yapmış herhangi bir kullanıcı. Geçersiz/süresi dolmuş link 404, hedef kişi 403 alır.
  preview: (token: string, signal?: AbortSignal) =>
    api<InvitePreview>(`/invites/${encodeURIComponent(token)}`, { signal }),
  accept: (token: string) => api<{ group_id: string }>(`/invites/${encodeURIComponent(token)}/accept`, { method: 'POST' }),
}

export const queryKeys = {
  me: ['me'] as const,
  notifications: ['notifications'] as const,
  groups: ['groups'] as const,
  group: (id: string) => ['groups', id] as const,
  invite: (groupId: string) => ['groups', groupId, 'invite'] as const,
  invitePreview: (token: string) => ['invites', token] as const,
  ideas: (groupId: string) => ['groups', groupId, 'ideas'] as const,
  // MULTI havuz sorguları da ['groups', id] altındadır: SSE ve grup geçersiz kılmaları onları da yeniler.
  pool: (groupId: string, poolId: string) => ['groups', groupId, 'pools', poolId] as const,
  poolIdeas: (groupId: string, poolId: string) => ['groups', groupId, 'pools', poolId, 'ideas'] as const,
  // Bilerek ['groups'] altında değil: grup listesi geçersiz kılındığında görseller yeniden
  // indirilmez. Sürüm değişince anahtar değişir ve yeni görsel çekilir.
  cover: (groupId: string, version: string) => ['cover', groupId, version] as const,
}

// Fikir mutasyonlarının ortak anahtarı: biri sürerken periyodik yenileme durur, böylece
// iyimser güncelleme eski bir yanıtla ezilmez.
export const mutationKeys = {
  ideas: (groupId: string) => ['ideas', groupId] as const,
  poolIdeas: (groupId: string, poolId: string) => ['ideas', groupId, 'pool', poolId] as const,
  // Katkı anahtarı: bu grubun tüm havuzlarındaki katkı işlemleri (periyodik yenilemeyi durdurur).
  contribution: (groupId: string) => ['pool-contribution', groupId] as const,
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
