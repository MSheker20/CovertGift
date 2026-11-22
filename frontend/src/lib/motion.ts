import { useEffect, useRef, useState, type CSSProperties } from 'react'

// Liste girişi: yalnızca bileşen ilk açıldıktan sonraki kısa pencerede oynar. Oylamada sıra
// değişince React düğümleri taşır ve CSS animasyonu yeniden başlardı; pencere bunu engeller.
// Pencere veri hazır olunca (`ready`) başlar; yükleme uzun sürse de giriş kaçmaz.
export function useIntroWindow(ready = true, ms = 900): boolean {
  const [on, setOn] = useState(true)
  useEffect(() => {
    if (!ready) return
    const timer = setTimeout(() => setOn(false), ms)
    return () => clearTimeout(timer)
  }, [ready, ms])
  return on
}

// Sıralı giriş gecikmesi: ilk 8 öğe 35 ms arayla, sonrası birlikte.
export function staggerStyle(index: number): CSSProperties {
  return { animationDelay: `${Math.min(index, 8) * 35}ms` }
}

export const INTRO_CLASS = 'animate-[rise-in_260ms_cubic-bezier(0.2,0.8,0.2,1)_both]'

// Önceki listede olmayan kimlikler (ör. SSE ile gelen yeni fikir, yeni seçilen kazanan).
// İlk yüklemede hiçbir şey "yeni" sayılmaz; sonuç `ms` sonra temizlenir.
export function useNewIds(ids: string[], ready: boolean, ms = 1800): ReadonlySet<string> {
  const previous = useRef<Set<string> | null>(null)
  const [fresh, setFresh] = useState<ReadonlySet<string>>(() => new Set())
  const key = ids.join(',')

  useEffect(() => {
    if (!ready) return
    const current = new Set(key ? key.split(',') : [])
    const before = previous.current
    previous.current = current
    if (!before) return
    const added = [...current].filter((id) => !before.has(id))
    if (added.length > 0) setFresh(new Set(added))
  }, [key, ready])

  useEffect(() => {
    if (fresh.size === 0) return
    const timer = setTimeout(() => setFresh(new Set()), ms)
    return () => clearTimeout(timer)
  }, [fresh, ms])

  return fresh
}
