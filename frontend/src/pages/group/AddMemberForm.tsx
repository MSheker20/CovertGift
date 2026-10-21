import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { groupsApi, queryKeys } from '@/api/endpoints'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'

// Her üye e-postayla üye ekleyebilir; hedef kişi ve mevcut üyeler backend'de reddedilir.
export function AddMemberForm({ groupId }: { groupId: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')

  const add = useMutation({
    mutationFn: () => groupsApi.addMember(groupId, email.trim()),
    onSuccess: (member) => {
      setEmail('')
      toast.success(t('members.added', { name: member.user?.name ?? email.trim() }))
      // Üye sayısı kişi başı bütçeyi ve çoğunluk vetosu eşiğini değiştirir.
      void queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId) })
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    add.mutate()
  }

  const error = add.error
    ? add.error instanceof ApiError && add.error.status === 404
      ? t('members.notFound')
      : errorMessage(add.error)
    : null

  return (
    <form onSubmit={onSubmit} className="mt-4 border-t border-rule pt-4">
      <label htmlFor="m-email" className="text-sm font-medium">
        {t('members.add')}
      </label>
      <div className="mt-2 flex gap-2">
        <Input
          id="m-email"
          type="email"
          required
          autoComplete="off"
          placeholder={t('members.emailPlaceholder')}
          aria-describedby="m-email-msg"
          aria-invalid={error ? true : undefined}
          value={email}
          onChange={(e) => {
            setEmail(e.target.value)
            if (add.error) add.reset()
          }}
        />
        <Button type="submit" variant="outline" className="h-11 md:h-10" disabled={add.isPending}>
          {add.isPending ? t('common.loading') : t('members.submit')}
        </Button>
      </div>
      <p id="m-email-msg" role={error ? 'alert' : undefined} className={`mt-2 text-xs ${error ? 'text-negative' : 'text-muted'}`}>
        {error ?? t('members.hint')}
      </p>
    </form>
  )
}
