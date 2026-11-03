<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useStorage } from '@vueuse/core'
import { MapPin, Scan, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { usePhotoMap } from '@/composables/usePhotoMap'
import MapProjectionControl from './MapProjectionControl.vue'
import { coordinate, hasLocation, groupPhotos } from '@/lib/mapGeometry'
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
  extra_photo_taken_date?: string
}
const shown = useStorage('photoly-gallery-map-visible', true)
const mapEl = ref<HTMLElement | null>(null)
const { map, fallback, failed } = usePhotoMap(mapEl)
const records = ref<LocatedPhoto[]>([])
const error = ref(false)
const loading = ref(false)
const ids = computed(() => new Set(props.photos.map((p) => p.photoId)))
const points = computed(() =>
  records.value.filter((p) => ids.value.has(p.photo_id) && hasLocation(p)),
)
let controller: AbortController | undefined
let needsFit = true
let loadedGallery = -1
function fit() {
  if (!points.value.length || !map.value || !shown.value) return
  map.value.resize()
  map.value.initialize(points.value.map(coordinate), 'photos')
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
  if (!m) return
  const groups = groupPhotos(points.value, (p) => m.projectVisible(p), 42)
  m.syncPins(
    groups.map((group) => {
      const first = group.photos[0]!
      return {
        key: group.key,
        coordinate: coordinate(first),
        title: group.photos.length === 1 ? first.photo_name : `${group.photos.length} photos`,
        element: () => {
          const thumb = document.createElement('div')
          thumb.className = 'gallery-map-pin'
          const crop = document.createElement('div')
          crop.className = 'gallery-map-image'
          const img = document.createElement('img')
          img.src = photoUrl(first.photo_id, 'renderThumbnail')
          img.alt = ''
          crop.append(img)
          thumb.append(crop)
          if (group.photos.length > 1) {
            const badge = document.createElement('span')
            badge.textContent = String(group.photos.length)
            thumb.append(badge)
          }
          return thumb
        },
        onClick: () => {
          if (group.photos.length === 1) {
            emit('openPhoto', first.photo_id)
            return
          }
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
          m.popup(coordinate(first), list)
          popupPhoto = first.photo_id
        },
      }
    }),
  )
  if (popupPhoto !== null && !groups.some((g) => g.photos.some((p) => p.photo_id === popupPhoto))) {
    m.closePopup()
    popupPhoto = null
  }
}
let popupPhoto: number | null = null
let frame = 0
const scheduleRebuild = () => {
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(rebuild)
}
let unsubscribe: (() => void) | undefined
watch(map, (m) => {
  unsubscribe?.()
  if (m) {
    unsubscribe = m.onChange(scheduleRebuild)
    fit()
    rebuild()
  }
})
watch(() => [props.galleryId, props.revision], load, { immediate: true })
watch(points, async () => {
  await nextTick()
  if (needsFit) fit()
  rebuild()
})
watch(shown, async (value) => {
  if (value) {
    await nextTick()
    map.value?.resize()
    if (needsFit) fit()
  }
})
onBeforeUnmount(() => {
  controller?.abort()
  cancelAnimationFrame(frame)
  unsubscribe?.()
})
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
      <div class="px-2 py-1"><MapProjectionControl :fallback="fallback" /></div>
      <p v-if="failed" role="alert" class="p-2 text-xs">
        Could not initialize the map. Reload to retry.
      </p>
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
.gallery-map-image {
  width: 100%;
  height: 100%;
  overflow: hidden;
  border-radius: 6px;
}
.gallery-map-image img {
  display: block;
  max-width: 100% !important;
  max-height: 100% !important;
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
  max-height: 150px;
  overflow-y: auto;
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
