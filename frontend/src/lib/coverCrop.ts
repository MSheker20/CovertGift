// Kapak (4:3) ve profil (kare) fotoğrafı kırpma hesapları ve tarayıcı yardımcıları.
// Büyük fotoğrafı küçültmek tarayıcının işidir: sunucu (backend/pkg/imageproc) yalnızca küçük
// görsel kabul eder (en çok 2 MiB, uzun kenar 2000 piksel) ve aynı kuralları savunma olarak
// yeniden uygular. Buradan her zaman 4:3, en çok 1600x1200 ve 2 MiB altında bir JPEG çıkar.

// Seçilebilecek en büyük ORİJİNAL dosya. Tarayıcıda küçültüldüğü için sunucu sınırından
// (COVER_UPLOAD_MAX_BYTES) çok büyüktür; 48 MP telefon fotoğrafları rahatça sığar.
export const COVER_MAX_BYTES = 40 * 1024 * 1024
// Çözülmeye çalışılacak en yüksek çözünürlük (tarayıcı belleğini korumak için).
export const COVER_MAX_PIXELS = 100_000_000
// Sunucunun kabul ettiği en büyük dosya (backend imageproc.MaxInputBytes).
export const COVER_UPLOAD_MAX_BYTES = 2 * 1024 * 1024
// Kırpma için tutulan çalışma kopyasının uzun kenarı. Daha büyük fotoğraf seçimden hemen sonra
// bu boyuta küçültülür; yakınlaştırma 2 kata kadar çıktı yine 1600 piksel genişlikte kalır.
export const COVER_WORK_MAX_EDGE = 3200
export const COVER_ASPECT = 4 / 3
export const COVER_MAX_WIDTH = 1600
export const COVER_MIN_WIDTH = 64
export const COVER_MIN_HEIGHT = 48
export const COVER_JPEG_QUALITY = 0.85
// Çıktı 2 MiB'ı aşarsa sırayla denenen düşük kaliteler.
const FALLBACK_QUALITIES = [0.75, 0.65, 0.5]
const WORK_JPEG_QUALITY = 0.92
const MAX_ZOOM = 4

// Kırpma biçimi: oran, çıktı genişliğinin üst sınırı, genişliğin yuvarlandığı adım (oran tam
// kalsın diye) ve kaynakta kabul edilen en küçük boyut. Sunucudaki imageproc biçimleriyle aynıdır.
export type CropShape = { aspect: number; maxWidth: number; widthStep: number; minWidth: number; minHeight: number }
export const COVER_SHAPE: CropShape = {
  aspect: COVER_ASPECT,
  maxWidth: COVER_MAX_WIDTH,
  widthStep: 4,
  minWidth: COVER_MIN_WIDTH,
  minHeight: COVER_MIN_HEIGHT,
}
// Profil fotoğrafı: kare, en çok 512x512, en az 128 piksel (backend imageproc.AvatarMaxSize/AvatarMinSize).
export const AVATAR_SHAPE: CropShape = { aspect: 1, maxWidth: 512, widthStep: 1, minWidth: 128, minHeight: 128 }

const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/webp']
export const COVER_ACCEPT = ACCEPTED_TYPES.join(',')

export type CoverFileError = 'tooLarge' | 'type'
// Görsel hazırlanırken çıkan hatalar; her biri cover.error.<kod> metnine karşılık gelir.
export type CoverImageErrorCode = 'heic' | 'read' | 'dimensions' | 'outputTooLarge'

export class CoverImageError extends Error {
  readonly code: CoverImageErrorCode
  constructor(code: CoverImageErrorCode) {
    super(code)
    this.name = 'CoverImageError'
    this.code = code
  }
}

type FileInfo = { size: number; type: string; name: string }

const extension = (name: string) => (name.includes('.') ? (name.toLowerCase().split('.').pop() ?? '') : '')

export function isHeicFile(file: FileInfo): boolean {
  const ext = extension(file.name)
  return /hei[cf]/.test(file.type) || ext === 'heic' || ext === 'heif'
}

// Dosyayı okumadan önce boyut ve tür denetimi. Tarayıcı türü bildirmediyse uzantıya bakılır.
// HEIC/HEIF reddedilmez: tarayıcı açabiliyorsa (ör. Safari) JPEG'e çevrilip gönderilir;
// açamıyorsa prepareImage anlaşılır bir 'heic' hatası verir.
export function validateCoverFile(file: FileInfo): CoverFileError | null {
  const ext = extension(file.name)
  const typeOk = file.type ? ACCEPTED_TYPES.includes(file.type) : ['jpg', 'jpeg', 'png', 'webp'].includes(ext)
  if (!typeOk && !isHeicFile(file)) return 'type'
  if (file.size > COVER_MAX_BYTES) return 'tooLarge'
  return null
}

// Çalışma kopyasının boyutu: uzun kenar en çok COVER_WORK_MAX_EDGE, oran korunur. Küçültme
// gerekmiyorsa null.
export function workingSize(width: number, height: number): { width: number; height: number } | null {
  const edge = Math.max(width, height)
  if (edge <= COVER_WORK_MAX_EDGE) return null
  const scale = COVER_WORK_MAX_EDGE / edge
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) }
}

// Kırpma durumu: çerçevenin görseldeki merkezi (görsel pikseli) ve yakınlaştırma (1 = çerçeve
// görsele tam oturur).
export type CropState = { cx: number; cy: number; zoom: number }
export type CropRect = { sx: number; sy: number; sw: number; sh: number }

// Yakınlaştırma 1 iken görünen alan: görsele sığan, biçimin oranındaki en büyük dikdörtgen.
function baseView(iw: number, ih: number, shape: CropShape) {
  const a = shape.aspect
  return iw / ih > a ? { w: ih * a, h: ih } : { w: iw, h: iw / a }
}

// Görünen alan en küçük çıktı genişliğinin altına inmesin diye üst sınır görsele göre kısılır.
export function maxZoom(iw: number, ih: number, shape: CropShape = COVER_SHAPE): number {
  return Math.max(1, Math.min(MAX_ZOOM, baseView(iw, ih, shape).w / shape.minWidth))
}

export function initialCrop(iw: number, ih: number): CropState {
  return { cx: iw / 2, cy: ih / 2, zoom: 1 }
}

// Yakınlaştırmayı sınırlar ve çerçeveyi görselin içinde tutar.
export function clampCrop(state: CropState, iw: number, ih: number, shape: CropShape = COVER_SHAPE): CropState {
  const zoom = Math.min(maxZoom(iw, ih, shape), Math.max(1, state.zoom))
  const base = baseView(iw, ih, shape)
  const w = base.w / zoom
  const h = base.h / zoom
  return {
    zoom,
    cx: Math.min(iw - w / 2, Math.max(w / 2, state.cx)),
    cy: Math.min(ih - h / 2, Math.max(h / 2, state.cy)),
  }
}

// Çerçevenin görseldeki karşılığı (kaynak dikdörtgen).
export function cropRect(state: CropState, iw: number, ih: number, shape: CropShape = COVER_SHAPE): CropRect {
  const s = clampCrop(state, iw, ih, shape)
  const base = baseView(iw, ih, shape)
  const sw = base.w / s.zoom
  const sh = base.h / s.zoom
  return { sx: s.cx - sw / 2, sy: s.cy - sh / 2, sw, sh }
}

// Çıktı boyutu: kaynak alandan büyük olmaz (büyütme yok), en çok biçimin üst sınırı, oran tam
// (kapakta en çok 1600x1200 ve 4:3, profilde en çok 512x512 kare).
export function outputSize(sourceWidth: number, shape: CropShape = COVER_SHAPE): { width: number; height: number } {
  const step = shape.widthStep
  const width = Math.max(step, Math.floor(Math.min(shape.maxWidth, sourceWidth) / step) * step)
  return { width, height: Math.round(width / shape.aspect) }
}

export function isTooSmall(iw: number, ih: number, shape: CropShape = COVER_SHAPE): boolean {
  return iw < shape.minWidth || ih < shape.minHeight
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
// DOM'a eklenmemiş <img> için tarayıcılar yalnızca başlığı okur; pikseller çizilene kadar
// çözülmez. Bu yüzden boyutu öğrenmek ucuzdur.
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

function canvasToBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) =>
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('encode'))), 'image/jpeg', quality),
  )
}

function whiteCanvas(width: number, height: number): { canvas: HTMLCanvasElement; ctx: CanvasRenderingContext2D } {
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('canvas')
  ctx.fillStyle = 'white'
  ctx.fillRect(0, 0, width, height)
  ctx.imageSmoothingEnabled = true
  ctx.imageSmoothingQuality = 'high'
  return { canvas, ctx }
}

// Büyük fotoğrafı küçültülmüş olarak çözer. createImageBitmap'in resize seçenekleri tarayıcının
// görseli doğrudan hedef boyutta çözmesine izin verir (JPEG'de tam çözünürlüklü bit eşlem hiç
// oluşmayabilir). Yön önce uygulanır; tarayıcı boyutu farklı yorumladıysa sonuç kullanılmaz.
async function downscaledBitmap(file: Blob, size: { width: number; height: number }): Promise<ImageBitmap | null> {
  if (typeof createImageBitmap !== 'function') return null
  try {
    const bitmap = await createImageBitmap(file, {
      imageOrientation: 'from-image',
      resizeWidth: size.width,
      resizeHeight: size.height,
      resizeQuality: 'high',
    })
    if (bitmap.width === size.width && bitmap.height === size.height) return bitmap
    bitmap.close()
  } catch {
    // Desteklenmeyen seçenek ya da tür: aşağıdaki yönteme düşülür.
  }
  return null
}

// Seçilen dosyayı kırpmaya hazırlar. Küçük görsel olduğu gibi kullanılır; büyük görsel
// (uzun kenar > COVER_WORK_MAX_EDGE) bir kez küçültülüp JPEG çalışma kopyasına çevrilir, böylece
// önizleme ve son çizim tam çözünürlüklü fotoğrafı bellekte tutmaz. Çözülemeyen dosyada
// CoverImageError ('heic', 'read' ya da 'dimensions') fırlatır.
export async function prepareImage(file: File): Promise<LoadedImage> {
  let probe: LoadedImage
  try {
    probe = await loadImage(file)
  } catch {
    throw new CoverImageError(isHeicFile(file) ? 'heic' : 'read')
  }
  const target = workingSize(probe.width, probe.height)
  if (!target) return probe
  if (probe.width * probe.height > COVER_MAX_PIXELS) {
    probe.release()
    throw new CoverImageError('dimensions')
  }
  try {
    const bitmap = await downscaledBitmap(file, target)
    const { canvas, ctx } = whiteCanvas(target.width, target.height)
    try {
      // Yedek yol (eski tarayıcı): <img> tam çözünürlükte çözülüp küçültülerek çizilir.
      ctx.drawImage(bitmap ?? probe.element, 0, 0, target.width, target.height)
    } finally {
      bitmap?.close()
    }
    const blob = await canvasToBlob(canvas, WORK_JPEG_QUALITY)
    canvas.width = canvas.height = 0 // canvas belleğini hemen bırak
    return await loadImage(blob)
  } catch {
    throw new CoverImageError(isHeicFile(file) ? 'heic' : 'read')
  } finally {
    probe.release()
  }
}

// Seçilen alanı canvas'a çizer ve biçimin boyutunda (kapakta en çok 1600x1200) JPEG blob üretir. Şeffaf alanlar beyaz
// zemine oturur. Sonuç sunucu sınırını (2 MiB) aşarsa kalite düşürülerek yeniden kodlanır;
// yine aşarsa CoverImageError('outputTooLarge').
export async function renderCrop(image: CanvasImageSource, rect: CropRect, shape: CropShape = COVER_SHAPE): Promise<Blob> {
  const { width, height } = outputSize(rect.sw, shape)
  const { canvas, ctx } = whiteCanvas(width, height)
  ctx.drawImage(image, rect.sx, rect.sy, rect.sw, rect.sh, 0, 0, width, height)
  for (const quality of [COVER_JPEG_QUALITY, ...FALLBACK_QUALITIES]) {
    const blob = await canvasToBlob(canvas, quality)
    if (blob.size <= COVER_UPLOAD_MAX_BYTES) return blob
  }
  throw new CoverImageError('outputTooLarge')
}
