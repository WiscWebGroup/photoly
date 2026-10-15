import { vi } from 'vitest'

/**
 * jsdom has no canvas, so give it a 2D context that ignores drawing and an encoder whose output
 * size is `width * height * quality` bytes (PNG, with no quality, counts as quality 1).
 */
export function stubCanvas() {
  const ctx = new Proxy({}, { get: () => () => undefined, set: () => true })
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(ctx as never)
  vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(function (
    this: HTMLCanvasElement,
    callback,
    type,
    quality,
  ) {
    callback(
      new Blob([new Uint8Array(Math.round(this.width * this.height * (quality ?? 1)))], { type }),
    )
  })
}

/** `new Image()` that "decodes" instantly to the given size. */
export function stubImage(naturalWidth = 400, naturalHeight = 300) {
  vi.stubGlobal(
    'Image',
    class {
      src = ''
      naturalWidth = naturalWidth
      naturalHeight = naturalHeight
      decode = () => Promise.resolve()
    },
  )
  URL.createObjectURL = vi.fn(() => 'blob:test')
  URL.revokeObjectURL = vi.fn()
}
