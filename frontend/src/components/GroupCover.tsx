import type { ReactNode } from 'react'

import { useGroupCover } from '@/api/useGroupCover'
import { Avatar } from '@/components/Avatar'
import { cn } from '@/lib/utils'

type CoverProps = {
  groupId: string
  version?: string
  // Görselin yanında başlık zaten okunuyorsa boş bırakılır (dekoratif).
  alt: string
  // Kutunun boyutu, oranı ve köşeleri (ör. "aspect-[4/3] w-44 rounded-[16px]").
  className?: string
  // Fotoğraf yoksa ya da yüklenemediyse gösterilir.
  fallback?: ReactNode
  // Sayfa geçişinde büyüyerek taşınacak öğenin adı (yalnızca grup sayfasında, tek öğe).
  transitionName?: string
}

// Sabit oranlı kutuda kapak görseli. Yüklenirken kutu boş yüzey olarak yer tutar (sıçrama olmaz).
export function GroupCoverImage({ groupId, version, alt, className, fallback = null, transitionName }: CoverProps) {
  const { url, isError } = useGroupCover(groupId, version)
  if (!version || isError) return <>{fallback}</>
  return (
    <span
      className={cn('block shrink-0 overflow-hidden bg-surface', className)}
      data-cover={version}
      // GroupLink tıklanan kartın kapağını bu işaretle bulur.
      data-vt="cover"
      style={transitionName ? { viewTransitionName: transitionName } : undefined}
    >
      {url && <img src={url} alt={alt} draggable={false} className="size-full object-cover" />}
    </span>
  )
}

const THUMB = { sm: 'size-7 rounded-[8px]', md: 'size-9 rounded-[10px]' }

// Liste satırı ve grup seçici için küçük kare görsel (4:3'ün ortası). Fotoğraf yoksa ya da
// yüklenirken kişi avatarı gösterilir. Yanında grubun adı yazdığı için dekoratiftir.
export function GroupThumb({
  group,
  size = 'md',
}: {
  // MULTI grupta hedef kişi yoktur (boş ad); avatar grup adından üretilir.
  group: { id: string; cover_version?: string; target_person_name: string; title?: string; mode?: string }
  size?: 'sm' | 'md'
}) {
  const { url } = useGroupCover(group.id, group.cover_version)
  if (!url) return <Avatar name={group.mode === 'MULTI' ? (group.title ?? '') : group.target_person_name} size={size} />
  return <img src={url} alt="" draggable={false} data-cover={group.cover_version} data-vt="cover" className={cn('shrink-0 object-cover', THUMB[size])} />
}
