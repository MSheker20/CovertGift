import { openEventStream } from './client'

const MAX_BACKOFF_MS = 30_000

type Handlers = {
  // Sunucu "changed" olayı gönderdiğinde ve yeniden bağlanınca (kaçanlar için) çağrılır.
  onChange: () => void
  onStatus: (connected: boolean) => void
}

// Veri taşımayan bir SSE akışını ("event: changed") dinler ve kopunca yeniden bağlanır.
// Akış fetch ile okunur (EventSource Authorization başlığı gönderemez; token URL'ye yazılmaz).
// Sunucu akışı ömrü dolunca kapatır: 1 sn sonra yeniden bağlanılır; hatalarda aralık artar
// (en çok 30 sn). 401/403/404'te durur. Dönen fonksiyon dinlemeyi bitirir.
export function subscribeToEvents(path: string, { onChange, onStatus }: Handlers): () => void {
  const controller = new AbortController()
  let attempt = 0
  let wasConnected = false
  let retry: ReturnType<typeof setTimeout> | undefined

  const connect = async () => {
    let stop = false
    try {
      const res = await openEventStream(path, controller.signal)
      if (!res.ok || !res.body) {
        stop = [401, 403, 404].includes(res.status)
        throw new Error(`events: HTTP ${res.status}`)
      }
      onStatus(true)
      attempt = 0
      // Bağlantı kopukken kaçan değişiklikler için yeniden bağlanınca bir kez yenile.
      if (wasConnected) onChange()
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
          if (frame.split('\n').includes('event: changed')) onChange()
        }
      }
    } catch {
      // Ağ hatası ya da kapanış; aşağıda yeniden bağlanılır.
    }
    if (controller.signal.aborted) return
    onStatus(false)
    if (stop) return
    const delay = Math.min(MAX_BACKOFF_MS, 1000 * 2 ** attempt)
    attempt++
    retry = setTimeout(() => void connect(), delay)
  }

  void connect()
  return () => {
    controller.abort()
    clearTimeout(retry)
  }
}
