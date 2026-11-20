// Go API ile konuşan tek fetch katmanı. Yanıt sözleşmesi:
// {"success": true, "data": ...} veya {"success": false, "error": "..."}.

const BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '')
const TOKEN_KEY = 'covertgift.token'

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export const tokenStore = {
  get(): string | null {
    try {
      return localStorage.getItem(TOKEN_KEY)
    } catch {
      return null
    }
  },
  set(token: string) {
    try {
      localStorage.setItem(TOKEN_KEY, token)
    } catch {
      // Gizli pencere vb. — oturum bu sekmede bellekte kalmaz, kullanıcı tekrar giriş yapar.
    }
  },
  clear() {
    try {
      localStorage.removeItem(TOKEN_KEY)
    } catch {
      // yok say
    }
  },
}

// Oturum düştüğünde (401) AuthProvider bu olayı dinleyip kullanıcıyı girişe yönlendirir.
export const UNAUTHORIZED_EVENT = 'covertgift:unauthorized'

type Envelope<T> = { success: boolean; data?: T; error?: string }

// Sunucunun uykuda olabileceği istekleri sayar. Azure'da API boştayken sıfıra ölçeklenir;
// uyanması birkaç saniye sürer. Son yanıtın üzerinden bu süreden fazla geçtiyse (ya da hiç
// yanıt alınmadıysa) yeni istek "soğuk" sayılır. Sunucu uyanıkken uzun süren istekler
// (Gemini önerisi, link önizleme) böylece "sunucu uyanıyor" notunu tetiklemez.
const IDLE_AFTER_MS = 4 * 60_000
let lastResponseAt = 0
let coldRequests = 0
const coldListeners = new Set<() => void>()

function setColdRequests(n: number) {
  coldRequests = n
  coldListeners.forEach((l) => l())
}

export const serverActivity = {
  coldRequests: () => coldRequests,
  subscribe(listener: () => void) {
    coldListeners.add(listener)
    return () => {
      coldListeners.delete(listener)
    }
  },
}

// Testler arasında durumu sıfırlar.
export function resetServerActivity() {
  lastResponseAt = 0
  setColdRequests(0)
}

async function tracked(input: string, init: RequestInit): Promise<Response> {
  const cold = Date.now() - lastResponseAt > IDLE_AFTER_MS
  if (cold) setColdRequests(coldRequests + 1)
  try {
    const res = await fetch(input, init)
    lastResponseAt = Date.now()
    return res
  } finally {
    if (cold) setColdRequests(coldRequests - 1)
  }
}

export async function api<T>(
  path: string,
  init: { method?: string; body?: unknown; signal?: AbortSignal } = {},
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`
  // FormData (dosya yükleme) gövdesinde Content-Type'ı sınır değeriyle tarayıcı yazar.
  const form = init.body instanceof FormData ? init.body : null
  if (init.body !== undefined && !form) headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await tracked(`${BASE_URL}/api/v1${path}`, {
      method: init.method ?? 'GET',
      headers,
      body: form ?? (init.body === undefined ? undefined : JSON.stringify(init.body)),
      signal: init.signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network')
  }

  let payload: Envelope<T> | null = null
  try {
    payload = (await res.json()) as Envelope<T>
  } catch {
    payload = null
  }

  if (!res.ok || !payload?.success) {
    if (res.status === 401 && token) {
      tokenStore.clear()
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
    throw new ApiError(res.status, payload?.error ?? res.statusText)
  }
  return payload.data as T
}

// Görsel gibi JSON olmayan bir yanıtı Blob olarak alır. <img> Authorization başlığı
// gönderemediği için korumalı görseller fetch ile okunur; token URL'ye yazılmaz.
// Hata yanıtları standart JSON zarfıyla gelir.
export async function apiBlob(path: string, signal?: AbortSignal): Promise<Blob> {
  const headers: Record<string, string> = {}
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await tracked(`${BASE_URL}/api/v1${path}`, { headers, signal })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network')
  }
  if (!res.ok) {
    let message = res.statusText
    try {
      message = ((await res.json()) as Envelope<unknown>).error ?? message
    } catch {
      // gövde JSON değil; durum metni kalır
    }
    if (res.status === 401 && token) {
      tokenStore.clear()
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
    throw new ApiError(res.status, message)
  }
  return res.blob()
}

// SSE akışı açar. EventSource Authorization başlığı gönderemediği için akış fetch ile
// okunur; böylece token URL'ye yazılmaz.
export function openEventStream(path: string, signal: AbortSignal): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'text/event-stream' }
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`
  return fetch(`${BASE_URL}/api/v1${path}`, { headers, signal, cache: 'no-store' })
}
