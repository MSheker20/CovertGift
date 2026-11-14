import { groupsApi, ideasApi, mutationKeys, poolsApi, queryKeys, type PoolRef } from '@/api/endpoints'
import type { CreateIdeaInput } from '@/api/types'

// Fikir listesi, bütçe, ilgi alanı, tarih ve kazanan işlemleri hem SINGLE grupta (/groups/:id/...)
// hem MULTI grubun bir havuzunda (/groups/:id/pools/:poolId/...) aynıdır. Ortak bileşenler
// (GiftPool, AddIdeaDialog, BudgetTile, InterestsTile, EventDateDialog) hangisine konuştuklarını
// bu bağdaştırıcıdan öğrenir; böylece çoğaltılmaz.
//
// poolScope yalnızca OpenPool alır: kendi havuzum (LockedPool) için kapsam kurulamaz, dolayısıyla
// o havuza hiçbir istek atılamaz (Anti-Spoiler).
export type Scope = {
  kind: 'group' | 'pool'
  groupId: string
  // Fikir listesinin sorgu anahtarı ve fikir mutasyonlarının ortak anahtarı.
  ideasKey: readonly unknown[]
  mutationKey: readonly unknown[]
  // Bütçe/ilgi alanlarını taşıyan detay sorgusu (grup detayı ya da havuz detayı).
  detailKey: readonly unknown[]
  listIdeas: (signal?: AbortSignal) => ReturnType<typeof ideasApi.list>
  createIdea: (input: CreateIdeaInput) => ReturnType<typeof ideasApi.create>
  previewLink: (url: string) => ReturnType<typeof ideasApi.previewLink>
  generateAI: () => ReturnType<typeof ideasApi.generateAI>
  addInterest: (interest: string) => ReturnType<typeof groupsApi.addInterest>
  removeInterest: (interest: string) => ReturnType<typeof groupsApi.removeInterest>
  updateBudget: (totalBudget: string) => Promise<unknown>
  updateEventDate: (eventDate: string | null) => Promise<unknown>
  selectWinners: (ideaIds: string[]) => Promise<unknown>
  reopen: () => Promise<unknown>
}

export function groupScope(groupId: string): Scope {
  return {
    kind: 'group',
    groupId,
    ideasKey: queryKeys.ideas(groupId),
    mutationKey: mutationKeys.ideas(groupId),
    detailKey: queryKeys.group(groupId),
    listIdeas: (signal) => ideasApi.list(groupId, signal),
    createIdea: (input) => ideasApi.create(groupId, input),
    previewLink: (url) => ideasApi.previewLink(groupId, url),
    generateAI: () => ideasApi.generateAI(groupId),
    addInterest: (interest) => groupsApi.addInterest(groupId, interest),
    removeInterest: (interest) => groupsApi.removeInterest(groupId, interest),
    updateBudget: (b) => groupsApi.updateBudget(groupId, b),
    updateEventDate: (d) => groupsApi.updateEventDate(groupId, d),
    selectWinners: (ids) => groupsApi.selectWinners(groupId, ids),
    reopen: () => groupsApi.reopen(groupId),
  }
}

export function poolScope(groupId: string, pool: PoolRef): Scope {
  return {
    kind: 'pool',
    groupId,
    ideasKey: queryKeys.poolIdeas(groupId, pool.id),
    mutationKey: mutationKeys.poolIdeas(groupId, pool.id),
    detailKey: queryKeys.pool(groupId, pool.id),
    listIdeas: (signal) => poolsApi.ideas(groupId, pool, signal),
    createIdea: (input) => poolsApi.createIdea(groupId, pool, input),
    // Link önizlemesi hiçbir şey yazmaz ve iki modda da grup düzeyinde çalışır.
    previewLink: (url) => poolsApi.previewLink(groupId, url),
    generateAI: () => poolsApi.generateAI(groupId, pool),
    addInterest: (interest) => poolsApi.addInterest(groupId, pool, interest),
    removeInterest: (interest) => poolsApi.removeInterest(groupId, pool, interest),
    updateBudget: (b) => poolsApi.updateBudget(groupId, pool, b),
    updateEventDate: (d) => poolsApi.updateEventDate(groupId, pool, d),
    selectWinners: (ids) => poolsApi.selectWinners(groupId, pool, ids),
    reopen: () => poolsApi.reopen(groupId, pool),
  }
}

// Havuz sekmeleri ile sekme paneli arasındaki WAI-ARIA bağlantısı için kimlikler.
export const tabId = (poolId: string) => `pool-tab-${poolId}`
export const PANEL_ID = 'pool-panel'
