export const pinColors = ['#22d3ee', '#60a5fa', '#c084fc', '#f472b6', '#fbbf24', '#4ade80']
export function pinColor(id: number) {
  return pinColors[(id - 1) % pinColors.length]!
}
export interface PinPoint {
  id: number
  number: number
  body: string
  x: number
  y: number
  color: string
}
export interface Rect {
  x: number
  y: number
  width: number
  height: number
}
const clamp = (v: number, min: number, max: number) => Math.max(min, Math.min(max, v))
const overlap = (a: Rect, b: Rect) =>
  Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x)) *
  Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y))
/** Prefer above, right, below, left; slide along edges to make room for earlier bubbles. */
export function layoutBubbles(
  points: { id: number; x: number; y: number; height?: number }[],
  bounds: Rect,
) {
  const placed: Rect[] = []
  const width = Math.min(240, bounds.width)
  return points.map((p) => {
    const height = Math.min(p.height ?? 136, bounds.height)
    const candidates = [
      { x: p.x - width / 2, y: p.y - height - 24 },
      { x: p.x + 24, y: p.y - height / 2 },
      { x: p.x - width / 2, y: p.y + 24 },
      { x: p.x - width - 24, y: p.y - height / 2 },
    ]
    for (const r of placed) {
      candidates.push(
        { x: r.x + r.width + 8, y: r.y },
        { x: r.x - width - 8, y: r.y },
        { x: r.x, y: r.y + r.height + 8 },
        { x: r.x, y: r.y - height - 8 },
      )
    }
    const ranked = candidates
      .map((c, i) => {
        const rect = {
          x: clamp(c.x, bounds.x, bounds.x + bounds.width - width),
          y: clamp(c.y, bounds.y, bounds.y + bounds.height - height),
          width,
          height,
        }
        const collisions =
          placed.reduce((sum, r) => sum + overlap(rect, r), 0) +
          points.reduce(
            (sum, point) =>
              sum +
              overlap(rect, {
                x: point.x - 20,
                y: point.y - 20,
                width: 40,
                height: 40,
              }) *
                (point.id === p.id ? 10 : 2),
            0,
          )
        // Keep all bubbles within bounds, then prioritize separation over proximity.
        const distance = Math.hypot(rect.x + width / 2 - p.x, rect.y + height / 2 - p.y)
        const shifted = Math.hypot(rect.x - c.x, rect.y - c.y)
        return { rect, score: collisions * 1000 + shifted + distance * 0.1 + i * 0.01 }
      })
      .sort((a, b) => a.score - b.score)
    const rect = ranked[0]!.rect
    placed.push(rect)
    return { ...rect, id: p.id, anchorX: p.x, anchorY: p.y }
  })
}
