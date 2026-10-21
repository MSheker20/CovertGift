import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink, Outlet } from 'react-router'
import { Gift, LogOut, Menu } from 'lucide-react'

import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { LanguageSwitch } from '@/components/LanguageSwitch'
import { Logo } from '@/components/Logo'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

// "Open plane" kabuğu: masaüstünde sayfa zemininde etiketli kenar çubuğu (232px, 1px ayraç),
// telefonda 56px üst bar + menü sayfası.
export function AppShell() {
  return (
    <div className="min-h-dvh bg-ground lg:grid lg:grid-cols-[232px_minmax(0,1fr)]">
      <aside className="sticky top-0 hidden h-dvh flex-col border-r border-border px-3 py-5 lg:flex">
        <div className="px-2.5">
          <Logo />
        </div>
        <Nav className="mt-8" />
        <div className="mt-auto flex flex-col gap-4">
          <LanguageSwitch className="mx-2.5 self-start" />
          <UserRow />
        </div>
      </aside>
      <MobileBar />
      <main className="min-w-0">
        <Outlet />
      </main>
    </div>
  )
}

function Nav({ className, onNavigate }: { className?: string; onNavigate?: () => void }) {
  const { t } = useTranslation()
  return (
    <nav aria-label={t('nav.main')} className={className}>
      <NavLink
        to="/"
        end={false}
        onClick={onNavigate}
        className={({ isActive }) =>
          cn(
            'flex h-10 items-center gap-2.5 rounded-[10px] px-2.5 text-sm text-ink transition-colors hover:bg-surface lg:h-8',
            isActive && 'bg-active hover:bg-active',
          )
        }
      >
        <Gift className="size-4 text-muted" strokeWidth={1.6} />
        {t('nav.groups')}
      </NavLink>
    </nav>
  )
}

function UserRow() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  if (!user) return <div className="h-12" />
  return (
    <div className="flex items-center gap-2.5 px-2.5">
      <Avatar name={user.name} seed={user.id} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{user.name}</p>
        <p className="truncate text-xs text-muted">{user.email}</p>
      </div>
      <Button variant="ghost" size="icon" onClick={logout} aria-label={t('nav.logout')} title={t('nav.logout')}>
        <LogOut />
      </Button>
    </div>
  )
}

function MobileBar() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between border-b border-border bg-ground px-4 lg:hidden">
      <Logo />
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t('nav.menu')}>
            <Menu className="size-5" />
          </Button>
        </DialogTrigger>
        <DialogContent closeLabel={t('common.close')} aria-describedby={undefined}>
          <DialogTitle className="mb-4 font-sans text-base font-semibold">{t('nav.menu')}</DialogTitle>
          <Nav onNavigate={() => setOpen(false)} />
          <div className="mt-6 flex flex-col gap-5 border-t border-rule pt-5">
            <LanguageSwitch className="mx-2.5 self-start" />
            <UserRow />
          </div>
        </DialogContent>
      </Dialog>
    </header>
  )
}
