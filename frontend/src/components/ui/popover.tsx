import * as React from 'react'
import { Popover as PopoverPrimitive } from 'radix-ui'

import { cn } from '@/lib/utils'

// shadcn/ui Popover (kısaltılmış). İçine yazı alanı konabilen yüzen panel; DropdownMenu'nun
// aksine klavye odağı içerikteki form öğelerinde serbestçe dolaşır.

function Popover(props: React.ComponentProps<typeof PopoverPrimitive.Root>) {
  return <PopoverPrimitive.Root data-slot="popover" {...props} />
}

function PopoverTrigger(props: React.ComponentProps<typeof PopoverPrimitive.Trigger>) {
  return <PopoverPrimitive.Trigger data-slot="popover-trigger" {...props} />
}

function PopoverContent({ className, sideOffset = 6, align = 'end', ...props }: React.ComponentProps<typeof PopoverPrimitive.Content>) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        data-slot="popover-content"
        sideOffset={sideOffset}
        align={align}
        className={cn(
          'z-50 rounded-[14px] border border-border bg-raised shadow-[0_8px_24px_rgba(0,0,0,0.12)] outline-none data-[state=open]:animate-[pop-in_120ms_ease-out]',
          className,
        )}
        {...props}
      />
    </PopoverPrimitive.Portal>
  )
}

export { Popover, PopoverContent, PopoverTrigger }
