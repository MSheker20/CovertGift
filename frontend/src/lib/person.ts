// Her kişiye kimliğinden türeyen sabit bir pastel ton verilir (kimlik, veri değil).
// Tonlar yan yana gelen kişiler için en az 60° aralıklı seçilir.
const HUES = [24, 150, 210, 300, 85, 345, 180, 260] as const

export function personHue(key: string): number {
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return HUES[h % HUES.length]
}

export function personColors(key: string) {
  const hue = personHue(key)
  return { background: `hsl(${hue} 70% 90%)`, color: `hsl(${hue} 45% 26%)` }
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  const first = parts[0][0] ?? ''
  const last = parts.length > 1 ? (parts[parts.length - 1][0] ?? '') : ''
  return (first + last).toLocaleUpperCase('tr-TR')
}
