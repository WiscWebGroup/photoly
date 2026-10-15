import { beforeEach, describe, expect, it, vi } from 'vitest'
import { stubCanvas } from '@/test/canvas'
import {
  FULL_CROP,
  chooseEncoding,
  clampCrop,
  cropForRatio,
  cropToCanvas,
  encodeToLimit,
  isEditable,
  rotateCrop,
  rotateToCanvas,
  rotatedSize,
} from '../imageEdit'

const canvas = (width: number, height: number) =>
  Object.assign(document.createElement('canvas'), { width, height })

describe('imageEdit', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    stubCanvas()
  })

  it('only edits formats the browser can re-encode', () => {
    expect(['jpg', 'JPEG', 'png', 'webp'].every(isEditable)).toBe(true)
    expect(['gif', 'svg', 'mp4', ''].some(isEditable)).toBe(false)
  })

  it('keeps the format, except PNG when it has to be lossy', () => {
    expect(chooseEncoding('jpg').ext).toBe('jpg')
    expect(chooseEncoding('webp').mime).toBe('image/webp')
    expect(chooseEncoding('png').ext).toBe('png')
    expect(chooseEncoding('png', true).ext).toBe('jpg')
  })

  it('rotates a crop box with the image', () => {
    const box = { x: 0.1, y: 0.2, w: 0.3, h: 0.4 }
    const cw = rotateCrop(box, true)
    expect(cw.x).toBeCloseTo(0.4)
    expect(cw.y).toBeCloseTo(0.1)
    expect([cw.w, cw.h]).toEqual([0.4, 0.3])
    const back = rotateCrop(cw, false)
    expect(back.x).toBeCloseTo(box.x)
    expect(back.y).toBeCloseTo(box.y)
    expect(back.w).toBeCloseTo(box.w)
    expect(back.h).toBeCloseTo(box.h)
    expect(rotateCrop(rotateCrop(FULL_CROP, true), true)).toEqual(FULL_CROP)
  })

  it('swaps width and height for quarter turns', () => {
    expect(rotatedSize(400, 300, 90)).toEqual({ width: 300, height: 400 })
    expect(rotatedSize(400, 300, 180)).toEqual({ width: 400, height: 300 })
    const rotated = rotateToCanvas(document.createElement('canvas'), 400, 300, 270)
    expect([rotated.width, rotated.height]).toEqual([300, 400])
  })

  it('keeps a crop inside the image', () => {
    expect(clampCrop({ x: 0.9, y: -1, w: 0.5, h: 2 })).toEqual({ x: 0.5, y: 0, w: 0.5, h: 1 })
    expect(clampCrop({ x: 0.5, y: 0.5, w: 0, h: 0 }).w).toBe(0.02)
  })

  it('centres the biggest crop of a given aspect ratio', () => {
    const wide = cropForRatio(1, 400, 200)
    expect(wide).toEqual({ x: 0.25, y: 0, w: 0.5, h: 1 })
    const tall = cropForRatio(2, 200, 400)
    expect(tall).toEqual({ x: 0, y: 0.375, w: 1, h: 0.25 })
  })

  it('crops to the box in pixels', () => {
    const out = cropToCanvas(canvas(400, 200), { x: 0.25, y: 0, w: 0.5, h: 0.5 })
    expect([out.width, out.height]).toEqual([200, 100])
  })

  describe('encodeToLimit', () => {
    const jpeg = chooseEncoding('jpg', true)

    it('uses the default quality without a limit', async () => {
      const out = await encodeToLimit(canvas(100, 100), jpeg)
      expect(out.quality).toBe(0.92)
      expect(out.blob.size).toBe(9200)
    })

    it('does not touch PNG output', async () => {
      const out = await encodeToLimit(canvas(100, 100), chooseEncoding('png'), 10)
      expect(out.quality).toBeUndefined()
      expect(out.blob.size).toBe(10000)
    })

    it('keeps full quality when it already fits', async () => {
      const out = await encodeToLimit(canvas(100, 100), jpeg, 20000)
      expect(out).toMatchObject({ quality: 1, scaledDown: false, overLimit: false })
    })

    it('finds the highest quality under the limit', async () => {
      const out = await encodeToLimit(canvas(100, 100), jpeg, 5000)
      expect(out.blob.size).toBeLessThanOrEqual(5000)
      expect(out.blob.size).toBeGreaterThan(4800)
      expect(out.quality).toBeCloseTo(0.5, 1)
      expect([out.width, out.height]).toEqual([100, 100])
      expect(out.scaledDown).toBe(false)
    })

    it('shrinks the image when quality alone cannot reach the limit', async () => {
      const out = await encodeToLimit(canvas(100, 100), jpeg, 200)
      expect(out.blob.size).toBeLessThanOrEqual(200)
      expect(out.width).toBeLessThan(100)
      expect(out).toMatchObject({ scaledDown: true, overLimit: false })
    })

    it('reports a limit it cannot meet', async () => {
      const out = await encodeToLimit(canvas(10, 10), jpeg, 1)
      expect(out.overLimit).toBe(true)
    })
  })
})
