import i18n from '@/i18n'

// Para her yerde iki ondalıkla, TRY ve aktif dilin ayraçlarıyla gösterilir.
export function formatMoney(value: string | number): string {
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  return new Intl.NumberFormat(locale(), {
    style: 'currency',
    currency: 'TRY',
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(n)
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale(), { day: 'numeric', month: 'long', year: 'numeric' }).format(d)
}

function locale() {
  return i18n.language === 'en' ? 'en-GB' : 'tr-TR'
}
