<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useStorage } from '@vueuse/core'
import { MapPin, Scan, X } from '@lucide/vue'
import L from 'leaflet'
import { Button } from '@/components/ui/button'
import { useLeafletMap } from '@/composables/useLeafletMap'
import type { Photo } from '@/composables/usePhotos'
import { call } from '@/lib/http'
import { photoUrl } from '@/lib/photoUrl'
const props = defineProps<{
  galleryId: number
  photos: Photo[]
  revision: number
  selecting?: boolean
}>()
const emit = defineEmits<{ openPhoto: [id: number] }>()
interface LocatedPhoto {
  photo_id: number
  photo_name: string
  extra_photo_gps_lat: number
  extra_photo_gps_lon: number
}
const shown = useStorage('photoly-gallery-map-visible', true)
const mapEl = ref<HTMLElement | null>(null)
const map = useLeafletMap(mapEl, [20, 0], 2)
const records = ref<LocatedPhoto[]>([])
const error = ref(false)
const loading = ref(false)
const ids = computed(() => new Set(props.photos.map((p) => p.photoId)))
const points = computed(() =>
  records.value.filter(
    (p) =>
      ids.value.has(p.photo_id) &&
      typeof p.extra_photo_gps_lat === 'number' &&
      typeof p.extra_photo_gps_lon === 'number' &&
      Number.isFinite(p.extra_photo_gps_lat) &&
      Number.isFinite(p.extra_photo_gps_lon) &&
      Math.abs(p.extra_photo_gps_lat) <= 90 &&
      Math.abs(p.extra_photo_gps_lon) <= 180,
  ),
)
let controller: AbortController | undefined
let needsFit = true
let loadedGallery = -1
let markers: L.LayerGroup | undefined
const latLng = (p: LocatedPhoto): L.LatLngTuple => [p.extra_photo_gps_lat, p.extra_photo_gps_lon]
function fit() {
  if (!points.value.length || !map.value || !shown.value) return
  map.value.invalidateSize()
  map.value.fitBounds(L.latLngBounds(points.value.map(latLng)), {
    padding: [32, 32],
    maxZoom: 13,
    animate: false,
  })
  needsFit = false
}
async function load() {
  controller?.abort()
  controller = new AbortController()
  const signal = controller.signal
  if (loadedGallery !== props.galleryId) {
    records.value = []
    needsFit = true
    loadedGallery = props.galleryId
  }
  loading.value = true
  error.value = false
  try {
    const data = await call<LocatedPhoto[]>({
      url: '/photo-extra/getGlobalMapPhotoByGallery',
      params: { gaId: props.galleryId },
      signal,
    })
    if (!signal.aborted) records.value = data
  } catch {
    if (!signal.aborted) error.value = true
  } finally {
    if (!signal.aborted) loading.value = false
  }
}
function rebuild() {
  const m = map.value
  if (!m || !markers) return
  markers.clearLayers()
  const groups: { point: L.Point; photos: LocatedPhoto[] }[] = []
  for (const p of points.value) {
    const point = m.project(latLng(p), m.getZoom())
    const group = groups.find((g) => g.point.distanceTo(point) < 42)
    if (group) group.photos.push(p)
    else groups.push({ point, photos: [p] })
  }
  for (const group of groups) {
    const first = group.photos[0]!
    const thumb = document.createElement('div')
    thumb.className = 'gallery-map-pin'
    const img = document.createElement('img')
    img.src = photoUrl(first.photo_id, 'renderThumbnail')
    img.alt = ''
    thumb.append(img)
    if (group.photos.length > 1) {
      const badge = document.createElement('span')
      badge.textContent = String(group.photos.length)
      thumb.append(badge)
    }
    const marker = L.marker(latLng(first), {
      title: group.photos.length === 1 ? first.photo_name : `${group.photos.length} photos`,
      icon: L.divIcon({ html: thumb, className: '', iconSize: [36, 36], iconAnchor: [18, 18] }),
    }).addTo(markers)
    if (group.photos.length === 1) marker.on('click', () => emit('openPhoto', first.photo_id))
    else {
      marker.bindPopup(
        () => {
          const list = document.createElement('div')
          list.className = 'gallery-map-photos'
          for (const photo of group.photos) {
            const button = document.createElement('button')
            button.type = 'button'
            button.title = photo.photo_name
            button.setAttribute('aria-label', `View ${photo.photo_name}`)
            const image = document.createElement('img')
            image.src = photoUrl(photo.photo_id, 'renderThumbnail')
            image.alt = ''
            image.loading = 'lazy'
            button.append(image)
            button.onclick = () => emit('openPhoto', photo.photo_id)
            list.append(button)
          }
          return list
        },
        { maxWidth: 230, maxHeight: 150, autoPan: false },
      )
    }
  }
}
watch(() => [props.galleryId, props.revision], load, { immediate: true })
watch(points, async () => {
  await nextTick()
  if (needsFit) fit()
  rebuild()
})
watch(shown, async (value) => {
  if (value) {
    await nextTick()
    map.value?.invalidateSize()
    if (needsFit) fit()
  }
})
onMounted(() => {
  markers = L.layerGroup().addTo(map.value!)
  map.value!.on('zoomend', rebuild)
  rebuild()
})
onBeforeUnmount(() => controller?.abort())
</script>
<template>
  <aside
    v-show="points.length || error"
    aria-label="Gallery map"
    class="fixed right-4 z-30 max-w-[calc(100vw-2rem)] md:right-6"
    :class="selecting ? 'bottom-36 md:bottom-20' : 'bottom-20 md:bottom-6'"
  >
    <Button
      v-if="!shown"
      size="icon"
      class="shadow-lg"
      aria-label="Show gallery map"
      @click="shown = true"
      ><MapPin
    /></Button>
    <div
      v-show="shown"
      class="bg-card flex h-[280px] max-h-[50dvh] w-[360px] max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl border shadow-lg"
    >
      <header class="flex shrink-0 items-center gap-2 border-b px-3 py-2">
        <MapPin class="text-primary size-4" />
        <h2 class="flex-1 text-sm font-semibold">Gallery map</h2>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Show all photo locations"
          title="Show all photo locations"
          :disabled="!points.length"
          @click="fit"
          ><Scan
        /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Hide gallery map" @click="shown = false"
          ><X
        /></Button>
      </header>
      <div v-if="error" role="alert" class="flex items-center justify-between gap-2 p-2 text-xs">
        <span>Could not load photo locations.</span
        ><Button variant="outline" size="sm" :disabled="loading" @click="load">Retry map</Button>
      </div>
      <div ref="mapEl" class="isolate min-h-0 flex-1" />
    </div>
  </aside>
</template>
<style>
.gallery-map-pin {
  position: relative;
  width: 36px;
  height: 36px;
  border: 2px solid var(--primary);
  border-radius: 8px;
  background: var(--muted);
  box-shadow: 0 2px 6px #0003;
}
.gallery-map-pin img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 6px;
}
.gallery-map-pin span {
  position: absolute;
  right: -8px;
  top: -8px;
  min-width: 18px;
  border-radius: 12px;
  padding: 1px 4px;
  background: var(--primary);
  color: var(--primary-foreground);
  text-align: center;
  font-size: 11px;
}
.gallery-map-photos {
  display: grid;
  grid-template-columns: repeat(3, 60px);
  gap: 4px;
}
.gallery-map-photos button {
  border-radius: 4px;
  overflow: hidden;
  cursor: pointer;
}
.gallery-map-photos img {
  width: 60px;
  height: 60px;
  object-fit: cover;
}
</style>
