import { Link, type LinkProps } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'

import { groupsApi, queryKeys } from '@/api/endpoints'

// Grup sayfasına giden bağlantı. İmleç gelince, odaklanınca ya da dokununca grup detayı önceden
// çekilir; geçiş View Transitions ile olur. Veri hazırsa kartın kapağı ve başlığı
// (data-vt="cover" / "title") grup sayfasındaki karşılıklarına büyüyerek gider; değilse sayfa
// yumuşakça değişir. Aynı grup ekranda birden çok kartta olabildiği için ad yalnızca tıklanan
// kartın öğelerine verilir (aynı ad iki öğede olursa tarayıcı geçişi iptal eder).
type Props = Omit<LinkProps, 'to' | 'viewTransition'> & { groupId: string }

let named: HTMLElement[] = []

function nameMorphSource(link: HTMLElement) {
  for (const el of named) el.style.viewTransitionName = ''
  named = []
  for (const [part, name] of [
    ['cover', 'group-cover'],
    ['title', 'group-title'],
  ] as const) {
    const el = link.querySelector<HTMLElement>(`[data-vt="${part}"]`)
    if (el) {
      el.style.viewTransitionName = name
      named.push(el)
    }
  }
}

export function GroupLink({ groupId, onPointerEnter, onFocus, onTouchStart, onClick, ...props }: Props) {
  const queryClient = useQueryClient()
  const prefetch = () =>
    void queryClient.prefetchQuery({
      queryKey: queryKeys.group(groupId),
      queryFn: ({ signal }) => groupsApi.detail(groupId, signal),
      staleTime: 15_000,
    })
  return (
    <Link
      {...props}
      to={`/groups/${groupId}`}
      viewTransition
      onPointerEnter={(e) => {
        prefetch()
        onPointerEnter?.(e)
      }}
      onFocus={(e) => {
        prefetch()
        onFocus?.(e)
      }}
      onTouchStart={(e) => {
        prefetch()
        onTouchStart?.(e)
      }}
      onClick={(e) => {
        nameMorphSource(e.currentTarget)
        onClick?.(e)
      }}
    />
  )
}
