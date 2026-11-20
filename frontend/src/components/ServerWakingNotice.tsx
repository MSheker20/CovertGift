import { useEffect, useState, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { LoaderCircle } from 'lucide-react'

import { serverActivity } from '@/api/client'

// Uykudaki API'ye giden bir istek uzun sürerse ekranın altında küçük bir not gösterir.
// Azure'da API boştayken sıfıra ölçeklenir ve ilk istekte birkaç saniye uyanır; not, o
// sırada uygulamanın donmadığını anlatır. Hızlı yanıtlarda hiç görünmez (gecikme).
// Canlı bölge hep yerinde durur; ekran okuyucu metin gelince duyurur.
export function ServerWakingNotice({ delayMs = 3000 }: { delayMs?: number }) {
  const { t } = useTranslation()
  const waiting = useSyncExternalStore(serverActivity.subscribe, serverActivity.coldRequests) > 0
  const [delayPassed, setDelayPassed] = useState(false)

  // Yalnızca "bekleyen istek var/yok" geçişinde çalışır; ikinci bir istek notu yeniden başlatmaz.
  useEffect(() => {
    if (!waiting) return
    const timer = window.setTimeout(() => setDelayPassed(true), delayMs)
    return () => {
      window.clearTimeout(timer)
      setDelayPassed(false)
    }
  }, [waiting, delayMs])
  const visible = waiting && delayPassed

  return (
    <div role="status" aria-live="polite" className="pointer-events-none fixed inset-x-0 bottom-5 z-50 flex justify-center px-4">
      {visible && (
        <p className="flex items-center gap-2 rounded-full border border-border bg-raised px-4 py-2 text-sm text-text shadow-lg">
          <LoaderCircle className="size-4 shrink-0 animate-spin text-accent motion-reduce:animate-none" aria-hidden />
          {t('common.serverWaking')}
        </p>
      )}
    </div>
  )
}
