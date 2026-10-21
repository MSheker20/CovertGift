import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  body: string
  confirmLabel: string
  cancelLabel: string
  pending?: boolean
  onConfirm: () => void
}

// Geri alınamayan işlemler (veto, silme) için nesnenin adını anan onay.
export function ConfirmDialog({ open, onOpenChange, title, body, confirmLabel, cancelLabel, pending, onConfirm }: Props) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{title}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{body}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={pending}>
            {cancelLabel}
          </Button>
          <Button className="bg-negative text-on-negative hover:bg-negative-hover" onClick={onConfirm} disabled={pending}>
            {pending ? t('common.loading') : confirmLabel}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
