import {
  Map as GLMap,
  Marker,
  Popup,
  NavigationControl,
  AttributionControl,
  setWorkerUrl,
} from 'maplibre-gl'
import type { GeoJSONSource } from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import workerURL from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
setWorkerUrl(workerURL)
import type { MapProjection } from '@/composables/useMapProjection'
import { createGlobeLighting } from './globeLighting'
import {
  photoBounds,
  photoCenter,
  sameLocation,
  wrapLongitude,
  type Coordinate,
} from './mapGeometry'
import {
  attribution,
  tileURL,
  heatStops,
  reduceMotion,
  type MapEngine,
  type MapPin,
  type Camera,
} from './mapEngine'

export async function createMapLibreEngine(
  el: HTMLElement,
  initial: MapProjection,
  onDayNightFailure: () => void = () => {},
): Promise<MapEngine> {
  const map = new GLMap({
    container: el,
    center: [10, 19],
    zoom: 0,
    minZoom: -4,
    maxZoom: 19,
    attributionControl: false,
    renderWorldCopies: true,
    style: {
      version: 8,
      projection: { type: initial },
      sources: {
        osm: { type: 'raster', tiles: [tileURL], tileSize: 256, maxzoom: 19, attribution },
      },
      layers: [{ id: 'osm', type: 'raster', source: 'osm' }],
    },
  })
  map.addControl(new NavigationControl({ visualizePitch: true }), 'bottom-right')
  map.addControl(new AttributionControl({ compact: false }), 'bottom-left')
  await new Promise<void>((resolve, reject) => {
    const ready = () => {
      clearTimeout(timer)
      map.off('error', failed)
      resolve()
    }
    const failed = (event: { error: { message: string } }) => {
      // Tile failures must not take down the photo map. Only initialization has a deadline.
      if (/webgl|gpu|shader/i.test(event.error.message)) {
        clearTimeout(timer)
        map.off('style.load', ready)
        map.remove()
        reject(event.error)
      }
    }
    const timer = setTimeout(() => {
      map.off('error', failed)
      map.off('style.load', ready)
      map.remove()
      reject(new Error('Map initialization timed out'))
    }, 15000)
    map.once('style.load', ready)
    map.on('error', failed)
  })
  map.addSource('photo-heat', {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })
  map.addLayer({
    id: 'photo-heat',
    type: 'heatmap',
    source: 'photo-heat',
    layout: { visibility: 'none' },
    paint: {
      'heatmap-radius': 26,
      'heatmap-intensity': 0.8,
      'heatmap-opacity': 0.85,
      'heatmap-color': [
        'interpolate',
        ['linear'],
        ['heatmap-density'],
        0,
        'rgba(0,0,0,0)',
        ...heatStops.flat(),
      ],
    },
  })
  const lighting = createGlobeLighting(map, el, initial, onDayNightFailure)
  let projection = initial,
    points: Coordinate[] = [],
    flatCamera: Camera | undefined
  let popup: Popup | undefined
  let popupClose: (() => void) | undefined
  const pins = new Map<string, { marker: Marker; pin: MapPin }>()
  const callbacks = new Set<() => void>()
  const changed = () => {
    el.dataset.ready = String(map.loaded())
    el.dataset.zoom = String(map.getZoom())
    el.dataset.longitude = String(map.getCenter().lng)
    el.dataset.latitude = String(map.getCenter().lat)
    callbacks.forEach((fn) => fn())
  }
  map.on('move', changed)
  map.on('resize', changed)
  map.on('idle', changed)
  // Globe diameter is approximately worldSize / π. Fill most of the shorter edge,
  // leaving space for the toolbar and attribution in both main and compact maps.
  const globeZoom = () =>
    Math.min(
      2.8,
      Math.log2((Math.max(100, Math.min(el.clientWidth, el.clientHeight)) * 0.72 * Math.PI) / 512),
    )
  const camera = (): Camera => ({
    center: [map.getCenter().lng, map.getCenter().lat],
    zoom: map.getZoom(),
  })
  function fit() {
    const bounds = photoBounds(points)
    map.stop()
    // Globe fitBounds cannot reliably solve a zero-area box across browsers.
    if (
      bounds &&
      Math.abs(bounds[1][0] - bounds[0][0]) < 1e-7 &&
      Math.abs(bounds[1][1] - bounds[0][1]) < 1e-7
    )
      map.jumpTo({ center: bounds[0], zoom: 11, pitch: 0, bearing: 0 })
    else if (bounds)
      map.fitBounds(bounds, {
        padding: Math.min(64, Math.min(el.clientWidth, el.clientHeight) * 0.16),
        maxZoom: 11,
        duration: 0,
        pitch: 0,
        bearing: 0,
      })
    else map.jumpTo({ center: [10, 19], zoom: 1 })
  }
  function closePopup() {
    const old = popup
    popup = undefined
    old?.remove()
    popupClose = undefined
  }
  const engine: MapEngine = {
    kind: 'maplibre',
    camera,
    dayNight: (enabled) => lighting.enabled(enabled),
    initialize(value, view = 'overview') {
      points = value
      if (projection === 'globe' && view === 'overview')
        map.jumpTo({ center: photoCenter(points), zoom: globeZoom(), pitch: 0, bearing: 0 })
      else fit()
      changed()
    },
    projection(value) {
      if (value === projection) return
      if (projection === 'mercator') flatCamera = camera()
      projection = value
      map.stop()
      map.setProjection({ type: value })
      lighting.projection(value)
      if (value === 'globe')
        map.easeTo({
          center: map.getCenter(),
          zoom: globeZoom(),
          pitch: 0,
          bearing: 0,
          duration: reduceMotion() ? 0 : 600,
        })
      else if (flatCamera) map.jumpTo({ ...flatCamera, pitch: 0, bearing: 0 })
      else fit()
      changed()
    },
    projectVisible(p) {
      const lng = map.getCenter().lng + wrapLongitude(p[0] - map.getCenter().lng)
      const coord: Coordinate = [
        lng,
        projection === 'mercator' ? Math.max(-85.051129, Math.min(85.051129, p[1])) : p[1],
      ]
      const screen = map.project(coord)
      if (
        !Number.isFinite(screen.x) ||
        !Number.isFinite(screen.y) ||
        screen.x < 0 ||
        screen.y < 0 ||
        screen.x > el.clientWidth ||
        screen.y > el.clientHeight
      )
        return null
      const back = map.unproject(screen)
      if (!sameLocation(coord, [back.lng, back.lat])) return null
      return screen
    },
    flyTo(point, zoom, done) {
      map.stop()
      if (done) map.once('moveend', done)
      map.flyTo({ center: point, zoom, duration: reduceMotion() ? 0 : 600 })
    },
    resize: () => map.resize(),
    syncPins(next) {
      const keys = new Set(next.map((p) => p.key))
      for (const [key, item] of pins)
        if (!keys.has(key)) {
          item.marker.remove()
          pins.delete(key)
        }
      for (const pin of next) {
        const existing = pins.get(pin.key)
        if (existing) {
          existing.pin = pin
          existing.marker.getElement().title = pin.title
          existing.marker.getElement().setAttribute('aria-label', pin.title)
          existing.marker.setLngLat(pin.coordinate)
          continue
        }
        const element = document.createElement('button')
        element.type = 'button'
        element.title = pin.title
        element.setAttribute('aria-label', pin.title)
        element.className = 'photo-map-marker'
        element.append(pin.element())
        const item = {
          pin,
          marker: new Marker({
            element,
            anchor: 'center',
            opacityWhenCovered: 0,
            subpixelPositioning: true,
          })
            .setLngLat(pin.coordinate)
            .addTo(map),
        }
        element.onclick = (event) => {
          event.stopPropagation()
          item.pin.onClick()
        }
        pins.set(pin.key, item)
      }
    },
    selectPin(key) {
      for (const [id, item] of pins)
        item.marker
          .getElement()
          .querySelector('.photo-pin')
          ?.classList.toggle('is-active', id === key)
    },
    popup(point, content, onClose) {
      closePopup()
      popupClose = onClose
      popup = new Popup({
        closeButton: false,
        maxWidth: '280px',
        anchor: el.clientHeight < 300 ? 'center' : undefined,
        offset: el.clientHeight < 300 ? 0 : 28,
        locationOccludedOpacity: 0,
        focusAfterOpen: false,
      })
        .setLngLat(point)
        .setDOMContent(content)
        .addTo(map)
      popup.on('close', () => {
        const cb = popupClose
        popupClose = undefined
        cb?.()
      })
    },
    closePopup,
    heat(value, visible) {
      const source = map.getSource<GeoJSONSource>('photo-heat')
      if (source?.type === 'geojson')
        source.setData({
          type: 'FeatureCollection',
          features: value.map((p) => ({
            type: 'Feature',
            geometry: { type: 'Point', coordinates: p },
            properties: {},
          })),
        })
      map.setLayoutProperty('photo-heat', 'visibility', visible ? 'visible' : 'none')
      el.dataset.heatmap = String(visible)
    },
    onChange(fn) {
      callbacks.add(fn)
      return () => callbacks.delete(fn)
    },
    destroy() {
      lighting.destroy()
      callbacks.clear()
      popupClose = undefined
      closePopup()
      pins.forEach((item) => item.marker.remove())
      map.remove()
    },
  }
  return engine
}
