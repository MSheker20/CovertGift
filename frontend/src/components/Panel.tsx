import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

// Tonal panel: beyazdan bir adım koyu yüzey, kenarsız ve gölgesiz; başlık doğrudan içinde.
export function Panel({
  title,
  count,
  action,
  className,
  children,
}: {
  title: string
  count?: number
  action?: ReactNode
  className?: string
  children: ReactNode
}) {
  return (
    <section className={cn('rounded-[22px] bg-surface p-4 md:p-5', className)}>
      <header className="mb-4 flex min-h-[38px] items-center gap-2">
        <h2 className="text-sm font-semibold">{title}</h2>
        {count !== undefined && (
          <span className="num rounded-full bg-pill-neutral-bg px-2 py-0.5 text-xs text-pill-neutral-ink">{count}</span>
        )}
        {action && <div className="ml-auto">{action}</div>}
      </header>
      {children}
    </section>
  )
}

export function Bar({ className }: { className?: string }) {
  return <span aria-hidden="true" className={cn('block rounded-full bg-active', className)} />
}

export function BlockError({ message, onRetry, retryLabel }: { message: string; onRetry: () => void; retryLabel: string }) {
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 rounded-[14px] bg-raised px-4 py-3.5 text-sm">
      <span className="size-2 shrink-0 rounded-full bg-negative" aria-hidden="true" />
      <span className="flex-1">{message}</span>
      <Button onClick={onRetry} label={retryLabel} />
    </div>
  )
}

function Button({ onClick, label }: { onClick: () => void; label: string }) {
  return (
    <button type="button" onClick={onClick} className="cursor-pointer font-medium underline-offset-4 hover:underline">
      {label}
    </button>
  )
}
