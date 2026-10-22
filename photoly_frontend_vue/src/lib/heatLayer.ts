import L from 'leaflet'

/** Low to high density: teal, the brand green, amber, then red. */
const STOPS: [number, string][] = [
  [0.15, '#5eead4'],
  [0.4, '#16827d'],
  [0.7, '#f59e0b'],
  [1, '#dc2626'],
]
export const heatGradient = `linear-gradient(to right, ${STOPS.map(([at, c]) => `${c} ${at * 100}%`).join(', ')})`

let colors: Uint8ClampedArray | undefined
/** 256 RGBA entries, indexed by density. */
function palette() {
  if (colors) return colors
  const c = document.createElement('canvas')
  c.width = 256
  c.height = 1
  const ctx = c.getContext('2d')!
  const g = ctx.createLinearGradient(0, 0, 256, 0)
  for (const [at, color] of STOPS) g.addColorStop(at, color)
  ctx.fillStyle = g
  ctx.fillRect(0, 0, 256, 1)
  return (colors = ctx.getImageData(0, 0, 256, 1).data)
}

/**
 * A density map of points on its own canvas: each point adds a soft blob, and the summed
 * darkness is then coloured from the palette. Redrawn after every pan or zoom.
 */
export class HeatLayer extends L.Layer {
  private canvas?: HTMLCanvasElement
  private host?: L.Map
  private stamp?: HTMLCanvasElement

  constructor(
    private points: L.LatLng[] = [],
    private radius = 26,
  ) {
    super()
  }

  setPoints(points: L.LatLng[]) {
    this.points = points
    this.redraw()
    return this
  }

  override onAdd(map: L.Map) {
    this.host = map
    this.canvas = L.DomUtil.create('canvas', 'leaflet-zoom-hide')
    this.canvas.style.pointerEvents = 'none'
    this.canvas.setAttribute('aria-hidden', 'true')
    map.getPanes().overlayPane.appendChild(this.canvas)
    map.on('moveend resize', this.redraw, this)
    this.redraw()
    return this
  }

  override onRemove(map: L.Map) {
    map.off('moveend resize', this.redraw, this)
    this.canvas?.remove()
    this.canvas = this.host = undefined
    return this
  }

  /** One blob: opaque in the middle, fading to nothing at the radius. */
  private blob() {
    if (this.stamp) return this.stamp
    const r = this.radius
    const c = document.createElement('canvas')
    c.width = c.height = r * 2
    const ctx = c.getContext('2d')!
    const g = ctx.createRadialGradient(r, r, 0, r, r, r)
    g.addColorStop(0, 'rgba(0,0,0,1)')
    g.addColorStop(1, 'rgba(0,0,0,0)')
    ctx.fillStyle = g
    ctx.fillRect(0, 0, r * 2, r * 2)
    return (this.stamp = c)
  }

  redraw() {
    const map = this.host
    const canvas = this.canvas
    if (!map || !canvas) return
    const size = map.getSize()
    L.DomUtil.setPosition(canvas, map.containerPointToLayerPoint([0, 0]))
    canvas.width = size.x
    canvas.height = size.y
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx || !size.x || !size.y) return

    const r = this.radius
    const blob = this.blob()
    // Spread the alpha so a handful of overlapping photos already reads as "hot".
    ctx.globalAlpha = 0.3
    for (const p of this.points) {
      const { x, y } = map.latLngToContainerPoint(p)
      if (x < -r || y < -r || x > size.x + r || y > size.y + r) continue
      ctx.drawImage(blob, x - r, y - r)
    }

    const image = ctx.getImageData(0, 0, size.x, size.y)
    const px = image.data
    const pal = palette()
    for (let i = 3; i < px.length; i += 4) {
      const a = px[i]!
      if (!a) continue
      const j = a * 4
      px[i - 3] = pal[j]!
      px[i - 2] = pal[j + 1]!
      px[i - 1] = pal[j + 2]!
      px[i] = Math.min(220, a * 1.6 + 40)
    }
    ctx.putImageData(image, 0, 0)
  }
}
