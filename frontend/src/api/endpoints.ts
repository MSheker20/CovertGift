import { api } from './client'
import type { CreateGroupInput, Group, GroupDetail, LoginResponse, User } from './types'

export const authApi = {
  login: (email: string, password: string) =>
    api<LoginResponse>('/auth/login', { method: 'POST', body: { email, password } }),
  register: (name: string, email: string, password: string) =>
    api<User>('/auth/register', { method: 'POST', body: { name, email, password } }),
  me: (signal?: AbortSignal) => api<User>('/me', { signal }),
}

export const groupsApi = {
  list: (signal?: AbortSignal) => api<Group[]>('/groups', { signal }),
  detail: (id: string, signal?: AbortSignal) => api<GroupDetail>(`/groups/${id}`, { signal }),
  create: (input: CreateGroupInput) => api<Group>('/groups', { method: 'POST', body: input }),
}

export const queryKeys = {
  me: ['me'] as const,
  groups: ['groups'] as const,
  group: (id: string) => ['groups', id] as const,
}
