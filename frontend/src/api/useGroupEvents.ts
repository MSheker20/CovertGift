import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { openEventStream } from './client'
import { mutationKeys, queryKeys } from './endpoints'

const MAX_BACKOFF_MS = 30_000
// Art arda gelen olaylar (ör. hızlı oylar) tek bir yeniden çekmede birleşir.
const DEBOUNCE_MS = 250

// Grup sayfası açıkken sunucunun "changed" olaylarını dinler (Server-Sent Events) ve
// grup detayını + fikirleri yeniden çeker. Olaylar veri taşımaz; veri her zamanki
// Anti-Spoiler korumalı uçlardan gelir. Bağlantı koparsa artan aralıklarla yeniden
// bağlanır; 401/403/404'te durur. Dönen değer bağlantının o an açık olup olmadığıdır;
// kapalıyken çağıran taraf sık periyodik yenilemeye döner.
export function useGroupEvents(groupId: string, enabled: boolean): boolean {
  const queryClient = useQueryClient()
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (!enabled) return
    const controller = new AbortController()
    let attempt = 0
    let wasConnected = false
    let retry: ReturnType<typeof setTimeout> | undefined
    let debounce: ReturnType<typeof setTimeout> | undefined

    const refresh = () => {
      clearTimeout(debounce)
      debounce = setTimeout(() => {
        // Kendi oyumuz işlenirken iyimser güncellemeyi ezme; işlem bitince zaten yenilenir.
        if (queryClient.isMutating({ mutationKey: mutationKeys.ideas(groupId) }) > 0) return
        void queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId) })
      }, DEBOUNCE_MS)
    }

    const connect = async () => {
      let stop = false
      try {
        const res = await openEventStream(`/groups/${groupId}/events`, controller.signal)
        if (!res.ok || !res.body) {
          stop = [401, 403, 404].includes(res.status)
          throw new Error(`events: HTTP ${res.status}`)
        }
        setConnected(true)
        attempt = 0
        // Bağlantı kopukken kaçan değişiklikler için yeniden bağlanınca bir kez yenile.
        if (wasConnected) refresh()
        wasConnected = true

        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        for (;;) {
          const { value, done } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
          let end: number
          while ((end = buffer.indexOf('\n\n')) >= 0) {
            const frame = buffer.slice(0, end)
            buffer = buffer.slice(end + 2)
            if (frame.split('\n').includes('event: changed')) refresh()
          }
        }
      } catch {
        // Ağ hatası ya da kapanış; aşağıda yeniden bağlanılır.
      }
      setConnected(false)
      if (controller.signal.aborted || stop) return
      // Sunucu akışı ömrü dolunca kapatır (attempt 0 → 1 sn); hatalarda aralık artar.
      const delay = Math.min(MAX_BACKOFF_MS, 1000 * 2 ** attempt)
      attempt++
      retry = setTimeout(() => void connect(), delay)
    }

    void connect()
    return () => {
      controller.abort()
      clearTimeout(retry)
      clearTimeout(debounce)
      setConnected(false)
    }
  }, [groupId, enabled, queryClient])

  return connected
}
