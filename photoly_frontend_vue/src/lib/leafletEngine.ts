import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { HeatLayer } from './heatLayer'
import { photoBounds, wrapLongitude, type Coordinate } from './mapGeometry'
import {
  attribution,
  tileURL,
  reduceMotion,
  type MapEngine,
  type MapPin,
  type Camera,
} from './mapEngine'
const ll = ([lng, lat]: Coordinate): L.LatLngTuple => [
  Math.max(-85.051129, Math.min(85.051129, lat)),
  lng,
]
export function createLeafletEngine(el: HTMLElement, saved?: Camera): MapEngine {
  const map = L.map(el, { attributionControl: false, zoomControl: false }).setView(
    ll(saved?.center ?? [10, 19]),
    Math.max(0, saved?.zoom ?? 2),
  )
  L.control.zoom({ position: 'bottomright' }).addTo(map)
  L.control.attribution({ position: 'bottomleft', prefix: false }).addTo(map)
  L.tileLayer(tileURL, { maxZoom: 19, attribution }).addTo(map)
  const displayCoordinate = (p: Coordinate) =>
    ll([map.getCenter().lng + wrapLongitude(p[0] - map.getCenter().lng), p[1]])
  const heat = new HeatLayer()
  const pins = new Map<string, { marker: L.Marker; pin: MapPin }>()
  const callbacks = new Set<() => void>(),
    changed = () => callbacks.forEach((fn) => fn())
  let preserve = !!saved
  map.on('move zoom resize', changed)
  return {
    kind: 'leaflet',
    camera: () => ({ center: [map.getCenter().lng, map.getCenter().lat], zoom: map.getZoom() }),
    initialize(points) {
      if (preserve) {
        preserve = false
        return
      }
      const b = photoBounds(points)
      if (b) map.fitBounds([ll(b[0]), ll(b[1])], { padding: [32, 32], maxZoom: 11, animate: false })
      changed()
    },
    projection() {},
    dayNight() {},
    projectVisible(p) {
      const point = map.latLngToContainerPoint(displayCoordinate(p))
      return point.x >= 0 && point.y >= 0 && point.x <= el.clientWidth && point.y <= el.clientHeight
        ? point
        : null
    },
    flyTo(p, zoom, done) {
      if (done) map.once('moveend', done)
      map.flyTo(displayCoordinate(p), zoom, { duration: 0.6, animate: !reduceMotion() })
    },
    resize: () => map.invalidateSize({ animate: false }),
    syncPins(next) {
      const keys = new Set(next.map((p) => p.key))
      for (const [key, item] of pins)
        if (!keys.has(key)) {
          item.marker.remove()
          pins.delete(key)
        }
      for (const pin of next) {
        const old = pins.get(pin.key)
        if (old) {
          old.pin = pin
          old.marker.setLatLng(displayCoordinate(pin.coordinate))
          continue
        }
        const element = pin.element()
        const marker = L.marker(displayCoordinate(pin.coordinate), {
          title: pin.title,
          icon: L.divIcon({
            html: element,
            className: 'photo-map-marker',
            iconSize: [
              parseFloat(element.style.width) || 36,
              parseFloat(element.style.height) || 36,
            ],
            iconAnchor: [
              (parseFloat(element.style.width) || 36) / 2,
              (parseFloat(element.style.height) || 36) / 2,
            ],
          }),
        }).addTo(map)
        const item = { marker, pin }
        marker.on('click', () => item.pin.onClick())
        pins.set(pin.key, item)
      }
    },
    selectPin(key) {
      for (const [id, item] of pins)
        item.marker
          .getElement()
          ?.querySelector('.photo-pin')
          ?.classList.toggle('is-active', key === id)
    },
    popup(p, content, onClose) {
      map.closePopup()
      const popup = L.popup({ closeButton: false, maxWidth: 280, offset: [0, -28] })
        .setLatLng(displayCoordinate(p))
        .setContent(content)
        .openOn(map)
      if (onClose) popup.on('remove', onClose)
    },
    closePopup: () => {
      map.closePopup()
    },
    heat(points, visible) {
      heat.setPoints(points.map((p) => L.latLng(ll(p))))
      if (visible && !map.hasLayer(heat)) heat.addTo(map)
      else if (!visible && map.hasLayer(heat)) map.removeLayer(heat)
      el.dataset.heatmap = String(visible)
    },
    onChange(fn) {
      callbacks.add(fn)
      return () => callbacks.delete(fn)
    },
    destroy() {
      callbacks.clear()
      map.remove()
    },
  }
}
