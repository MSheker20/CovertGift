import { Toaster as Sonner, type ToasterProps } from 'sonner'

// shadcn/ui Sonner; tema sistem ayarını izler (next-themes kullanılmıyor).
function Toaster(props: ToasterProps) {
  return (
    <Sonner
      theme="system"
      position="top-center"
      toastOptions={{
        classNames: {
          toast: '!rounded-[14px] !border-border !bg-raised !text-ink !font-sans',
          description: '!text-muted',
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
