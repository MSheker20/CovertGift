import { useTranslation } from 'react-i18next'
import { Link, Outlet, useMatch, useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronDown, LayoutGrid, LogOut, UserRound, Users } from 'lucide-react'

import { groupsApi, queryKeys } from '@/api/endpoints'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { GroupThumb } from '@/components/GroupCover'
import { LanguageSwitch } from '@/components/LanguageSwitch'
import { Logo, LogoMark } from '@/components/Logo'
import { FriendsPopover } from '@/components/FriendsPopover'
import { NotificationBell } from '@/components/NotificationBell'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { LANGUAGES } from '@/i18n'

// Yüzen üst bar (kullanıcı isteği, 2026-10-21): sayfanın üstünde, kenarlardan içeride duran
// yuvarlatılmış tek bar. Solda işaret, yanında grup seçici; sağda dil, bildirim zili ve hesap menüsü.
// Opak yüzey ve 1px kenar; bulanıklık yok. Telefonda aynı bar, kelime işareti ve dil anahtarı gizli.
export function AppShell() {
  return (
    <div className="min-h-dvh bg-ground">
      {/* Sayfa geçişinde üst bar yerinde kalır; yalnızca içerik değişir. */}
      <div className="sticky top-0 z-40 px-3 pt-3 [view-transition-name:app-header] md:px-6 md:pt-4">
        <header className="mx-auto flex h-14 max-w-[1148px] items-center gap-1 rounded-full border border-border bg-raised py-2 pr-2 pl-3 md:h-16 md:gap-2 md:pl-5">
          <Link to="/" className="mr-1 inline-flex shrink-0 items-center rounded-full md:mr-3" aria-label="CovertGift">
            <LogoMark className="size-7 sm:hidden" />
            <Logo className="hidden sm:inline-flex" />
          </Link>
          <GroupSwitcher />
          <div className="ml-auto flex shrink-0 items-center gap-1 md:gap-2">
            <LanguageSwitch className="hidden md:inline-flex" />
            <FriendsPopover />
            <NotificationBell />
            <AccountMenu />
          </div>
        </header>
      </div>
      <main className="min-w-0">
        <Outlet />
      </main>
    </div>
  )
}

// Grup seçici: ana sayfada "Tüm gruplar", grup sayfasında o grubun adı. Menüde tüm gruplar listelenir.
function GroupSwitcher() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const match = useMatch('/groups/:id')
  const currentId = match?.params.id
  const groups = useQuery({ queryKey: queryKeys.groups, queryFn: ({ signal }) => groupsApi.list(signal) })
  const current = groups.data?.find((g) => g.id === currentId)
  const label = current ? current.title : t('nav.allGroups')

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('nav.switchGroup')}
          className="inline-flex h-10 min-w-0 cursor-pointer items-center gap-2 rounded-full bg-surface pr-3 pl-1.5 text-sm font-medium transition-colors hover:bg-active data-[state=open]:bg-active md:h-11 md:pr-4"
        >
          {current ? (
            <GroupThumb group={current} size="sm" />
          ) : (
            <span className="inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-soft text-on-soft">
              <LayoutGrid className="size-3.5" />
            </span>
          )}
          <span className="max-w-[38vw] truncate sm:max-w-[260px]">{label}</span>
          <ChevronDown className="size-4 shrink-0 text-muted" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[min(320px,calc(100vw-24px))]">
        <DropdownMenuItem onSelect={() => navigate('/')}>
          <LayoutGrid />
          <span className="flex-1">{t('nav.allGroups')}</span>
          {!currentId && <Check className="!text-accent" />}
        </DropdownMenuItem>
        {(groups.data?.length ?? 0) > 0 && <DropdownMenuSeparator />}
        {groups.data?.map((g) => (
          <DropdownMenuItem key={g.id} onSelect={() => navigate(`/groups/${g.id}`)} className="h-auto min-h-11 py-2">
            <GroupThumb group={g} size="sm" />
            <span className="min-w-0 flex-1">
              <span className="block truncate">{g.title}</span>
              <span className="block truncate text-xs text-muted">{t('home.forPerson', { name: g.target_person_name })}</span>
            </span>
            {g.id === currentId && <Check className="!text-accent" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function AccountMenu() {
  const { t, i18n } = useTranslation()
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  if (!user) return <span className="size-10" />
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('nav.account')}
          className="inline-flex h-10 cursor-pointer items-center gap-2.5 rounded-full pr-1 pl-1 transition-colors hover:bg-surface data-[state=open]:bg-surface md:h-11 md:pr-3"
        >
          <Avatar name={user.name} seed={user.id} userId={user.id} version={user.avatar_version} />
          <span className="hidden max-w-[160px] min-w-0 text-left lg:block">
            <span className="block truncate text-sm leading-5 font-medium">{user.name}</span>
            <span className="block truncate text-xs leading-4 text-muted">{user.email}</span>
          </span>
          <ChevronDown className="hidden size-4 text-muted lg:block" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel className="flex items-center gap-3 py-2.5">
          <Avatar name={user.name} seed={user.id} userId={user.id} version={user.avatar_version} />
          <span className="min-w-0">
            <span className="block truncate text-sm font-medium text-ink">{user.name}</span>
            <span className="block truncate text-xs">{user.email}</span>
          </span>
        </DropdownMenuLabel>
        {/* Telefonda dil anahtarı barda yer almadığı için menüde. */}
        <div className="md:hidden">
          <DropdownMenuSeparator />
          {LANGUAGES.map((lng) => (
            <DropdownMenuItem key={lng} onSelect={() => void i18n.changeLanguage(lng)}>
              <span className="flex-1">{lng === 'tr' ? 'Türkçe' : 'English'}</span>
              {i18n.language === lng && <Check className="!text-accent" />}
            </DropdownMenuItem>
          ))}
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate('/profile')}>
          <UserRound />
          {t('nav.profile')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate('/friends')}>
          <Users />
          {t('nav.friends')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={logout}>
          <LogOut />
          {t('nav.logout')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
