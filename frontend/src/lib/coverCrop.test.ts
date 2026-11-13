import { describe, expect, it } from 'vitest'

import {
  COVER_MAX_BYTES,
  COVER_UPLOAD_MAX_BYTES,
  COVER_WORK_MAX_EDGE,
  clampCrop,
  cropRect,
  initialCrop,
  isTooSmall,
  maxZoom,
  outputSize,
  validateCoverFile,
  workingSize,
} from './coverCrop'

const file = (name: string, type: string, size = 1000) => ({ name, type, size })

describe('validateCoverFile', () => {
  it('accepts large original photos, since the browser shrinks them before upload', () => {
    expect(COVER_MAX_BYTES).toBe(40 * 1024 * 1024)
    expect(COVER_UPLOAD_MAX_BYTES).toBe(2 * 1024 * 1024)
    expect(validateCoverFile(file('a.jpg', 'image/jpeg'))).toBeNull()
    expect(validateCoverFile(file('a.png', 'image/png'))).toBeNull()
    // 48 MP telefon fotoğrafı: eski 10 MB sınırının üstünde, artık seçilebilir.
    expect(validateCoverFile(file('IMG_48MP.jpg', 'image/jpeg', 18 * 1024 * 1024))).toBeNull()
    expect(validateCoverFile(file('a.webp', 'image/webp', COVER_MAX_BYTES))).toBeNull()
  })

  it('rejects files over 40 MB', () => {
    expect(validateCoverFile(file('a.jpg', 'image/jpeg', COVER_MAX_BYTES + 1))).toBe('tooLarge')
  })

  it('lets HEIC through to the browser decoder (by type or by extension)', () => {
    expect(validateCoverFile(file('IMG_1.HEIC', 'image/heic'))).toBeNull()
    expect(validateCoverFile(file('IMG_1.heif', ''))).toBeNull()
  })

  it('rejects other types and falls back to the extension when the browser reports no type', () => {
    expect(validateCoverFile(file('a.gif', 'image/gif'))).toBe('type')
    expect(validateCoverFile(file('notes.pdf', 'application/pdf'))).toBe('type')
    expect(validateCoverFile(file('a.svg', 'image/svg+xml'))).toBe('type')
    expect(validateCoverFile(file('a.jpeg', ''))).toBeNull()
    expect(validateCoverFile(file('a', ''))).toBe('type')
  })
})

describe('crop geometry', () => {
  it('starts with the largest centred 4:3 area', () => {
    // 16:9: tam yükseklik, ortadan 1440 genişlik.
    expect(cropRect(initialCrop(1920, 1080), 1920, 1080)).toEqual({ sx: 240, sy: 0, sw: 1440, sh: 1080 })
    // Dikey: tam genişlik, ortadan 750 yükseklik.
    expect(cropRect(initialCrop(1000, 2000), 1000, 2000)).toEqual({ sx: 0, sy: 625, sw: 1000, sh: 750 })
    // Zaten 4:3: görselin tamamı.
    expect(cropRect(initialCrop(800, 600), 800, 600)).toEqual({ sx: 0, sy: 0, sw: 800, sh: 600 })
  })

  it('keeps the frame inside the image when dragged past an edge', () => {
    expect(cropRect({ cx: -500, cy: 540, zoom: 1 }, 1920, 1080).sx).toBe(0)
    expect(cropRect({ cx: 99999, cy: 540, zoom: 1 }, 1920, 1080).sx).toBe(480)
    const zoomed = cropRect({ cx: 0, cy: 0, zoom: 2 }, 1920, 1080)
    expect(zoomed).toEqual({ sx: 0, sy: 0, sw: 720, sh: 540 })
    const corner = cropRect({ cx: 99999, cy: 99999, zoom: 2 }, 1920, 1080)
    expect(corner.sx + corner.sw).toBe(1920)
    expect(corner.sy + corner.sh).toBe(1080)
  })

  it('always yields a 4:3 source area', () => {
    for (const [iw, ih, zoom] of [
      [1920, 1080, 1],
      [1000, 2000, 1.7],
      [333, 777, 3],
    ] as const) {
      const r = cropRect({ cx: iw / 3, cy: ih / 3, zoom }, iw, ih)
      expect(r.sw / r.sh).toBeCloseTo(4 / 3, 6)
      expect(r.sx).toBeGreaterThanOrEqual(0)
      expect(r.sy).toBeGreaterThanOrEqual(0)
    }
  })

  it('limits zoom to 1–4 and never below the minimum output width', () => {
    expect(clampCrop({ cx: 960, cy: 540, zoom: 0.2 }, 1920, 1080).zoom).toBe(1)
    expect(clampCrop({ cx: 960, cy: 540, zoom: 9 }, 1920, 1080).zoom).toBe(4)
    expect(maxZoom(1920, 1080)).toBe(4)
    // 128 piksel genişlikte görsel en çok 2 kat yakınlaşır (görünen alan 64 pikselin altına inmez).
    expect(maxZoom(128, 96)).toBe(2)
    expect(maxZoom(64, 48)).toBe(1)
  })
})

describe('outputSize', () => {
  it('caps at 1600x1200 and never upscales', () => {
    expect(outputSize(4000)).toEqual({ width: 1600, height: 1200 })
    expect(outputSize(1600)).toEqual({ width: 1600, height: 1200 })
    expect(outputSize(800)).toEqual({ width: 800, height: 600 })
  })

  it('rounds down to an exact 4:3 size', () => {
    expect(outputSize(801.7)).toEqual({ width: 800, height: 600 })
    expect(outputSize(1439)).toEqual({ width: 1436, height: 1077 })
  })
})

describe('isTooSmall', () => {
  it('requires at least 64x48', () => {
    expect(isTooSmall(63, 500)).toBe(true)
    expect(isTooSmall(500, 47)).toBe(true)
    expect(isTooSmall(64, 48)).toBe(false)
  })
})

describe('workingSize', () => {
  it('keeps photos whose long edge fits the working size', () => {
    expect(workingSize(1600, 1200)).toBeNull()
    expect(workingSize(COVER_WORK_MAX_EDGE, 2400)).toBeNull()
  })

  it('shrinks large photos to the working size, keeping the aspect ratio', () => {
    expect(workingSize(6000, 4000)).toEqual({ width: 3200, height: 2133 })
    expect(workingSize(6000, 8000)).toEqual({ width: 2400, height: 3200 })
    expect(workingSize(8064, 6048)).toEqual({ width: 3200, height: 2400 }) // 48 MP
  })
})
