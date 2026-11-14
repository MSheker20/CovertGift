// Girişten sonra dönülecek yol (ör. /invite/:token). Yalnızca uygulama içi, tek eğik çizgiyle
// başlayan yollar kabul edilir: "//", ters eğik çizgi, kodlanmış eğik çizgi ile başlayan
// yollar ve adresi çözüldüğünde başka bir origin'e giden değerler ana sayfaya düşer.
export function returnPath(state: unknown): string {
  const from = (state as { from?: unknown } | null)?.from
  if (typeof from !== 'string' || !from.startsWith('/') || from.startsWith('//')) return '/'
  if (from.includes('\\') || /^\/(%2f|%5c)/i.test(from)) return '/'
  try {
    const origin = window.location.origin
    return new URL(from, origin).origin === origin ? from : '/'
  } catch {
    return '/'
  }
}
