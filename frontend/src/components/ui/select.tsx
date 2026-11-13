import * as React from 'react'

import { cn } from '@/lib/utils'

// Yerel <select>: klavye ve ekran okuyucu desteği tarayıcıdan gelir. Input ile aynı görünüm.
function Select({ className, ...props }: React.ComponentProps<'select'>) {
  return (
    <select
      data-slot="select"
      className={cn(
        'h-11 w-full min-w-0 cursor-pointer rounded-[12px] border border-control bg-raised px-3 text-[15px] text-ink transition-colors outline-none focus-visible:border-accent focus-visible:ring-2 focus-visible:ring-accent/25 disabled:opacity-60 aria-invalid:border-negative aria-invalid:ring-negative/20 md:h-10 md:text-sm',
        className,
      )}
      {...props}
    />
  )
}

export { Select }
