/** A crop box as fractions (0..1) of the rotated image, so it survives preview resizing. */
export interface Crop {
  x: number
  y: number
  w: number
  h: number
}

export type Rotation = 0 | 90 | 180 | 270

export const FULL_CROP: Crop = { x: 0, y: 0, w: 1, h: 1 }

/** Formats the editor can decode and re-encode in the browser. */
const EDITABLE = ['jpg', 'jpeg', 'png', 'webp']
export const isEditable = (format = '') => EDITABLE.includes(format.toLowerCase())

export interface Encoding {
  mime: 'image/jpeg' | 'image/png' | 'image/webp'
  ext: 'jpg' | 'png' | 'webp'
}

/**
 * Keeps the source format where possible. PNG has no quality setting, so a size limit
 * (or any lossy request) turns it into a JPEG.
 */
export function chooseEncoding(format = '', lossy = false): Encoding {
  const f = format.toLowerCase()
  if (f === 'webp') return { mime: 'image/webp', ext: 'webp' }
  if (f === 'png' && !lossy) return { mime: 'image/png', ext: 'png' }
  return { mime: 'image/jpeg', ext: 'jpg' }
}

export const rotateCrop = (c: Crop, clockwise: boolean): Crop =>
  clockwise
    ? { x: 1 - c.y - c.h, y: c.x, w: c.h, h: c.w }
    : { x: c.y, y: 1 - c.x - c.w, w: c.h, h: c.w }

/** Pixel size of the image once rotated (90/270 swap width and height). */
export const rotatedSize = (width: number, height: number, rotation: Rotation) =>
  rotation % 180 === 0 ? { width, height } : { width: height, height: width }

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n))

/** Keeps a crop box inside the image and at least `min` (fraction) on each side. */
export function clampCrop(c: Crop, min = 0.02): Crop {
  const w = clamp(c.w, min, 1)
  const h = clamp(c.h, min, 1)
  return { x: clamp(c.x, 0, 1 - w), y: clamp(c.y, 0, 1 - h), w, h }
}

/** The largest centred crop with aspect `ratio` (width / height) for an image of the given size. */
export function cropForRatio(ratio: number, width: number, height: number): Crop {
  const w = Math.min(1, (ratio * height) / width)
  const h = Math.min(1, width / ratio / height)
  return { x: (1 - w) / 2, y: (1 - h) / 2, w, h }
}

function canvasOf(width: number, height: number) {
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(width))
  canvas.height = Math.max(1, Math.round(height))
  return canvas
}

/** Draws `source` rotated by `rotation` degrees clockwise onto a new canvas. */
export function rotateToCanvas(
  source: CanvasImageSource,
  sourceWidth: number,
  sourceHeight: number,
  rotation: Rotation,
) {
  const { width, height } = rotatedSize(sourceWidth, sourceHeight, rotation)
  const canvas = canvasOf(width, height)
  const ctx = canvas.getContext('2d')!
  ctx.translate(canvas.width / 2, canvas.height / 2)
  ctx.rotate((rotation * Math.PI) / 180)
  ctx.drawImage(source, -sourceWidth / 2, -sourceHeight / 2)
  return canvas
}

/** Copies the `crop` region of `source` into a new canvas, scaled by `scale`. */
export function cropToCanvas(source: HTMLCanvasElement, crop: Crop, scale = 1, opaque = false) {
  const sx = Math.round(crop.x * source.width)
  const sy = Math.round(crop.y * source.height)
  const sw = Math.max(1, Math.round(crop.w * source.width))
  const sh = Math.max(1, Math.round(crop.h * source.height))
  const canvas = canvasOf(sw * scale, sh * scale)
  const ctx = canvas.getContext('2d')!
  if (opaque) {
    // JPEG has no alpha: transparent pixels would turn black.
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
  }
  ctx.imageSmoothingQuality = 'high'
  ctx.drawImage(source, sx, sy, sw, sh, 0, 0, canvas.width, canvas.height)
  return canvas
}

export const toBlob = (canvas: HTMLCanvasElement, mime: string, quality?: number) =>
  new Promise<Blob>((resolve, reject) =>
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('Encoding failed'))), mime, quality),
  )

export interface Encoded {
  blob: Blob
  /** 0..1, absent for lossless output. */
  quality?: number
  width: number
  height: number
  /** True when the limit could only be met by shrinking the dimensions. */
  scaledDown: boolean
  /** True when even the smallest attempt stayed above the limit. */
  overLimit: boolean
}

export const DEFAULT_QUALITY = 0.92
const MIN_QUALITY = 0.1
const SEARCH_STEPS = 7
const MAX_SHRINKS = 8

/**
 * Encodes `canvas`. With `maxBytes`, finds the highest quality whose output fits; if even the
 * lowest quality is too big, shrinks the dimensions until it fits.
 */
export async function encodeToLimit(
  canvas: HTMLCanvasElement,
  { mime }: Encoding,
  maxBytes?: number,
): Promise<Encoded> {
  const result = (
    blob: Blob,
    c: HTMLCanvasElement,
    quality: number | undefined,
    scaledDown: boolean,
  ) => ({
    blob,
    quality,
    width: c.width,
    height: c.height,
    scaledDown,
    overLimit: maxBytes !== undefined && blob.size > maxBytes,
  })
  if (mime === 'image/png') return result(await toBlob(canvas, mime), canvas, undefined, false)
  if (!maxBytes)
    return result(await toBlob(canvas, mime, DEFAULT_QUALITY), canvas, DEFAULT_QUALITY, false)

  let current = canvas
  for (let shrink = 0; ; shrink++) {
    const best = await toBlob(current, mime, 1)
    if (best.size <= maxBytes) return result(best, current, 1, shrink > 0)

    const worst = await toBlob(current, mime, MIN_QUALITY)
    if (worst.size <= maxBytes) {
      let lo = MIN_QUALITY
      let hi = 1
      let fit = worst
      for (let i = 0; i < SEARCH_STEPS; i++) {
        const mid = (lo + hi) / 2
        const blob = await toBlob(current, mime, mid)
        if (blob.size <= maxBytes) {
          lo = mid
          fit = blob
        } else hi = mid
      }
      return result(fit, current, lo, shrink > 0)
    }
    if (shrink === MAX_SHRINKS || Math.min(current.width, current.height) <= 16) {
      return result(worst, current, MIN_QUALITY, shrink > 0)
    }
    // Size scales roughly with pixel count, so aim a little below the ratio we need.
    const factor = Math.min(0.9, Math.sqrt(maxBytes / worst.size) * 0.95)
    const next = canvasOf(current.width * factor, current.height * factor)
    const ctx = next.getContext('2d')!
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(current, 0, 0, next.width, next.height)
    current = next
  }
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}
