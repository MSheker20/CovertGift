import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { subscribeToEvents } from './eventStream'
import { mutationKeys, queryKeys } from './endpoints'

// Art arda gelen olaylar (ör. hızlı oylar) tek bir yeniden çekmede birleşir.
const DEBOUNCE_MS = 250

// Grup sayfası açıkken sunucunun "changed" olaylarını dinler (Server-Sent Events) ve
// grup detayını + fikirleri yeniden çeker. Olaylar veri taşımaz; veri her zamanki
// Anti-Spoiler korumalı uçlardan gelir. Bağlantı koparsa artan aralıklarla yeniden
// bağlanır; 401/403/404'te durur (bkz. subscribeToEvents). Dönen değer bağlantının o an
// açık olup olmadığıdır; kapalıyken çağıran taraf sık periyodik yenilemeye döner.
export function useGroupEvents(groupId: string, enabled: boolean): boolean {
  const queryClient = useQueryClient()
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (!enabled) return
    let debounce: ReturnType<typeof setTimeout> | undefined

    const refresh = () => {
      clearTimeout(debounce)
      debounce = setTimeout(() => {
        // Kendi oyumuz işlenirken iyimser güncellemeyi ezme; işlem bitince zaten yenilenir.
        if (queryClient.isMutating({ mutationKey: mutationKeys.ideas(groupId) }) > 0) return
        void queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId) })
      }, DEBOUNCE_MS)
    }

    const unsubscribe = subscribeToEvents(`/groups/${groupId}/events`, { onChange: refresh, onStatus: setConnected })
    return () => {
      unsubscribe()
      clearTimeout(debounce)
      setConnected(false)
    }
  }, [groupId, enabled, queryClient])

  return connected
}
