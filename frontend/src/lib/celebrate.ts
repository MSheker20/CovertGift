// Kazanan seçildiğinde kısa bir konfeti. Kütüphane yalnızca bu anda yüklenir (ayrı parça).
// Azaltılmış hareket tercihinde ya da canvas olmayan ortamda (testler) hiçbir şey yapmaz.
export async function celebrate(): Promise<void> {
  if (typeof window === 'undefined' || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  if (!document.createElement('canvas').getContext?.('2d')) return
  try {
    const { default: confetti } = await import('canvas-confetti')
    // Renkler tasarım token'larından okunur (kobalt, hardal, açık kobalt, yeşil).
    const css = getComputedStyle(document.documentElement)
    const colors = ['--primary', '--field', '--accent', '--positive']
      .map((name) => css.getPropertyValue(name).trim())
      .filter(Boolean)
    const base = { colors, disableForReducedMotion: true, zIndex: 60, ticks: 220 }
    void confetti({ ...base, particleCount: 90, spread: 70, startVelocity: 42, origin: { y: 0.4 } })
    setTimeout(() => {
      void confetti({ ...base, particleCount: 45, angle: 60, spread: 55, origin: { x: 0, y: 0.6 } })
      void confetti({ ...base, particleCount: 45, angle: 120, spread: 55, origin: { x: 1, y: 0.6 } })
    }, 180)
  } catch {
    // Konfeti süstür; yüklenemezse kutlama sessizce atlanır.
  }
}
