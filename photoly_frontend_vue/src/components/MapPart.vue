<script setup lang="ts">
import { onMounted, ref } from 'vue'
import L from 'leaflet'
import { toast } from 'vue-sonner'
import '@/resources/RepeatMarker.js'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useLeafletMap } from '@/composables/useLeafletMap'
import { call } from '@/lib/http'
import { photoUrl } from '@/lib/photoUrl'

interface MapPhoto {
  photo_id: number
  photo_name: string
  extra_photo_gps_lat?: number
  extra_photo_gps_lon?: number
  extra_photo_taken_date?: string
}

// RepeatMarker.js adds this layer type, which has no typings.
type RepeatedMarkers = L.GridLayer & { addMarker(marker: L.Marker): void }
const repeatedMarkers = () =>
  (L.gridLayer as unknown as { repeatedMarkers(): RepeatedMarkers }).repeatedMarkers()

const mapEl = ref<HTMLElement | null>(null)
const map = useLeafletMap(mapEl, [19, 10], 3)
const loading = ref(true)
const viewing = ref<MapPhoto | null>(null)

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<HTMLElementTagNameMap[K]> = {},
) {
  return Object.assign(document.createElement(tag), props)
}

/** Built from DOM nodes, not an HTML string, so photo names can't inject markup. */
function popupFor(photo: MapPhoto) {
  const thumb = el('img', { src: photoUrl(photo.photo_id, true) })
  thumb.style.width = '100px'
  thumb.onerror = () => {
    thumb.onerror = null
    thumb.src = photoUrl(photo.photo_id)
  }
  const mapLink = el('a', {
    href: `https://maps.google.com/?q=${photo.extra_photo_gps_lat},${photo.extra_photo_gps_lon}`,
    target: '_blank',
    textContent: 'Open in Map',
  })
  const seePhoto = el('button', { textContent: 'See Photo' })
  seePhoto.onclick = () => (viewing.value = photo)

  const row = (child: Node) => el('div').appendChild(child).parentElement!
  const popup = el('div')
  popup.append(
    el('h2', { textContent: photo.photo_name }),
    el('p', { textContent: photo.extra_photo_taken_date ?? '' }),
    thumb,
    row(mapLink),
    row(seePhoto),
  )
  return popup
}

onMounted(async () => {
  try {
    const photos = await call<MapPhoto[]>({ url: '/photo-extra/getGlobalMapPhoto' })
    const layer = repeatedMarkers().addTo(map.value!)
    for (const photo of photos) {
      const { extra_photo_gps_lat: lat, extra_photo_gps_lon: lon } = photo
      if (!lat || !lon) continue
      layer.addMarker(
        L.marker([lat, lon], { draggable: false, title: photo.photo_name })
          .addTo(map.value!)
          .bindPopup(popupFor(photo)),
      )
    }
    setTimeout(() => map.value?.invalidateSize(), 100)
  } catch {
    toast.warning('No Info')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="relative h-full min-h-96">
    <div ref="mapEl" class="absolute inset-0" />
    <Skeleton v-if="loading" class="absolute inset-0 z-[1000] rounded-none opacity-80" />

    <Dialog :open="!!viewing" @update:open="viewing = null">
      <DialogContent class="max-w-[85vw] p-2 sm:max-w-3xl">
        <DialogTitle class="sr-only">{{ viewing?.photo_name }}</DialogTitle>
        <DialogDescription class="sr-only">Full-size photo</DialogDescription>
        <img
          v-if="viewing"
          :src="photoUrl(viewing.photo_id)"
          :alt="viewing.photo_name"
          class="max-h-[85vh] w-full rounded-md object-contain"
        />
      </DialogContent>
    </Dialog>
  </div>
</template>
