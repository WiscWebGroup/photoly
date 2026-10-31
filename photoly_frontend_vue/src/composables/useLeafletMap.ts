import { onBeforeUnmount, onMounted, shallowRef, type Ref } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'

// Bundlers rewrite the asset URLs Leaflet guesses for its default marker, so set it explicitly.
L.Marker.prototype.options.icon = L.icon({
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  shadowSize: [41, 41],
  iconAnchor: [12, 41],
  shadowAnchor: [12, 41],
})

/** An OpenStreetMap map mounted on `el`, available as `map` once the component is mounted. */
export function useLeafletMap(
  el: Ref<HTMLElement | null>,
  center: L.LatLngExpression,
  zoom: number,
) {
  const map = shallowRef<L.Map>()

  onMounted(() => {
    map.value = L.map(el.value!, { attributionControl: false }).setView(center, zoom)
    L.tileLayer('https://a.tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 19 }).addTo(
      map.value,
    )
  })
  let resizeFrame = 0
  useResizeObserver(el, () => {
    cancelAnimationFrame(resizeFrame)
    resizeFrame = requestAnimationFrame(() =>
      map.value?.invalidateSize({ pan: true, animate: false, debounceMoveend: true }),
    )
  })
  onBeforeUnmount(() => {
    cancelAnimationFrame(resizeFrame)
    map.value?.remove()
  })

  return map
}
