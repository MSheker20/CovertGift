import { useQueryClient } from '@tanstack/react-query'

import { queryKeys } from '@/api/endpoints'

// Arkadaşlık linkinin tam adresi. Kod yalnızca yolda taşınır.
export function friendLinkUrl(code: string): string {
  return `${window.location.origin}/f/${encodeURIComponent(code)}`
}

// Arkadaş listesini yeniden çeker (sayfa, popup ve rozet aynı sorguyu paylaşır).
export function useRefreshFriends() {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: queryKeys.friends, exact: true })
}
