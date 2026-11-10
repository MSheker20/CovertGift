// Backend modelleriyle eşleşen tipler (backend/internal/models).
// decimal.Decimal alanları JSON'da string olarak gelir.

export type User = {
  id: string
  name: string
  email: string
  avatar_url?: string
  created_at: string
}

export type Group = {
  id: string
  title: string
  description?: string
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
}

export type GroupStatus = 'ACTIVE' | 'COMPLETED'

// GET /groups satırı: grup alanları ve ana ekran sayaçları.
export type GroupListItem = Group & {
  member_count: number
  // Reddedilmemiş fikirler.
  idea_count: number
  // Bana özel: açık gruptaki, benim eklemediğim ve henüz oy vermediğim aktif fikirler.
  pending_votes: number
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
}

export type LoginResponse = { token: string; user: User }

export type CreateGroupInput = {
  title: string
  target_person_name: string
  total_budget: string
  description?: string
  target_interests?: string
  // Sunucu kayıtlı kullanıcıyı bulur; bulamazsa 404 döner.
  target_email?: string
  event_date?: string
}

export type AISource = { title: string; url: string }

export type IdeaStatus = 'ACTIVE' | 'REJECTED' | 'SELECTED'

// GET /groups/:id/ideas — fikir + toplam skor + giriş yapan kullanıcının oyu (1, -1 veya 0).
export type Idea = {
  id: string
  group_id: string
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
