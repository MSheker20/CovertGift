// Backend modelleriyle eşleşen tipler (backend/internal/models).
// decimal.Decimal alanları JSON'da string olarak gelir.

export type User = {
  id: string
  name: string
  email: string
  // Profil fotoğrafının sürümü; fotoğraf yoksa yok. Görsel GET /users/:id/avatar ile alınır.
  avatar_version?: string
  created_at: string
}

// SINGLE: tek hedef kişi (eski davranış). MULTI: her hediye alacak kişi için ayrı havuz. Kurulurken
// seçilir, değişmez. Alan eski yanıtlarda yoksa SINGLE sayılır.
export type GroupMode = 'SINGLE' | 'MULTI'

export type Group = {
  id: string
  title: string
  description?: string
  mode?: GroupMode
  // MULTI grupta boştur.
  target_person_name: string
  target_interests?: string
  target_user_id?: string
  total_budget: string
  creator_id: string
  created_at: string
  creator?: User
  // "YYYY-MM-DD"; hediyenin verileceği gün (isteğe bağlı).
  event_date?: string
  // COMPLETED: admin kazananı seçti; fikir ve oy işlemleri kapalı.
  status: GroupStatus
  completed_at?: string
  // Kapak fotoğrafının sürümü; fotoğraf yoksa alan gelmez. Görselin kendisi JSON'da yoktur,
  // GET /groups/:id/cover ile alınır; bu değer önbellek anahtarıdır.
  cover_version?: string
  // true: kapağı yalnızca yönetici ekler/değiştirir/kaldırır. false: tüm üyeler.
  cover_admin_only?: boolean
}

// PUT /groups/:id/cover yanıtı.
export type CoverUploadResult = { cover_version: string; width: number; height: number }

export type AvatarUploadResult = { avatar_version: string; width: number; height: number }

export type GroupStatus = 'ACTIVE' | 'COMPLETED'

// GET /groups satırı: grup alanları ve ana ekran sayaçları.
export type GroupListItem = Group & {
  member_count: number
  // Reddedilmemiş fikirler.
  idea_count: number
  // Bana özel: açık gruptaki, benim eklemediğim ve henüz oy vermediğim aktif fikirler.
  pending_votes: number
  // Yalnızca MULTI: hediye alacak kişilerin adları (havuz kurulma sırasıyla; ben de olabilirim).
  recipients?: AccountSummary[]
  // Yalnızca MULTI: görebildiğim aktif havuzların en yakın tarihi.
  next_event_date?: string
  // Yalnızca MULTI ve next_event_date varken: o tarihi taşıyan havuzların alıcıları (aynı gün birden
  // çok kişi olabilir). Kendi havuzum asla yer almaz (Anti-Spoiler).
  next_event_recipients?: AccountSummary[]
}

export type GroupMember = {
  id: string
  group_id: string
  user_id: string
  role: 'admin' | 'member'
  joined_at: string
  user?: User
}

export type GroupDetail = {
  group: Group
  member_count: number
  per_person_budget: string
  members?: GroupMember[]
  my_role: 'admin' | 'member'
  interests: string[]
  // Bağlı hedef hesabın yalnızca kimliği ve adı (e-posta yok). Hedef bağlı değilse ya da hesap
  // yüklenemediyse gelmez; arayüz o zaman target_person_name'i kullanır. Grup listesinde yoktur.
  target_account?: AccountSummary
  // Yalnızca MULTI: havuz kartları; kendi havuzum LockedPool olarak gelir.
  pools?: PoolView[]
}

// avatar_version yalnızca fotoğrafını görebildiğim kişilerde (ortak grup üyeleri) gelir.
export type AccountSummary = { id: string; name: string; avatar_version?: string }

export type LoginResponse = { token: string; user: User }

// PATCH /me yanıtı. JWT e-posta taşıdığı için token yalnızca e-posta değiştiğinde gelir;
// geldiğinde saklı token bununla değiştirilmelidir.
export type UpdateProfileResponse = { user: User; token?: string }

export type CreateGroupInput = {
  title: string
  // MULTI grupta hedef, bütçe, ilgi alanı ve tarih gönderilmez (400); SINGLE'da mode yoksa SINGLE'dır.
  mode?: GroupMode
  target_person_name?: string
  total_budget?: string
  description?: string
  target_interests?: string
  // Sunucu kayıtlı kullanıcıyı bulur; bulamazsa 404 döner.
  target_email?: string
  // Arkadaş listesinden seçilen hedef (target_email ile birlikte gönderilmez).
  target_user_id?: string
  // Gruba davet edilecek arkadaşlar (en fazla 30). Kabul edene kadar üye sayılmazlar.
  member_ids?: string[]
  event_date?: string
}

export type AISource = { title: string; url: string }

export type IdeaStatus = 'ACTIVE' | 'REJECTED' | 'SELECTED'

// GET /groups/:id/ideas — fikir + toplam skor + giriş yapan kullanıcının oyu (1, -1 veya 0).
export type Idea = {
  id: string
  group_id: string
  // Yalnızca MULTI havuz fikirlerinde.
  pool_id?: string
  title: string
  description?: string
  estimated_price: string
  source_type: 'AI' | 'MANUAL'
  status: IdeaStatus
  is_over_budget: boolean
  search_query: string
  image_url?: string
  product_url?: string
  ai_sources?: AISource[]
  ai_search_suggestions?: string
  created_by: string
  created_at: string
  creator?: User
  score: number
  my_vote: -1 | 0 | 1
}

export type LinkPreview = {
  title: string
  description: string
  image_url: string
  price: string
  currency?: string
  final_url: string
  search_query: string
  source: string
  partial: boolean
  warning?: string
}

export type CreateIdeaInput = {
  title: string
  description?: string
  estimated_price: string
  search_query?: string
  product_url?: string
  image_url?: string
}

// Bildirim türleri (backend/internal/models/notification.go). Metin istemcide type + params
// ile oluşturulur (src/lib/notifications.ts).
export type NotificationType =
  | 'idea_added'
  | 'ai_ideas_added'
  | 'added_to_group'
  | 'member_joined'
  | 'winner_selected'
  | 'group_reopened'
  | 'event_date_changed'
  | 'budget_changed'
  | 'event_reminder'
  | 'winner_overdue'
  | 'pool_added'
  | 'pool_manager_assigned'
  | 'group_invite'

export type NotificationParams = {
  // İşlemi yapan üyenin adı (hatırlatmalarda yok).
  actor_name?: string
  idea_title?: string
  member_name?: string
  count?: number
  // "YYYY-MM-DD"; tarih kaldırıldıysa null.
  event_date?: string | null
  total_budget?: string
  // Hatırlatma üretildiği anda kalan gün (0: bugün, 1: yarın).
  days_left?: number
  // Havuz bildirimlerinde hediye alacak kişinin adı.
  recipient_name?: string
}

export type AppNotification = {
  id: string
  group_id: string
  group_title: string
  // Yalnızca MULTI havuz olaylarında.
  pool_id?: string
  // Bilinmeyen (yeni) türler genel bir metinle gösterilir.
  type: NotificationType | (string & {})
  params: NotificationParams
  read_at?: string
  created_at: string
}

// GET /notifications
export type NotificationList = {
  items: AppNotification[]
  unread_count: number
}

// Davet linki. Ham token yalnızca oluşturma yanıtında bir kez gelir; durum yanıtında asla yoktur.
export type InviteDays = 1 | 7 | 30
export type InviteStatus = { active: boolean; expires_at?: string; created_at?: string }
export type InviteCreated = { token: string; expires_at: string }
export type InvitePreview = {
  // Yalnızca already_member true iken gelir; katılınca accept yanıtı kimliği verir.
  group_id?: string
  group_title: string
  inviter_name: string
  member_count: number
  already_member: boolean
}

// MULTI havuzlar (docs: multi-api-contract). Kendi havuzum yalnızca 4 alanlı kilitli karttır;
// arayüz o havuz için hiçbir /pools/:id/... isteği yapmaz.
// manager yalnızca grup yöneticisinin kendi havuzunda gelir (sorumluyu kendisi atar); diğer
// alıcıların kilitli kartında alan hiç yoktur.
export type LockedPool = { id: string; recipient: AccountSummary; is_mine: true; locked: true; manager?: AccountSummary }

export type OpenPool = {
  id: string
  recipient: AccountSummary
  is_mine: false
  locked: false
  // Etkin yönetici; yoksa (admin'in kendi havuzu ve sorumlu hesabı silinmiş) null.
  manager: AccountSummary | null
  // Bütçe, tarih, kazanan, veto gibi yönetici yetkileri bende mi (havuz yöneticisi ya da admin).
  can_manage: boolean
  status: GroupStatus
  completed_at?: string
  event_date?: string
  total_budget: string
  // Katılan yoksa null.
  per_person_budget: string | null
  no_contributors: boolean
  contributor_count: number
  my_contribution: boolean
  idea_count: number
  pending_votes: number
}

export type PoolView = OpenPool | LockedPool

export type Contributor = { user_id: string; name: string; contributing: boolean; avatar_version?: string }

export type PoolDetail = OpenPool & { interests: string[]; contributors: Contributor[] }

// PUT /groups/:id/pools/:poolId/contribution yanıtı.
export type ContributionResult = {
  contributing: boolean
  contributor_count: number
  per_person_budget: string | null
  no_contributors: boolean
}

// Arkadaşlar. E-posta hiçbir zaman gelmez.
export type Friend = {
  // İstek kabul/ret/geri çekme uçlarında kullanılır.
  friendship_id: string
  id: string
  name: string
  avatar_version?: string
  since: string
  accepted_at?: string
}

// GET /friends
export type FriendList = { friends: Friend[]; incoming: Friend[]; outgoing: Friend[] }

export type FriendRequestResult = { status: 'PENDING' | 'ACCEPTED' }
export type FriendLink = { code: string }
export type FriendLinkPreview = {
  user_id: string
  name: string
  avatar_version?: string
  is_self: boolean
  already_friend: boolean
}

// Kullanıcıya gelen grup daveti. Hedefi olduğum grupların davetleri hiç gelmez.
export type MyGroupInvite = {
  id: string
  group_id: string
  group_title: string
  group_mode: GroupMode
  inviter_name: string
  member_count: number
  created_at: string
}

// Grubun bekleyen davetlerinden biri (grup sayfası).
export type PendingGroupInvite = {
  user_id: string
  name: string
  avatar_version?: string
  inviter_name: string
  created_at: string
}

// POST /groups/:id/members yanıtı: e-postayla gönderilen davet.
export type InvitedMember = { user_id: string; name: string; status: 'INVITED' }
