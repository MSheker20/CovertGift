import { useUserAvatar } from '@/api/useUserAvatar'
import { cn } from '@/lib/utils'
import { initials, personStyle } from '@/lib/person'

type Props = {
  name: string
  seed?: string
  size?: 'xs' | 'sm' | 'md' | 'lg'
  className?: string
  // Profil fotoğrafı: kullanıcı kimliği ve sürüm. Sürüm yoksa, fotoğraf yüklenirken ya da
  // alınamazsa (ör. ortak grup yok, 404) baş harfler gösterilir; kutunun boyutu değişmez.
  userId?: string
  version?: string
}

const SIZES = { xs: 'size-5 text-[9px]', sm: 'size-7 text-[11px]', md: 'size-9 text-xs', lg: 'size-11 text-sm' }

// Dekoratiftir: yanında kişinin adı her zaman yazılıdır.
export function Avatar({ name, seed, size = 'md', className, userId, version }: Props) {
  const url = useUserAvatar(userId, version)
  return (
    <span
      aria-hidden="true"
      style={personStyle(seed ?? name)}
      data-avatar={url ? 'photo' : 'initials'}
      className={cn(
        'avatar inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold',
        SIZES[size],
        className,
      )}
    >
      {url ? <img src={url} alt="" draggable={false} className="size-full object-cover" /> : initials(name)}
    </span>
  )
}
