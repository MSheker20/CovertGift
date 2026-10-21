import * as React from 'react'
import { Slot } from 'radix-ui'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '@/lib/utils'

// shadcn/ui Button, CovertGift token'larına uyarlanmış: hap köşeler, gölgesiz,
// tek dolu ana aksiyon kobalt, ikincil yüzey + 1px kenar.
const buttonVariants = cva(
  'inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-full font-medium transition-[background-color,color,transform,filter] duration-150 active:scale-[0.97] disabled:pointer-events-none disabled:opacity-60 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'bg-primary font-semibold text-on-primary hover:bg-primary-hover',
        field: 'bg-field font-semibold text-on-field hover:brightness-[0.96]',
        outline: 'border border-control bg-raised text-ink hover:bg-surface',
        ghost: 'text-ink hover:bg-active',
        link: 'rounded-none px-0 text-ink underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-[38px] px-4 text-sm',
        sm: 'h-8 px-3 text-sm',
        lg: 'h-11 px-5 text-[15px]',
        icon: 'size-[38px]',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
)

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> & VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : 'button'
  return <Comp data-slot="button" className={cn(buttonVariants({ variant, size, className }))} {...props} />
}

// eslint-disable-next-line react-refresh/only-export-components
export { Button, buttonVariants }
