import { useQuery } from '@tanstack/react-query'

import { ApiError } from './client'
import { queryKeys, usersApi } from './endpoints'
import { useObjectUrl } from './useGroupCover'

// Bir kişinin profil fotoğrafı. <img> Authorization başlığı gönderemediği için görsel fetch ile
// Blob olarak alınır (useGroupCover ile aynı yol). Önbellek anahtarı avatar_version'dır; sürüm yoksa
// istek atılmaz. Sunucu yalnızca ortak gruptakilere verir; 404 olursa baş harfler gösterilir.
export function useUserAvatar(userId: string | undefined, version: string | undefined) {
  const enabled = !!userId && !!version
  const query = useQuery({
    queryKey: queryKeys.avatar(userId ?? '', version ?? ''),
    queryFn: ({ signal }) => usersApi.avatar(userId!, version!, signal),
    enabled,
    staleTime: Infinity,
    gcTime: 5 * 60_000,
    retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
  })
  return useObjectUrl(enabled ? query.data : undefined)
}
