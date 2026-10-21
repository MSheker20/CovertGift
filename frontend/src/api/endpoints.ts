import { api } from './client'
import type {
  CreateGroupInput,
  CreateIdeaInput,
  Group,
  GroupDetail,
  GroupMember,
  Idea,
  LinkPreview,
  LoginResponse,
  User,
} from './types'

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
  addMember: (id: string, email: string) =>
    api<GroupMember>(`/groups/${id}/members`, { method: 'POST', body: { email } }),
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
  reject: (ideaId: string) => api<unknown>(`/ideas/${ideaId}/reject`, { method: 'POST' }),
  remove: (ideaId: string) => api<unknown>(`/ideas/${ideaId}`, { method: 'DELETE' }),
}

export const queryKeys = {
  me: ['me'] as const,
  groups: ['groups'] as const,
  group: (id: string) => ['groups', id] as const,
  ideas: (groupId: string) => ['groups', groupId, 'ideas'] as const,
}
