import { useEffect, useId, useRef, useState, type DragEvent, type KeyboardEvent, type PointerEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ImagePlus, ZoomIn, ZoomOut } from 'lucide-react'

import { FormError } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  COVER_ACCEPT,
  clampCrop,
  cropRect,
  initialCrop,
  isTooSmall,
  loadImage,
  maxZoom,
  renderCrop,
  validateCoverFile,
  type CropState,
  type LoadedImage,
} from '@/lib/coverCrop'
import { cn } from '@/lib/utils'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  // Kırpılmış 4:3 JPEG (en çok 1600x1200). Yüklemeyi çağıran yapar; başarıda diyaloğu kapatır.
  onCropped: (image: Blob) => void
  // Onay düğmesinin metni ("Fotoğrafı kaydet", "Bu fotoğrafı kullan").
  confirmLabel: string
  // Çağıranın yükleme durumu ve hatası.
  pending?: boolean
  error?: string | null
}

// Kapak fotoğrafı seçme ve 4:3 kırpma. Bağımlılıksız: çerçeve sabit, görsel sürüklenir ve
// yakınlaştırılır (fare, dokunma, tekerlek, iki parmak, klavye, kaydırıcı); çıktı canvas ile üretilir.
export function CoverCropDialog({ open, onOpenChange, onCropped, confirmLabel, pending = false, error = null }: Props) {
  const { t } = useTranslation()
  // Görsel hazırlanırken ya da yüklenirken diyalog kapatılamaz.
  const [busy, setBusy] = useState(false)
  const working = busy || pending
  return (
    <Dialog open={open} onOpenChange={(next) => !working && onOpenChange(next)}>
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>{t('cover.dialogTitle')}</DialogTitle>
          <DialogDescription>{t('cover.dialogBody')}</DialogDescription>
        </DialogHeader>
        {/* İçerik yalnızca diyalog açıkken bağlıdır; her açılışta seçim sıfırdan başlar. */}
        <CropFlow
          onCancel={() => onOpenChange(false)}
          onCropped={onCropped}
          confirmLabel={confirmLabel}
          pending={pending}
          error={error}
          busy={busy}
          setBusy={setBusy}
        />
      </DialogContent>
    </Dialog>
  )
}

function CropFlow({
  onCancel,
  onCropped,
  confirmLabel,
  pending,
  error,
  busy,
  setBusy,
}: Pick<Props, 'onCropped' | 'confirmLabel'> & {
  onCancel: () => void
  pending: boolean
  error: string | null
  busy: boolean
  setBusy: (busy: boolean) => void
}) {
  const { t } = useTranslation()
  const [image, setImage] = useState<LoadedImage | null>(null)
  const [crop, setCrop] = useState<CropState>({ cx: 0, cy: 0, zoom: 1 })
  const [localError, setLocalError] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const working = busy || pending

  // Görsel değişince ve diyalog kapanınca object URL serbest bırakılır.
  useEffect(() => () => image?.release(), [image])

  const pick = async (file: File | undefined) => {
    if (!file) return
    const invalid = validateCoverFile(file)
    if (invalid) {
      setLocalError(t(`cover.error.${invalid}`))
      return
    }
    setBusy(true)
    try {
      const loaded = await loadImage(file)
      if (isTooSmall(loaded.width, loaded.height)) {
        loaded.release()
        setLocalError(t('cover.error.tooSmall'))
        return
      }
      setLocalError(null)
      setCrop(initialCrop(loaded.width, loaded.height))
      setImage(loaded)
    } catch {
      setLocalError(t('cover.error.read'))
    } finally {
      setBusy(false)
    }
  }

  const confirm = async () => {
    if (!image || working) return
    setBusy(true)
    try {
      onCropped(await renderCrop(image.element, cropRect(crop, image.width, image.height)))
    } catch {
      setLocalError(t('cover.error.read'))
    } finally {
      setBusy(false)
    }
  }

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    void pick(e.dataTransfer.files[0])
  }

  return (
    <div className="flex flex-col gap-4">
      <FormError message={localError ?? error} />
      <input
        ref={fileRef}
        type="file"
        accept={COVER_ACCEPT}
        className="sr-only"
        tabIndex={-1}
        aria-label={t('cover.pick')}
        onChange={(e) => {
          void pick(e.target.files?.[0])
          e.target.value = '' // aynı dosya yeniden seçilebilsin
        }}
      />
      {image ? (
        <>
          <Cropper image={image} crop={crop} onChange={setCrop} disabled={working} />
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center">
            <Button variant="ghost" className="sm:mr-auto" onClick={() => fileRef.current?.click()} disabled={working}>
              {t('cover.pickAnother')}
            </Button>
            <Button variant="outline" onClick={onCancel} disabled={working}>
              {t('manage.cancel')}
            </Button>
            <Button onClick={() => void confirm()} disabled={working} aria-busy={working}>
              {pending ? t('cover.uploading') : busy ? t('cover.preparing') : confirmLabel}
            </Button>
          </div>
        </>
      ) : (
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          onDragOver={(e) => {
            e.preventDefault()
            setDragOver(true)
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
          disabled={working}
          className={cn(
            'flex aspect-[4/3] w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-[16px] border border-dashed border-control bg-surface px-4 text-center transition-colors hover:border-accent disabled:cursor-default disabled:opacity-60',
            dragOver && 'border-accent bg-soft',
          )}
        >
          <ImagePlus className="size-7 text-muted" aria-hidden />
          <span className="text-[15px] font-medium">{busy ? t('cover.preparing') : t('cover.pick')}</span>
          <span className="text-xs text-muted">{t('cover.pickHint')}</span>
        </button>
      )}
      <p role="status" className="sr-only">
        {pending ? t('cover.uploading') : ''}
      </p>
    </div>
  )
}

const KEY_STEP = 0.05
const KEY_STEP_LARGE = 0.2
const ZOOM_STEP = 1.1

// Sabit 4:3 çerçeve. Görsel çerçevenin arkasında yüzde değerleriyle konumlanır; çerçevede
// görünen alan, kaydedilecek alanın kendisidir (önizleme).
function Cropper({
  image,
  crop,
  onChange,
  disabled,
}: {
  image: LoadedImage
  crop: CropState
  onChange: (next: CropState | ((prev: CropState) => CropState)) => void
  disabled: boolean
}) {
  const { t } = useTranslation()
  const hintId = useId()
  const frameRef = useRef<HTMLDivElement>(null)
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const { width: iw, height: ih } = image
  const rect = cropRect(crop, iw, ih)
  const zoomLimit = maxZoom(iw, ih)

  const update = (change: (prev: CropState) => CropState) => onChange((prev) => clampCrop(change(prev), iw, ih))
  const zoomBy = (factor: number) => update((s) => ({ ...s, zoom: s.zoom * factor }))

  // Tekerlek: React wheel dinleyicisi pasif olduğu için sayfa kaymasını engellemek üzere yerel dinleyici.
  useEffect(() => {
    const frame = frameRef.current
    if (!frame || disabled) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      onChange((prev) => clampCrop({ ...prev, zoom: prev.zoom * Math.exp(-e.deltaY * 0.0015) }, iw, ih))
    }
    frame.addEventListener('wheel', onWheel, { passive: false })
    return () => frame.removeEventListener('wheel', onWheel)
  }, [disabled, iw, ih, onChange])

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (disabled) return
    e.currentTarget.setPointerCapture?.(e.pointerId)
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
  }
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const prev = pointers.current.get(e.pointerId)
    if (!prev || disabled) return
    const frameWidth = e.currentTarget.getBoundingClientRect().width || 1
    const next = { x: e.clientX, y: e.clientY }
    if (pointers.current.size >= 2) {
      // İki parmak: parmaklar arası mesafenin oranı kadar yakınlaştır.
      const other = [...pointers.current.entries()].find(([id]) => id !== e.pointerId)?.[1]
      if (other) {
        const before = Math.hypot(prev.x - other.x, prev.y - other.y)
        const after = Math.hypot(next.x - other.x, next.y - other.y)
        if (before > 0 && after > 0) zoomBy(after / before)
      }
    } else {
      // Sürükleme: görsel parmağı izler; çerçevenin merkezi ters yönde kayar.
      const dx = next.x - prev.x
      const dy = next.y - prev.y
      update((s) => {
        const scale = cropRect(s, iw, ih).sw / frameWidth
        return { ...s, cx: s.cx - dx * scale, cy: s.cy - dy * scale }
      })
    }
    pointers.current.set(e.pointerId, next)
  }
  const onPointerEnd = (e: PointerEvent<HTMLDivElement>) => {
    pointers.current.delete(e.pointerId)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (disabled) return
    const step = e.shiftKey ? KEY_STEP_LARGE : KEY_STEP
    const move = (dx: number, dy: number) =>
      update((s) => {
        const r = cropRect(s, iw, ih)
        return { ...s, cx: s.cx + dx * r.sw * step, cy: s.cy + dy * r.sh * step }
      })
    switch (e.key) {
      case 'ArrowLeft':
        move(-1, 0)
        break
      case 'ArrowRight':
        move(1, 0)
        break
      case 'ArrowUp':
        move(0, -1)
        break
      case 'ArrowDown':
        move(0, 1)
        break
      case '+':
      case '=':
        zoomBy(ZOOM_STEP)
        break
      case '-':
      case '_':
        zoomBy(1 / ZOOM_STEP)
        break
      default:
        return
    }
    e.preventDefault()
  }

  const percent = Math.round(crop.zoom * 100)
  return (
    <div className="flex flex-col gap-3">
      <div
        ref={frameRef}
        role="group"
        tabIndex={0}
        aria-label={t('cover.cropArea')}
        aria-describedby={hintId}
        data-crop={`${Math.round(rect.sx)},${Math.round(rect.sy)},${Math.round(rect.sw)},${Math.round(rect.sh)}`}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerEnd}
        onPointerCancel={onPointerEnd}
        onKeyDown={onKeyDown}
        className={cn(
          'relative aspect-[4/3] w-full touch-none overflow-hidden rounded-[16px] bg-surface select-none',
          disabled ? 'opacity-70' : 'cursor-grab active:cursor-grabbing',
        )}
      >
        <img
          src={image.url}
          alt={t('cover.previewAlt')}
          draggable={false}
          className="pointer-events-none absolute max-w-none"
          style={{
            width: `${(iw / rect.sw) * 100}%`,
            left: `${(-rect.sx / rect.sw) * 100}%`,
            top: `${(-rect.sy / rect.sh) * 100}%`,
          }}
        />
      </div>
      <div className="flex items-center gap-3">
        <ZoomOut className="size-4 shrink-0 text-muted" aria-hidden />
        <input
          type="range"
          min={1}
          max={zoomLimit}
          step={0.01}
          value={crop.zoom}
          disabled={disabled || zoomLimit <= 1}
          aria-label={t('cover.zoom')}
          aria-valuetext={t('cover.zoomValue', { percent })}
          onChange={(e) => update((s) => ({ ...s, zoom: Number(e.target.value) }))}
          className="h-6 min-w-0 flex-1 cursor-pointer accent-primary disabled:cursor-default disabled:opacity-60"
        />
        <ZoomIn className="size-4 shrink-0 text-muted" aria-hidden />
        <span className="num w-11 shrink-0 text-right text-xs text-muted" aria-hidden>
          {t('cover.zoomValue', { percent })}
        </span>
      </div>
      <p id={hintId} className="text-xs text-muted">
        {t('cover.cropHint')}
      </p>
    </div>
  )
}
