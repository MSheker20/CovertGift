import * as React from 'react'

import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'h-11 w-full min-w-0 rounded-[12px] border border-control bg-raised px-3.5 text-[15px] text-ink transition-colors outline-none placeholder:text-[#6b6b6b] focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/25 disabled:opacity-60 aria-invalid:border-negative aria-invalid:ring-negative/20 md:h-10 md:text-sm',
        className,
      )}
      {...props}
    />
  )
}

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'min-h-20 w-full rounded-[12px] border border-control bg-raised px-3.5 py-2.5 text-[15px] text-ink outline-none placeholder:text-[#6b6b6b] focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/25 md:text-sm',
        className,
      )}
      {...props}
    />
  )
}

export { Input, Textarea }
