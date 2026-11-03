import { beforeEach, expect, it, vi } from 'vitest'
import { createMapLibreEngine } from '../maplibreEngine'
const map = vi.hoisted(() => ({
  addControl: vi.fn(),
  addSource: vi.fn(),
  addLayer: vi.fn(),
  on: vi.fn(),
  off: vi.fn(),
  once: vi.fn((_event: string, callback: () => void) => queueMicrotask(callback)),
  loaded: () => true,
  getZoom: () => 0,
  getCenter: () => ({ lng: 10, lat: 19 }),
  jumpTo: vi.fn(),
  fitBounds: vi.fn(),
  remove: vi.fn(),
  stop: vi.fn(),
}))
vi.mock('maplibre-gl', () => ({
  Map: class {
    constructor() {
      return map
    }
  },
  NavigationControl: class {},
  AttributionControl: class {},
  Marker: class {},
  Popup: class {},
  setWorkerUrl: vi.fn(),
}))
beforeEach(() => vi.clearAllMocks())
it('keeps the main globe overview and explicitly fits Gallery photos in either projection', async () => {
  const el = document.createElement('div')
  Object.defineProperties(el, { clientWidth: { value: 360 }, clientHeight: { value: 200 } })
  const engine = await createMapLibreEngine(el, 'globe')
  engine.initialize([[-68, 44]])
  expect(map.jumpTo).toHaveBeenCalledWith(expect.objectContaining({ center: [-68, 44] }))
  expect(map.fitBounds).not.toHaveBeenCalled()
  for (const projection of ['globe', 'mercator'] as const) {
    const gallery = await createMapLibreEngine(el, projection)
    gallery.initialize(
      [
        [-68, 44],
        [-67, 45],
      ],
      'photos',
    )
    expect(map.fitBounds).toHaveBeenLastCalledWith(
      [
        [-68, 44],
        [-67, 45],
      ],
      { maxZoom: 11, padding: 32, duration: 0, pitch: 0, bearing: 0 },
    )
    gallery.destroy()
  }
  engine.destroy()
})
it('fits Gallery photos across the date line and uses the default center for an empty set', async () => {
  const engine = await createMapLibreEngine(document.createElement('div'), 'globe')
  engine.initialize(
    [
      [179, 10],
      [-179, 20],
    ],
    'photos',
  )
  expect(map.fitBounds).toHaveBeenCalledWith(
    [
      [179, 10],
      [181, 20],
    ],
    expect.any(Object),
  )
  engine.initialize([], 'photos')
  expect(map.jumpTo).toHaveBeenLastCalledWith({ center: [10, 19], zoom: 1 })
  engine.destroy()
})

it('centers identical coordinates directly instead of fitting an empty area', async () => {
  const engine = await createMapLibreEngine(document.createElement('div'), 'globe')
  engine.initialize(
    [
      [-68, 44],
      [-68, 44],
    ],
    'photos',
  )
  expect(map.fitBounds).not.toHaveBeenCalled()
  expect(map.jumpTo).toHaveBeenLastCalledWith({ center: [-68, 44], zoom: 11, pitch: 0, bearing: 0 })
  engine.destroy()
})
