// Gemini URL üretmez: pazar yeri linkleri yalnızca search_query'den burada kurulur.
export function marketplaceLinks(searchQuery: string) {
  const q = encodeURIComponent(searchQuery.trim())
  return [
    { name: 'Trendyol', href: `https://www.trendyol.com/sr?q=${q}` },
    { name: 'Amazon TR', href: `https://www.amazon.com.tr/s?k=${q}` },
  ]
}

// Kullanıcıya gösterilen dış linkler yalnızca http(s) olabilir (javascript: vb. engellenir).
export function safeHttpUrl(raw: string | undefined): string | null {
  if (!raw) return null
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : null
  } catch {
    return null
  }
}
