import { cn } from '@/lib/utils'

// Ürünün işareti: kurdeleli kutu. Vurgu renginde, isimle birlikte kullanılır.
export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" className={cn('size-7 text-accent', className)}>
      <rect x="3" y="11" width="26" height="18" rx="5" fill="currentColor" />
      <rect x="1.5" y="7" width="29" height="7" rx="3.5" fill="currentColor" />
      <rect x="14" y="7" width="4" height="22" fill="#fff" />
      <path d="M16 7c-2-5-8-5-7-1.5C9.6 7.4 14 7 16 7Zm0 0c2-5 8-5 7-1.5C22.4 7.4 18 7 16 7Z" fill="currentColor" />
    </svg>
  )
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2.5 text-[17px] font-semibold tracking-[-0.01em]', className)}>
      <LogoMark />
      CovertGift
    </span>
  )
}
