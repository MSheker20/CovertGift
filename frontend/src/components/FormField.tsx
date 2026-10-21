import type { ReactNode } from 'react'

import { Label } from '@/components/ui/label'

type Props = { id: string; label: string; hint?: string; optional?: string; children: ReactNode }

export function FormField({ id, label, hint, optional, children }: Props) {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>
        {label}
        {optional && <span className="font-normal text-muted">({optional})</span>}
      </Label>
      {children}
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted">
          {hint}
        </p>
      )}
    </div>
  )
}

export function FormError({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <p role="alert" className="rounded-[12px] bg-pill-negative-bg px-3.5 py-2.5 text-sm text-pill-negative-ink">
      {message}
    </p>
  )
}
