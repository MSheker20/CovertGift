import * as React from 'react'
import { Switch as SwitchPrimitive } from 'radix-ui'

import { cn } from '@/lib/utils'

// shadcn/ui Switch, CovertGift token'larına uyarlanmış. Durum yalnızca renkle verilmez: topuz
// konumu değişir ve çağıran taraf yanında durumu yazıyla da söyler.
function Switch({ className, ...props }: React.ComponentProps<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'peer inline-flex h-7 w-12 shrink-0 cursor-pointer items-center rounded-full border-2 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-60 aria-disabled:cursor-not-allowed aria-disabled:opacity-60',
        'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=unchecked]:border-muted data-[state=unchecked]:bg-surface',
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block size-4 rounded-full transition-transform data-[state=checked]:translate-x-[24px] data-[state=checked]:bg-on-primary data-[state=unchecked]:translate-x-[3px] data-[state=unchecked]:bg-muted"
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
