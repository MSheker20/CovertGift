import { cn } from '@/lib/utils'
import { initials, personStyle } from '@/lib/person'

type Props = { name: string; seed?: string; size?: 'xs' | 'sm' | 'md' | 'lg'; className?: string }

const SIZES = { xs: 'size-5 text-[9px]', sm: 'size-7 text-[11px]', md: 'size-9 text-xs', lg: 'size-11 text-sm' }

export function Avatar({ name, seed, size = 'md', className }: Props) {
  return (
    <span
      aria-hidden="true"
      style={personStyle(seed ?? name)}
      className={cn('avatar inline-flex shrink-0 items-center justify-center rounded-full font-semibold', SIZES[size], className)}
    >
      {initials(name)}
    </span>
  )
}
