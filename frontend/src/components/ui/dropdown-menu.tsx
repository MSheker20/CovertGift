import * as React from 'react'
import { DropdownMenu as DropdownMenuPrimitive } from 'radix-ui'
import { CheckIcon } from 'lucide-react'

import { cn } from '@/lib/utils'

// shadcn/ui DropdownMenu (kısaltılmış). Menü yüzdüğü için gölge taşır.

function DropdownMenu(props: React.ComponentProps<typeof DropdownMenuPrimitive.Root>) {
  return <DropdownMenuPrimitive.Root data-slot="dropdown-menu" {...props} />
}

function DropdownMenuTrigger(props: React.ComponentProps<typeof DropdownMenuPrimitive.Trigger>) {
  return <DropdownMenuPrimitive.Trigger data-slot="dropdown-menu-trigger" {...props} />
}

function DropdownMenuContent({
  className,
  sideOffset = 6,
  ...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        className={cn(
          'z-50 min-w-44 rounded-[14px] border border-border bg-raised p-1 shadow-[0_8px_24px_rgba(0,0,0,0.12)] data-[state=open]:animate-[pop-in_120ms_ease-out]',
          className,
        )}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  )
}

function DropdownMenuItem({
  className,
  variant = 'default',
  ...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Item> & { variant?: 'default' | 'destructive' }) {
  return (
    <DropdownMenuPrimitive.Item
      data-slot="dropdown-menu-item"
      className={cn(
        'flex h-10 cursor-pointer items-center gap-2.5 rounded-[10px] px-3 text-sm outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-surface [&_svg]:size-4 [&_svg]:text-muted',
        variant === 'destructive' && 'text-negative [&_svg]:text-negative',
        className,
      )}
      {...props}
    />
  )
}

// Açık/kapalı ayar satırı (menuitemcheckbox). Sağda işaret kutusu; metin gerekirse alt satıra iner.
function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.CheckboxItem>) {
  return (
    <DropdownMenuPrimitive.CheckboxItem
      data-slot="dropdown-menu-checkbox-item"
      className={cn(
        'group/check flex min-h-10 cursor-pointer items-center gap-2.5 rounded-[10px] px-3 py-2 text-sm outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-surface',
        className,
      )}
      {...props}
    >
      <span className="min-w-0 flex-1">{children}</span>
      <span
        aria-hidden
        className="inline-flex size-[18px] shrink-0 items-center justify-center rounded-[6px] border border-control text-on-primary group-data-[state=checked]/check:border-primary group-data-[state=checked]/check:bg-primary"
      >
        <DropdownMenuPrimitive.ItemIndicator>
          <CheckIcon className="size-3.5" strokeWidth={3} />
        </DropdownMenuPrimitive.ItemIndicator>
      </span>
    </DropdownMenuPrimitive.CheckboxItem>
  )
}

function DropdownMenuGroup(props: React.ComponentProps<typeof DropdownMenuPrimitive.Group>) {
  return <DropdownMenuPrimitive.Group data-slot="dropdown-menu-group" {...props} />
}

function DropdownMenuRadioGroup(props: React.ComponentProps<typeof DropdownMenuPrimitive.RadioGroup>) {
  return <DropdownMenuPrimitive.RadioGroup data-slot="dropdown-menu-radio-group" {...props} />
}

// Tek seçimli satır (menuitemradio). Görünümü çağıran belirler; seçili durum data-[state=checked].
function DropdownMenuRadioItem({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.RadioItem>) {
  return (
    <DropdownMenuPrimitive.RadioItem
      data-slot="dropdown-menu-radio-item"
      className={cn(
        'flex cursor-pointer items-center outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

function DropdownMenuLabel({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Label>) {
  return <DropdownMenuPrimitive.Label data-slot="dropdown-menu-label" className={cn('px-3 py-2 text-xs text-muted', className)} {...props} />
}

function DropdownMenuSeparator({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
  return <DropdownMenuPrimitive.Separator data-slot="dropdown-menu-separator" className={cn('my-1 h-px bg-rule', className)} {...props} />
}

export {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
}
