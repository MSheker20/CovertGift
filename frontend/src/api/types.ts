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
}
