import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { subscribeToEvents } from './eventStream'
import { mutationKeys, queryKeys } from './endpoints'

// Art arda gelen olaylar (ör. hızlı oylar) tek bir yeniden çekmede birleşir.
const DEBOUNCE_MS = 250

// Grup sayfası açıkken sunucunun "changed" olaylarını dinler (Server-Sent Events) ve grup
// detayını + fikirleri yeniden çeker. Havuz (MULTI) sorguları ['groups', id] altındadır; aynı
// geçersiz kılma onları da yeniler. Görebildiğim havuz kümesi değişince sunucu "changed" yazıp
// akışı kapatır; yeniden bağlanma yeni kümeyle devam eder. Olaylar veri taşımaz; veri her zamanki
// Anti-Spoiler korumalı uçlardan gelir. Bağlantı koparsa artan aralıklarla yeniden bağlanır;
// 401/403/404'te durur (bkz. subscribeToEvents). Dönen değer bağlantının açık olup olmadığıdır;
// kapalıyken çağıran taraf sık periyodik yenilemeye döner.
export function useGroupEvents(groupId: string, enabled: boolean): boolean {
  const queryClient = useQueryClient()
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (!enabled) return
    let debounce: ReturnType<typeof setTimeout> | undefined
    // Kendi fikir/oy/katkı işlemim sürerken iyimser güncellemeyi ezmemek için yenileme ertelenir;
    // olay kaybolmaz: işlem bitince bekleyen yenileme yapılır.
    let deferred = false
    const busy = () =>
      queryClient.isMutating({ mutationKey: mutationKeys.ideas(groupId) }) +
        queryClient.isMutating({ mutationKey: mutationKeys.contribution(groupId) }) >
      0
    const invalidate = () => void queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId) })

    const refresh = () => {
      clearTimeout(debounce)
      debounce = setTimeout(() => {
        if (busy()) {
          deferred = true
          return
        }
        invalidate()
      }, DEBOUNCE_MS)
    }
    const stopWatching = queryClient.getMutationCache().subscribe(() => {
      if (deferred && !busy()) {
        deferred = false
        invalidate()
      }
    })

    const unsubscribe = subscribeToEvents(`/groups/${groupId}/events`, { onChange: refresh, onStatus: setConnected })
    return () => {
      unsubscribe()
      stopWatching()
      clearTimeout(debounce)
      setConnected(false)
    }
  }, [groupId, enabled, queryClient])

  return connected
}
