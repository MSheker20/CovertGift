import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'

import { ApiError } from './client'
import { groupsApi, queryKeys } from './endpoints'

// Blob başına tek object URL, kullanan bileşen sayısıyla birlikte tutulur. Son kullanan
// ayrılınca URL serbest bırakılır (URL.revokeObjectURL); bırakma bir sonraki tura ertelenir,
// böylece aynı anda yeniden bağlanan bileşen (ör. StrictMode) geçerli URL'yi kaybetmez.
const urls = new WeakMap<Blob, { url: string; users: number }>()

function entryFor(blob: Blob) {
  let entry = urls.get(blob)
  if (!entry) {
    entry = { url: URL.createObjectURL(blob), users: 0 }
    urls.set(blob, entry)
  }
  return entry
}

function releaseSoon(blob: Blob) {
  setTimeout(() => {
    const entry = urls.get(blob)
    if (entry && entry.users <= 0) {
      URL.revokeObjectURL(entry.url)
      urls.delete(blob)
    }
  }, 0)
}

// Blob için object URL döner; blob değişince ve bileşen kaldırılınca eskisini serbest bırakır.
export function useObjectUrl(blob: Blob | null | undefined): string | undefined {
  const url = blob ? entryFor(blob).url : undefined
  useEffect(() => {
    if (!blob) return
    entryFor(blob).users += 1
    return () => {
      const entry = urls.get(blob)
      if (entry) entry.users -= 1
      releaseSoon(blob)
    }
  }, [blob])
  return url
}

// Grubun kapak fotoğrafı. <img> Authorization başlığı gönderemediği için görsel fetch ile
// Blob olarak alınır ve object URL ile gösterilir. Önbellek anahtarı cover_version'dır: aynı
// sürüm bir kez indirilir, kapak değişince anahtar değişir ve yenisi çekilir. Sürüm yoksa
// (fotoğraf yok) istek atılmaz. Hedef kişi grubu hiç göremediği için bu istek de atılmaz;
// atılsa bile sunucu 403 döner.
export function useGroupCover(groupId: string, version?: string) {
  const query = useQuery({
    queryKey: queryKeys.cover(groupId, version ?? ''),
    queryFn: ({ signal }) => groupsApi.cover(groupId, version!, signal),
    enabled: !!groupId && !!version,
    staleTime: Infinity,
    gcTime: 5 * 60_000,
    retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
  })
  const url = useObjectUrl(version ? query.data : undefined)
  return { url, isError: !!version && query.isError }
}
