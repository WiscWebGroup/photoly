import type { Map as GLMap } from 'maplibre-gl'
import type { MapProjection } from '@/composables/useMapProjection'
import { DayNightLayer } from './dayNightLayer'

/** Owns one map's resources; the Vue preference is shared, the renderer is not. */
export function createGlobeLighting(
  map: GLMap,
  el: HTMLElement,
  initial: MapProjection,
  onFailure: () => void,
) {
  let projection = initial
  let enabled = true
  let failed = false
  let disposed = false
  let layer: DayNightLayer | undefined
  let timer: ReturnType<typeof setInterval> | undefined

  function stop() {
    clearInterval(timer)
    timer = undefined
    if (layer && map.getLayer(layer.id)) map.removeLayer(layer.id)
    layer = undefined
    el.dataset.dayNight = 'false'
  }
  function fail() {
    if (disposed || failed) return
    failed = true
    stop()
    el.dataset.dayNightAvailable = 'false'
    onFailure()
  }
  function sync() {
    if (disposed) return
    const active =
      enabled && !failed && projection === 'globe' && !document.hidden && map.getZoom() < 6
    if (!active) {
      stop()
      return
    }
    if (layer) return
    try {
      layer = new DayNightLayer(fail)
      map.addLayer(layer, 'photo-heat')
      layer.update(new Date())
      timer = setInterval(() => layer?.update(new Date()), 60000)
      el.dataset.dayNight = 'true'
    } catch {
      fail()
    }
  }
  function brightness() {
    // Keep projection switching and the explicit zoom fade deterministic.
    map.setPaintProperty('osm', 'raster-brightness-max-transition', { duration: 0 })
    map.setPaintProperty(
      'osm',
      'raster-brightness-max',
      projection === 'globe' ? ['interpolate', ['linear'], ['zoom'], 3, 0.9, 6, 1] : 1,
    )
  }
  el.dataset.dayNightAvailable = 'true'
  brightness()
  document.addEventListener('visibilitychange', sync)
  map.on('zoom', sync)
  // The engine applies the saved preference before the first illumination layer is added.
  return {
    enabled(value: boolean) {
      enabled = value
      sync()
    },
    projection(value: MapProjection) {
      projection = value
      brightness()
      sync()
    },
    destroy() {
      disposed = true
      document.removeEventListener('visibilitychange', sync)
      map.off('zoom', sync)
      stop()
    },
  }
}
