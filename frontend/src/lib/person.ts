import type React from 'react'

// Her kişiye kimliğinden türeyen sabit bir pastel ton verilir (kimlik, veri değil).
// Tonlar yan yana gelen kişiler için en az 60° aralıklı seçilir.
const HUES = [24, 150, 210, 300, 85, 345, 180, 260] as const

export function personHue(key: string): number {
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return HUES[h % HUES.length]
}

// Açık/koyu tema doygunluk ve açıklığı index.css'teki .avatar sınıfından gelir; burada yalnızca ton seçilir.
export function personStyle(key: string) {
  return { '--h': String(personHue(key)) } as React.CSSProperties
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  const first = parts[0][0] ?? ''
  const last = parts.length > 1 ? (parts[parts.length - 1][0] ?? '') : ''
  return (first + last).toLocaleUpperCase('tr-TR')
}
