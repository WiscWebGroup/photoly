// Image preparation that matches Photoly's server (Pillow) closely enough that browser and
// server vectors are interchangeable: decode without colour management, EXIF orientation,
// white behind transparency, RGB, Pillow's antialiased bilinear resize to 224 x 224, then
// SigLIP's normalisation to [-1, 1], channel-first. Ported from poc/single-binary/web/preprocess.js.

export const SIZE = 224
export interface RgbImage {
  rgb: Uint8Array
  width: number
  height: number
}

export async function decodeRGB(blob: Blob): Promise<RgbImage> {
  const bitmap = await createImageBitmap(blob, {
    imageOrientation: 'from-image',
    colorSpaceConversion: 'none',
    premultiplyAlpha: 'none',
  })
  const { width, height } = bitmap
  const canvas = new OffscreenCanvas(width, height)
  const ctx = canvas.getContext('2d', { willReadFrequently: true })!
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, width, height)
  ctx.drawImage(bitmap, 0, 0)
  bitmap.close()
  const rgba = ctx.getImageData(0, 0, width, height).data
  const rgb = new Uint8Array(width * height * 3)
  for (let i = 0, j = 0; i < rgba.length; i += 4, j += 3) {
    rgb[j] = rgba[i]!
    rgb[j + 1] = rgba[i + 1]!
    rgb[j + 2] = rgba[i + 2]!
  }
  return { rgb, width, height }
}

const PRECISION = 1 << 22

// Pillow's resampling coefficients (precompute_coeffs) for the bilinear (triangle) filter:
// when shrinking, the filter widens with the scale, so it antialiases.
function coefficients(inSize: number, outSize: number) {
  const scale = inSize / outSize
  const filterScale = Math.max(scale, 1)
  const support = filterScale
  const taps: { min: number; weights: number[] }[] = []
  for (let out = 0; out < outSize; out++) {
    const center = (out + 0.5) * scale
    const min = Math.max(Math.trunc(center - support + 0.5), 0)
    const max = Math.min(Math.trunc(center + support + 0.5), inSize)
    const raw: number[] = []
    let total = 0
    for (let x = min; x < max; x++) {
      const w = Math.max(0, 1 - Math.abs((x - center + 0.5) / filterScale))
      raw.push(w)
      total += w
    }
    taps.push({ min, weights: raw.map((w) => Math.round((total ? w / total : 0) * PRECISION)) })
  }
  return taps
}

// Pillow starts each sum at half a unit and shifts right, i.e. rounds half up.
const clip8 = (sum: number) => {
  const v = Math.floor((sum + PRECISION / 2) / PRECISION)
  return v <= 0 ? 0 : v >= 255 ? 255 : v
}

/** Horizontal pass, then vertical, rounding to 8 bits after each pass as Pillow does. */
export function resizeBilinear(
  { rgb, width, height }: RgbImage,
  outW: number,
  outH: number,
): Uint8Array {
  const xs = coefficients(width, outW)
  const ys = coefficients(height, outH)
  const mid = new Uint8Array(outW * height * 3)
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < outW; x++) {
      const { min, weights } = xs[x]!
      let r = 0,
        g = 0,
        b = 0
      for (let k = 0; k < weights.length; k++) {
        const i = (y * width + min + k) * 3
        r += rgb[i]! * weights[k]!
        g += rgb[i + 1]! * weights[k]!
        b += rgb[i + 2]! * weights[k]!
      }
      const o = (y * outW + x) * 3
      mid[o] = clip8(r)
      mid[o + 1] = clip8(g)
      mid[o + 2] = clip8(b)
    }
  }
  const out = new Uint8Array(outW * outH * 3)
  for (let y = 0; y < outH; y++) {
    const { min, weights } = ys[y]!
    for (let x = 0; x < outW; x++) {
      let r = 0,
        g = 0,
        b = 0
      for (let k = 0; k < weights.length; k++) {
        const i = ((min + k) * outW + x) * 3
        r += mid[i]! * weights[k]!
        g += mid[i + 1]! * weights[k]!
        b += mid[i + 2]! * weights[k]!
      }
      const o = (y * outW + x) * 3
      out[o] = clip8(r)
      out[o + 1] = clip8(g)
      out[o + 2] = clip8(b)
    }
  }
  return out
}

/** Normalises 224 x 224 RGB bytes to a channel-first Float32Array in [-1, 1]. */
export function normalize(small: Uint8Array): Float32Array {
  const plane = SIZE * SIZE
  const pixels = new Float32Array(3 * plane)
  for (let i = 0; i < plane; i++)
    for (let c = 0; c < 3; c++) pixels[c * plane + i] = (small[i * 3 + c]! / 255 - 0.5) / 0.5
  return pixels
}

/** Float32Array pixel_values for a [1, 3, 224, 224] tensor. */
export async function prepareImage(blob: Blob): Promise<Float32Array> {
  return normalize(resizeBilinear(await decodeRGB(blob), SIZE, SIZE))
}
