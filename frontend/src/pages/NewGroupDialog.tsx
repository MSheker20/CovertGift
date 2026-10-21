import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { groupsApi, queryKeys } from '@/api/endpoints'
import { FormError, FormField } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input, Textarea } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'

const MONEY = /^\d{1,8}([.,]\d{1,2})?$/

const EMPTY = { title: '', target: '', budget: '', interests: '', notes: '' }

export function NewGroupDialog({ trigger }: { trigger: ReactNode }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(EMPTY)
  const [localError, setLocalError] = useState<string | null>(null)

  const set = (key: keyof typeof EMPTY) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  const mutation = useMutation({
    mutationFn: () =>
      groupsApi.create({
        title: form.title.trim(),
        target_person_name: form.target.trim(),
        total_budget: (form.budget.trim() || '0').replace(',', '.'),
        description: form.notes.trim() || undefined,
        // Backend virgül ve satır sonu ayrımlı metni listeye ayrıştırır.
        target_interests: form.interests.trim() || undefined,
      }),
    onSuccess: (group) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      toast.success(t('newGroup.created', { title: group.title }))
      setOpen(false)
      setForm(EMPTY)
      navigate(`/groups/${group.id}`)
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (form.budget.trim() && !MONEY.test(form.budget.trim())) {
      setLocalError(t('newGroup.invalidBudget'))
      return
    }
    setLocalError(null)
    mutation.mutate()
  }

  const onOpenChange = (next: boolean) => {
    setOpen(next)
    if (!next) {
      mutation.reset()
      setLocalError(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent closeLabel={t('common.close')}>
        <DialogHeader>
          <DialogTitle>{t('newGroup.title')}</DialogTitle>
          <DialogDescription>{t('newGroup.description')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <FormError message={localError ?? (mutation.error ? errorMessage(mutation.error) : null)} />
          <FormField id="g-title" label={t('newGroup.groupTitle')}>
            <Input
              id="g-title"
              required
              minLength={2}
              maxLength={150}
              placeholder={t('newGroup.groupTitlePlaceholder')}
              value={form.title}
              onChange={set('title')}
            />
          </FormField>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField id="g-target" label={t('newGroup.target')}>
              <Input
                id="g-target"
                required
                minLength={2}
                maxLength={100}
                placeholder={t('newGroup.targetPlaceholder')}
                value={form.target}
                onChange={set('target')}
              />
            </FormField>
            <FormField id="g-budget" label={t('newGroup.budget')} hint={t('newGroup.budgetHint')}>
              <Input
                id="g-budget"
                inputMode="decimal"
                placeholder="2500"
                aria-describedby="g-budget-hint"
                className="num"
                value={form.budget}
                onChange={set('budget')}
              />
            </FormField>
          </div>
          <FormField
            id="g-interests"
            label={t('newGroup.interests')}
            optional={t('newGroup.optional')}
            hint={t('newGroup.interestsHint')}
          >
            <Input
              id="g-interests"
              placeholder={t('newGroup.interestsPlaceholder')}
              aria-describedby="g-interests-hint"
              value={form.interests}
              onChange={set('interests')}
            />
          </FormField>
          <FormField id="g-notes" label={t('newGroup.notes')} optional={t('newGroup.optional')}>
            <Textarea
              id="g-notes"
              rows={2}
              placeholder={t('newGroup.notesPlaceholder')}
              value={form.notes}
              onChange={set('notes')}
            />
          </FormField>
          <Button type="submit" size="lg" className="mt-1 w-full sm:ml-auto sm:w-auto" disabled={mutation.isPending}>
            {mutation.isPending ? t('common.loading') : t('newGroup.submit')}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
