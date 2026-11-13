import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, beforeEach } from 'vitest'

import i18n from '@/i18n'
import { server } from './server'

// jsdom'da olmayan tarayıcı API'leri (Radix menüleri ve Sonner bunları kullanır).
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= ResizeObserverStub as unknown as typeof ResizeObserver
window.matchMedia ??= ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener() {},
  removeListener() {},
  addEventListener() {},
  removeEventListener() {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia
// jsdom object URL üretmez; kapak görselleri için sahte ama benzersiz adresler yeter.
let objectUrlSeq = 0
URL.createObjectURL ??= () => `blob:test/${++objectUrlSeq}`
URL.revokeObjectURL ??= () => {}
Element.prototype.scrollIntoView ??= function () {}
Element.prototype.hasPointerCapture ??= () => false
Element.prototype.releasePointerCapture ??= function () {}
Element.prototype.setPointerCapture ??= function () {}

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
beforeEach(async () => {
  localStorage.clear()
  await i18n.changeLanguage('tr')
})
afterEach(() => {
  cleanup()
  server.resetHandlers()
})
afterAll(() => server.close())
