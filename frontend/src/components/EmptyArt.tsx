import { Bell, Gift, Lightbulb, Users, type LucideIcon } from 'lucide-react'

import { cn } from '@/lib/utils'

const ICONS: Record<'gift' | 'ideas' | 'bell' | 'friends', LucideIcon> = { gift: Gift, ideas: Lightbulb, bell: Bell, friends: Users }

// Boş durumların küçük çizimi: açık kobalt daire, ortada ikon, çevresinde hardal ve kobalt
// konfeti noktaları. Dekoratiftir; yanında her zaman açıklayan metin vardır.
export function EmptyArt({ kind, className }: { kind: keyof typeof ICONS; className?: string }) {
  const Icon = ICONS[kind]
  return (
    <span aria-hidden="true" className={cn('relative inline-flex size-16 shrink-0 items-center justify-center', className)}>
      <span className="absolute inset-1 rounded-full bg-soft" />
      <span className="absolute top-0 right-1 size-2 rotate-12 rounded-[2px] bg-field" />
      <span className="absolute bottom-1.5 left-0 size-1.5 rounded-full bg-accent" />
      <span className="absolute top-3 left-0.5 size-1 rounded-full bg-field" />
      <span className="absolute right-0 bottom-3 h-1 w-2.5 -rotate-12 rounded-full bg-accent/70" />
      <Icon className="relative size-6 text-on-soft" strokeWidth={1.8} />
    </span>
  )
}
