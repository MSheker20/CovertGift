import { describe, expect, it } from 'vitest'

import i18n from '@/i18n'
import { formatMoney } from './format'
import { marketplaceLinks, safeHttpUrl } from './marketplace'
import { initials, personHue } from './person'

describe('marketplaceLinks', () => {
  it('builds encoded Trendyol and Amazon TR search URLs from search_query only', () => {
    expect(marketplaceLinks(' hediye çeki & kupa ')).toEqual([
      { name: 'Trendyol', href: 'https://www.trendyol.com/sr?q=hediye%20%C3%A7eki%20%26%20kupa' },
      { name: 'Amazon TR', href: 'https://www.amazon.com.tr/s?k=hediye%20%C3%A7eki%20%26%20kupa' },
    ])
  })
})

describe('safeHttpUrl', () => {
  it.each([
    ['https://ty.gl/abc', 'https://ty.gl/abc'],
    ['http://example.org/x', 'http://example.org/x'],
    ['javascript:alert(1)', null],
    ['data:text/html,hi', null],
    ['not a url', null],
    [undefined, null],
  ])('%s → %s', (input, expected) => {
    expect(safeHttpUrl(input)).toBe(expected)
  })
})

describe('formatMoney', () => {
  it('uses two decimals and the active language separators', async () => {
    await i18n.changeLanguage('tr')
    expect(formatMoney('3200.5')).toBe('₺3.200,50')
    await i18n.changeLanguage('en')
    expect(formatMoney(3200.5)).toBe('₺3,200.50')
  })

  it('prints a dash for values that are not numbers', () => {
    expect(formatMoney('abc')).toBe('—')
  })
})

describe('person helpers', () => {
  it('takes first and last initials with Turkish casing', () => {
    expect(initials('ilker şahin')).toBe('İŞ')
    expect(initials('  Deniz  ')).toBe('D')
    expect(initials('')).toBe('?')
  })

  it('gives the same hue for the same key', () => {
    expect(personHue('u-1')).toBe(personHue('u-1'))
  })
})
