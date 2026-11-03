import { describe, expect, it } from 'vitest'
import { layoutBubbles, pinColor, pinColors } from '../notePins'
describe('pin presentation', () => {
  it('cycles stable colors using identity', () => {
    expect(new Set([1, 2, 3, 4, 5, 6].map(pinColor)).size).toBe(6)
    expect(pinColor(7)).toBe(pinColor(1))
    expect(pinColors).toHaveLength(6)
  })
  it('keeps corner bubbles within the visible photo', () => {
    const bounds = { x: 6, y: 6, width: 788, height: 588 }
    const bubbles = layoutBubbles(
      [
        { id: 1, x: 0, y: 0 },
        { id: 2, x: 800, y: 0 },
        { id: 3, x: 0, y: 600 },
        { id: 4, x: 800, y: 600 },
      ],
      bounds,
    )
    for (const b of bubbles) {
      expect(b.x).toBeGreaterThanOrEqual(bounds.x)
      expect(b.y).toBeGreaterThanOrEqual(bounds.y)
      expect(b.x + b.width).toBeLessThanOrEqual(bounds.x + bounds.width)
      expect(b.y + b.height).toBeLessThanOrEqual(bounds.y + bounds.height)
    }
  })
  it('separates nearby bubbles when there is room', () => {
    const bubbles = layoutBubbles(
      [1, 2, 3].map((id) => ({ id, x: 600, y: 400 })),
      { x: 0, y: 0, width: 1200, height: 800 },
    )
    for (const a of bubbles)
      for (const b of bubbles) {
        if (a.id >= b.id) continue
        expect(
          a.x + a.width <= b.x ||
            b.x + b.width <= a.x ||
            a.y + a.height <= b.y ||
            b.y + b.height <= a.y,
        ).toBe(true)
      }
  })
  it('fits a small photo without negative dimensions or overflow', () => {
    const [b] = layoutBubbles([{ id: 1, x: 10, y: 10 }], { x: 6, y: 6, width: 90, height: 80 })
    expect(b).toMatchObject({ x: 6, y: 6, width: 90, height: 80 })
  })
})
