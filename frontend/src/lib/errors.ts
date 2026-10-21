import { ApiError } from '@/api/client'
import i18n from '@/i18n'

// Backend mesajları Türkçedir; bilinen durumlar dile göre çevrilir, diğerleri olduğu gibi gösterilir.
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 0) return i18n.t('common.networkError')
    return err.message || i18n.t('common.unknownError')
  }
  return i18n.t('common.unknownError')
}

export function isAntiSpoiler(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403 && /sürpriz/i.test(err.message)
}
