import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { FormError, FormField } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input, Textarea } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { safeHttpUrl } from '@/lib/marketplace'
import type { Scope } from './scope'

const MONEY = /^\d{1,8}([.,]\d{1,2})?$/
const EMPTY = { link: '', title: '', price: '', searchQuery: '', notes: '', productUrl: '', imageUrl: '' }

// Link yapıştır → backend önizlemesi formu doldurur. Önizleme kısmi ya da başarısız olsa da
// kullanıcı formu elle tamamlayıp ekleyebilir.
export function AddIdeaDialog({ scope, trigger }: { scope: Scope; trigger: ReactNode }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(EMPTY)
  const [notice, setNotice] = useState<{ kind: 'ok' | 'partial'; text: string } | null>(null)
  const [localError, setLocalError] = useState<string | null>(null)

  const set = (key: keyof typeof EMPTY) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  const preview = useMutation({
    mutationFn: (url: string) => scope.previewLink(url),
    onSuccess: (p) => {
      setForm((f) => ({
        ...f,
        title: p.title || f.title,
        price: p.price ? p.price.replace(',', '.') : f.price,
        searchQuery: p.search_query || f.searchQuery,
        notes: f.notes || p.description,
        productUrl: p.final_url || f.link.trim(),
        imageUrl: p.image_url || '',
      }))
      setNotice(p.partial ? { kind: 'partial', text: t('addIdea.partial') } : { kind: 'ok', text: t('addIdea.filled') })
    },
  })

  const create = useMutation({
    mutationFn: () =>
      scope.createIdea({
        title: form.title.trim(),
        estimated_price: (form.price.trim() || '0').replace(',', '.'),
        search_query: form.searchQuery.trim() || undefined,
        description: form.notes.trim() || undefined,
        product_url: form.productUrl || safeHttpUrl(form.link.trim()) || undefined,
        image_url: form.imageUrl || undefined,
      }),
    onSuccess: (idea) => {
      void queryClient.invalidateQueries({ queryKey: scope.ideasKey })
      toast.success(t('addIdea.added', { title: idea.title }))
      onOpenChange(false)
    },
  })

  const fetchPreview = () => {
    if (!safeHttpUrl(form.link.trim())) {
      setLocalError(t('addIdea.invalidLink'))
      return
    }
    setLocalError(null)
    setNotice(null)
    preview.mutate(form.link.trim())
  }

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (form.price.trim() && !MONEY.test(form.price.trim())) {
      setLocalError(t('addIdea.invalidPrice'))
      return
    }
    setLocalError(null)
    create.mutate()
  }

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      setForm(EMPTY)
      setNotice(null)
      setLocalError(null)
      preview.reset()
      create.reset()
    }
  }

  const error = localError ?? (preview.error ? errorMessage(preview.error) : create.error ? errorMessage(create.error) : null)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent closeLabel={t('common.close')}>
        <DialogHeader>
          <DialogTitle>{t('addIdea.title')}</DialogTitle>
          <DialogDescription>{t('addIdea.description')}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-2">
          <FormField id="i-link" label={t('addIdea.link')}>
            <div className="flex gap-2">
              <Input
                id="i-link"
                type="url"
                inputMode="url"
                placeholder={t('addIdea.linkPlaceholder')}
                value={form.link}
                onChange={set('link')}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    fetchPreview()
                  }
                }}
              />
              <Button type="button" variant="outline" className="h-11 md:h-10" onClick={fetchPreview} disabled={preview.isPending || !form.link.trim()}>
                {preview.isPending ? t('addIdea.fetching') : t('addIdea.fetch')}
              </Button>
            </div>
          </FormField>
          {notice && (
            <p
              role="status"
              className={
                notice.kind === 'ok'
                  ? 'rounded-[12px] bg-soft px-3.5 py-2.5 text-sm text-on-soft'
                  : 'rounded-[12px] bg-pill-warn-bg px-3.5 py-2.5 text-sm text-pill-warn-ink'
              }
            >
              {notice.text}
            </p>
          )}
        </div>

        <form onSubmit={onSubmit} className="mt-5 flex flex-col gap-4 border-t border-rule pt-5">
          <FormError message={error} />
          <div className="flex gap-3">
            {form.imageUrl && safeHttpUrl(form.imageUrl) && (
              <img
                src={safeHttpUrl(form.imageUrl)!}
                alt=""
                referrerPolicy="no-referrer"
                className="size-[72px] shrink-0 rounded-[12px] bg-white object-contain p-1"
              />
            )}
            <div className="flex-1">
              <FormField id="i-title" label={t('addIdea.name')}>
                <Input id="i-title" required minLength={2} maxLength={200} value={form.title} onChange={set('title')} />
              </FormField>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField id="i-price" label={t('addIdea.price')}>
              <Input id="i-price" inputMode="decimal" placeholder="1450" className="num" value={form.price} onChange={set('price')} />
            </FormField>
            <FormField
              id="i-query"
              label={t('addIdea.searchQuery')}
              optional={t('addIdea.optional')}
              hint={t('addIdea.searchQueryHint')}
            >
              <Input id="i-query" maxLength={255} aria-describedby="i-query-hint" value={form.searchQuery} onChange={set('searchQuery')} />
            </FormField>
          </div>
          <FormField id="i-notes" label={t('addIdea.notes')} optional={t('addIdea.optional')}>
            <Textarea id="i-notes" rows={2} value={form.notes} onChange={set('notes')} />
          </FormField>
          <Button type="submit" size="lg" className="mt-1 w-full sm:ml-auto sm:w-auto" disabled={create.isPending}>
            {create.isPending ? t('common.loading') : t('addIdea.submit')}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
