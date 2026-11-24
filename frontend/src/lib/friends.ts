// Arkadaşlık linkinin tam adresi. Kod yalnızca yolda taşınır.
export function friendLinkUrl(code: string): string {
  return `${window.location.origin}/f/${encodeURIComponent(code)}`
}
