import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { Map as GLMap } from 'maplibre-gl'
import { createGlobeLighting } from '../globeLighting'

const created = vi.hoisted(
  () => [] as { update: ReturnType<typeof vi.fn>; fail: () => void; id: string }[],
)
vi.mock('../dayNightLayer', () => ({
  DayNightLayer: class {
    id = 'globe-day-night'
    update = vi.fn()
    constructor(public fail: () => void) {
      created.push(this)
    }
  },
}))
let zoom = 2
let current: unknown
let onZoom: () => void
let controller: ReturnType<typeof createGlobeLighting>
const map = {
  getZoom: () => zoom,
  setPaintProperty: vi.fn(),
  addLayer: vi.fn((layer: unknown) => {
    current = layer
  }),
  getLayer: vi.fn(() => current),
  removeLayer: vi.fn(() => {
    current = undefined
  }),
  on: vi.fn((_event: string, fn: () => void) => {
    onZoom = fn
  }),
  off: vi.fn(),
}
let element: HTMLElement
const failure = vi.fn()
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-03-20T06:00:00Z'))
  vi.clearAllMocks()
  created.length = 0
  zoom = 2
  current = undefined
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  element = document.createElement('div')
  controller = createGlobeLighting(map as unknown as GLMap, element, 'globe', failure)
})
afterEach(() => {
  controller.destroy()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

it('inserts below heat and updates once a minute, not on map movements', () => {
  controller.enabled(true)
  expect(map.addLayer).toHaveBeenCalledWith(created[0], 'photo-heat')
  expect(created[0]!.update).toHaveBeenCalledExactlyOnceWith(new Date())
  onZoom()
  vi.advanceTimersByTime(60000)
  expect(created[0]!.update).toHaveBeenCalledTimes(2)
  expect(created).toHaveLength(1)
  expect(created[0]!.update).toHaveBeenLastCalledWith(new Date('2026-03-20T06:01:00Z'))
  expect(element.dataset.dayNight).toBe('true')
})

it('releases resources while disabled, flat, hidden, zoomed in and destroyed', () => {
  controller.enabled(false)
  expect(created).toHaveLength(0)
  controller.enabled(true)
  controller.enabled(false)
  expect(vi.getTimerCount()).toBe(0)
  controller.enabled(true)
  controller.projection('mercator')
  expect(map.setPaintProperty).toHaveBeenCalledWith('osm', 'raster-brightness-max', 1)
  expect(vi.getTimerCount()).toBe(0)
  controller.projection('globe')
  expect(element.dataset.dayNight).toBe('true')
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
  document.dispatchEvent(new Event('visibilitychange'))
  expect(vi.getTimerCount()).toBe(0)
  expect(current).toBeUndefined()
  vi.advanceTimersByTime(120000)
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  document.dispatchEvent(new Event('visibilitychange'))
  expect(created.at(-1)!.update).toHaveBeenLastCalledWith(new Date())
  zoom = 6
  onZoom()
  expect(vi.getTimerCount()).toBe(0)
  zoom = 3
  onZoom()
  expect(vi.getTimerCount()).toBe(1)
  controller.destroy()
  document.dispatchEvent(new Event('visibilitychange'))
  expect(vi.getTimerCount()).toBe(0)
  expect(current).toBeUndefined()
  expect(map.off).toHaveBeenCalledWith('zoom', onZoom)
})

it('disables only illumination after shader failure and ignores callbacks after disposal', () => {
  controller.enabled(true)
  created[0]!.fail()
  expect(failure).toHaveBeenCalledOnce()
  expect(element.dataset.dayNightAvailable).toBe('false')
  expect(vi.getTimerCount()).toBe(0)
  controller.enabled(true)
  expect(created).toHaveLength(1)
  controller.destroy()
  created[0]!.fail()
  expect(failure).toHaveBeenCalledOnce()
})
