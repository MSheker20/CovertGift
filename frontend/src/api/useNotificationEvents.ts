import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { subscribeToEvents } from './eventStream'
import { queryKeys } from './endpoints'

// Art arda gelen bildirimler tek bir yeniden çekmede birleşir.
const DEBOUNCE_MS = 250

// Oturum açıkken kullanıcının bildirim akışını dinler. Olay veri taşımaz; "changed" gelince
// bildirim listesi (ve yeni bir gruba eklenmiş olabileceğimiz için grup listesi) yeniden
// çekilir. Bağlantı koparsa yeniden bağlanır ve kaçanlar için bir kez yeniler. Dönen değer
// bağlantının açık olup olmadığıdır; kapalıyken çağıran taraf sık periyodik yenilemeye döner.
export function useNotificationEvents(enabled: boolean): boolean {
  const queryClient = useQueryClient()
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (!enabled) return
    let debounce: ReturnType<typeof setTimeout> | undefined

    const refresh = () => {
      clearTimeout(debounce)
      debounce = setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: queryKeys.notifications })
        // exact: grup detayı ve fikirler kendi akışlarıyla yenilenir; burada yalnızca liste.
        void queryClient.invalidateQueries({ queryKey: queryKeys.groups, exact: true })
        // Arkadaşlık olayları zil satırı üretmez; yalnızca bu sinyalle Arkadaşlar rozeti yenilenir.
        void queryClient.invalidateQueries({ queryKey: queryKeys.friends, exact: true })
        void queryClient.invalidateQueries({ queryKey: queryKeys.myGroupInvites })
      }, DEBOUNCE_MS)
    }

    const unsubscribe = subscribeToEvents('/notifications/stream', { onChange: refresh, onStatus: setConnected })
    return () => {
      unsubscribe()
      clearTimeout(debounce)
      setConnected(false)
    }
  }, [enabled, queryClient])

  return connected
}
