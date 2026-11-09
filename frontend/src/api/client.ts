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

export async function api<T>(
  path: string,
  init: { method?: string; body?: unknown; signal?: AbortSignal } = {},
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`
  if (init.body !== undefined) headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await fetch(`${BASE_URL}/api/v1${path}`, {
      method: init.method ?? 'GET',
      headers,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
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

// SSE akışı açar. EventSource Authorization başlığı gönderemediği için akış fetch ile
// okunur; böylece token URL'ye yazılmaz.
export function openEventStream(path: string, signal: AbortSignal): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'text/event-stream' }
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`
  return fetch(`${BASE_URL}/api/v1${path}`, { headers, signal, cache: 'no-store' })
}
