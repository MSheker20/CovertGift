import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { friendsApi, queryKeys } from '@/api/endpoints'
import { Avatar } from '@/components/Avatar'
import { Bar, BlockError } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/errors'

// Kişisel arkadaşlık linkini açan giriş yapmış kullanıcının karşılama ekranı. Giriş yoksa
// RequireAuth girişe yönlendirir ve bu yola geri döndürür. Onaylayınca doğrudan arkadaş olunur.
export function FriendLinkPage() {
  const { t } = useTranslation()
  const { code = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const preview = useQuery({
    queryKey: queryKeys.friendLinkPreview(code),
    queryFn: ({ signal }) => friendsApi.previewLink(code, signal),
    retry: false,
  })

  const accept = useMutation({
    mutationFn: () => friendsApi.acceptLink(code),
    onSuccess: async () => {
      toast.success(t('friends.linkPage.added', { name: preview.data?.name ?? '' }))
      await queryClient.invalidateQueries({ queryKey: queryKeys.friends, exact: true })
      navigate('/friends', { replace: true })
      queryClient.removeQueries({ queryKey: queryKeys.friendLinkPreview(code) })
    },
  })

  const invalid = preview.error instanceof ApiError && preview.error.status === 404
  const data = preview.data

  return (
    <div className="mx-auto flex max-w-[520px] flex-col px-4 pt-10 pb-16 md:pt-16">
      <section aria-busy={preview.isPending} className="rounded-[28px] bg-surface p-6 md:p-8">
        {preview.isPending ? (
          <div role="status">
            <span className="sr-only">{t('common.loading')}</span>
            <Bar className="size-16 rounded-full" />
            <Bar className="mt-4 h-8 w-3/4" />
            <Bar className="mt-8 h-11 w-full" />
          </div>
        ) : invalid ? (
          <div role="alert">
            <h1 className="font-display text-[24px] leading-tight font-bold">{t('friends.linkPage.invalidTitle')}</h1>
            <p className="mt-2 text-[15px] text-muted">{t('friends.linkPage.invalidLead')}</p>
            <Button asChild size="lg" className="mt-6">
              <Link to="/">{t('invite.page.home')}</Link>
            </Button>
          </div>
        ) : preview.isError || !data ? (
          <BlockError message={t('friends.linkPage.loadFailed')} onRetry={() => void preview.refetch()} retryLabel={t('common.retry')} />
        ) : (
          <div>
            <Avatar name={data.name} seed={data.user_id} userId={data.user_id} version={data.avatar_version} size="lg" />
            <h1 className="font-display mt-4 text-[28px] leading-[1.1] font-bold md:text-[34px]">
              {data.is_self ? t('friends.linkPage.selfTitle') : t('friends.linkPage.title', { name: data.name })}
            </h1>
            <p className="mt-3 text-[15px] text-muted">
              {data.is_self
                ? t('friends.linkPage.selfLead')
                : data.already_friend
                  ? t('friends.linkPage.alreadyLead', { name: data.name })
                  : t('friends.linkPage.lead', { name: data.name })}
            </p>
            <div className="mt-6 flex flex-col gap-2 sm:flex-row">
              {data.is_self || data.already_friend ? (
                <Button asChild size="lg">
                  <Link to="/friends">{t('friends.linkPage.goToFriends')}</Link>
                </Button>
              ) : (
                <Button size="lg" onClick={() => accept.mutate()} disabled={accept.isPending}>
                  {accept.isPending ? t('common.loading') : t('friends.linkPage.accept')}
                </Button>
              )}
            </div>
            {accept.isError && (
              <p role="alert" className="mt-3 text-sm text-negative">
                {errorMessage(accept.error)}
              </p>
            )}
          </div>
        )}
      </section>
    </div>
  )
}
