// Kapak fotoğrafı kırpma hesapları ve tarayıcı yardımcıları.
// Sunucu (backend/pkg/imageproc) aynı kuralları savunma olarak yeniden uygular: tür bayttan
// doğrulanır, görsel ortadan 4:3 kırpılır ve en çok 1600x1200 JPEG olarak saklanır.

export const COVER_MAX_BYTES = 10 * 1024 * 1024
export const COVER_ASPECT = 4 / 3
export const COVER_MAX_WIDTH = 1600
export const COVER_MIN_WIDTH = 64
export const COVER_MIN_HEIGHT = 48
export const COVER_JPEG_QUALITY = 0.85
const MAX_ZOOM = 4

const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/webp']
export const COVER_ACCEPT = ACCEPTED_TYPES.join(',')

export type CoverFileError = 'tooLarge' | 'heic' | 'type'

// Dosyayı okumadan önce boyut ve tür denetimi. Tarayıcı türü bildirmediyse uzantıya bakılır.
export function validateCoverFile(file: { size: number; type: string; name: string }): CoverFileError | null {
  const ext = file.name.toLowerCase().split('.').pop() ?? ''
  if (/hei[cf]/.test(file.type) || ext === 'heic' || ext === 'heif') return 'heic'
  const typeOk = file.type ? ACCEPTED_TYPES.includes(file.type) : ['jpg', 'jpeg', 'png', 'webp'].includes(ext)
  if (!typeOk) return 'type'
  if (file.size > COVER_MAX_BYTES) return 'tooLarge'
  return null
}

// Kırpma durumu: çerçevenin görseldeki merkezi (görsel pikseli) ve yakınlaştırma (1 = çerçeve
// görsele tam oturur).
export type CropState = { cx: number; cy: number; zoom: number }
export type CropRect = { sx: number; sy: number; sw: number; sh: number }

// Yakınlaştırma 1 iken görünen alan: görsele sığan en büyük 4:3 dikdörtgen.
function baseView(iw: number, ih: number) {
  return iw / ih > COVER_ASPECT ? { w: ih * COVER_ASPECT, h: ih } : { w: iw, h: iw / COVER_ASPECT }
}

// Görünen alan en küçük çıktı genişliğinin altına inmesin diye üst sınır görsele göre kısılır.
export function maxZoom(iw: number, ih: number): number {
  return Math.max(1, Math.min(MAX_ZOOM, baseView(iw, ih).w / COVER_MIN_WIDTH))
}

export function initialCrop(iw: number, ih: number): CropState {
  return { cx: iw / 2, cy: ih / 2, zoom: 1 }
}

// Yakınlaştırmayı sınırlar ve çerçeveyi görselin içinde tutar.
export function clampCrop(state: CropState, iw: number, ih: number): CropState {
  const zoom = Math.min(maxZoom(iw, ih), Math.max(1, state.zoom))
  const base = baseView(iw, ih)
  const w = base.w / zoom
  const h = base.h / zoom
  return {
    zoom,
    cx: Math.min(iw - w / 2, Math.max(w / 2, state.cx)),
    cy: Math.min(ih - h / 2, Math.max(h / 2, state.cy)),
  }
}

// Çerçevenin görseldeki karşılığı (kaynak dikdörtgen).
export function cropRect(state: CropState, iw: number, ih: number): CropRect {
  const s = clampCrop(state, iw, ih)
  const base = baseView(iw, ih)
  const sw = base.w / s.zoom
  const sh = base.h / s.zoom
  return { sx: s.cx - sw / 2, sy: s.cy - sh / 2, sw, sh }
}

// Çıktı boyutu: kaynak alandan büyük olmaz (büyütme yok), en çok 1600x1200, tam 4:3.
export function outputSize(sourceWidth: number): { width: number; height: number } {
  const width = Math.max(4, Math.floor(Math.min(COVER_MAX_WIDTH, sourceWidth) / 4) * 4)
  return { width, height: (width / 4) * 3 }
}

export function isTooSmall(iw: number, ih: number): boolean {
  return iw < COVER_MIN_WIDTH || ih < COVER_MIN_HEIGHT
}

export type LoadedImage = {
  element: HTMLImageElement
  url: string
  width: number
  height: number
  // Object URL'yi serbest bırakır.
  release: () => void
}

// Dosyayı <img> olarak yükler. Tarayıcı EXIF yönünü hem gösterimde hem canvas çiziminde uygular.
export function loadImage(file: Blob): Promise<LoadedImage> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const element = new Image()
    element.onload = () =>
      resolve({
        element,
        url,
        width: element.naturalWidth,
        height: element.naturalHeight,
        release: () => URL.revokeObjectURL(url),
      })
    element.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('decode'))
    }
    element.src = url
  })
}

// Seçilen alanı canvas'a çizer ve JPEG blob üretir. Şeffaf alanlar beyaz zemine oturur.
export function renderCrop(image: CanvasImageSource, rect: CropRect): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const { width, height } = outputSize(rect.sw)
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      reject(new Error('canvas'))
      return
    }
    ctx.fillStyle = 'white'
    ctx.fillRect(0, 0, width, height)
    ctx.imageSmoothingEnabled = true
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(image, rect.sx, rect.sy, rect.sw, rect.sh, 0, 0, width, height)
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('encode'))), 'image/jpeg', COVER_JPEG_QUALITY)
  })
}
