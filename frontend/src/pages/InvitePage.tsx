import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { invitesApi, queryKeys } from '@/api/endpoints'
import { Bar, BlockError } from '@/components/Panel'
import { SurpriseScreen } from '@/components/SurpriseScreen'
import { Button } from '@/components/ui/button'
import { errorMessage, isAntiSpoiler } from '@/lib/errors'

// Davet linkini açan giriş yapmış kullanıcının karşılama ekranı. Giriş yoksa RequireAuth
// girişe yönlendirir ve bu yola geri döndürür. Token yalnızca yolda taşınır.
export function InvitePage() {
  const { t } = useTranslation()
  const { token = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const preview = useQuery({
    queryKey: queryKeys.invitePreview(token),
    queryFn: ({ signal }) => invitesApi.preview(token, signal),
    retry: false,
  })

  const accept = useMutation({
    mutationFn: () => invitesApi.accept(token),
    onSuccess: async ({ group_id }) => {
      toast.success(t('invite.page.joined'))
      await queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      navigate(`/groups/${group_id}`, { replace: true })
      // Geri tuşu "Katıl"ı tekrar göstermesin ve token önbellekte kalmasın.
      queryClient.removeQueries({ queryKey: queryKeys.invitePreview(token) })
    },
  })

  const failure = preview.error ?? accept.error
  if (isAntiSpoiler(failure)) return <SurpriseScreen />

  const invalid = failure instanceof ApiError && failure.status === 404
  const data = preview.data

  return (
    <div className="mx-auto flex max-w-[520px] flex-col px-4 pt-10 pb-16 md:pt-16">
      <section aria-busy={preview.isPending} className="rounded-[28px] bg-surface p-6 md:p-8">
        {preview.isPending ? (
          <div role="status">
            <span className="sr-only">{t('invite.page.loading')}</span>
            <Bar className="h-8 w-3/4" />
            <Bar className="mt-4 h-5 w-1/2" />
            <Bar className="mt-8 h-11 w-full" />
          </div>
        ) : invalid ? (
          <div role="alert">
            <h1 className="font-display text-[24px] leading-tight font-bold">{t('invite.page.invalidTitle')}</h1>
            <p className="mt-2 text-[15px] text-muted">{t('invite.page.invalidLead')}</p>
            <Button asChild size="lg" className="mt-6">
              <Link to="/">{t('invite.page.home')}</Link>
            </Button>
          </div>
        ) : preview.isError || !data ? (
          <BlockError message={t('invite.page.loadFailed')} onRetry={() => void preview.refetch()} retryLabel={t('common.retry')} />
        ) : (
          <div>
            <p className="text-sm text-muted">{t('invite.page.invitedBy', { name: data.inviter_name })}</p>
            <h1 className="font-display mt-2 text-[28px] leading-[1.1] font-bold md:text-[34px]">{data.group_title}</h1>
            <p className="mt-3 text-[15px] text-muted">{t('invite.page.members', { count: data.member_count })}</p>

            {data.already_member && data.group_id ? (
              <div className="mt-6">
                <p role="status" className="text-[15px] font-medium">
                  {t('invite.page.alreadyMember')}
                </p>
                <Button asChild size="lg" className="mt-4 w-full sm:w-auto">
                  <Link to={`/groups/${data.group_id}`}>{t('invite.page.goToGroup')}</Link>
                </Button>
              </div>
            ) : (
              <div className="mt-6">
                <Button size="lg" className="w-full sm:w-auto" onClick={() => accept.mutate()} disabled={accept.isPending}>
                  {accept.isPending ? t('invite.page.joining') : t('invite.page.join')}
                </Button>
                {accept.isError && (
                  <p role="alert" className="mt-3 text-sm text-negative">
                    {errorMessage(accept.error)}
                  </p>
                )}
              </div>
            )}
          </div>
        )}
      </section>
    </div>
  )
}
