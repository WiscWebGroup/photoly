<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import L from 'leaflet'
import { toast } from 'vue-sonner'
import '@/resources/RepeatMarker.js'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { MapPin } from '@lucide/vue'
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
const all = shallowRef<MapPhoto[]>([])
const located = computed(() =>
  all.value.filter((p) => p.extra_photo_gps_lat && p.extra_photo_gps_lon),
)
/** The located photos inside the current map view, for the side list. */
const inView = shallowRef<MapPhoto[]>([])
const markers = new Map<number, L.Marker>()

function updateInView() {
  const bounds = map.value?.getBounds()
  if (!bounds) return
  inView.value = located.value.filter((p) =>
    bounds.contains([p.extra_photo_gps_lat!, p.extra_photo_gps_lon!]),
  )
}

function focus(photo: MapPhoto) {
  const marker = markers.get(photo.photo_id)
  if (!marker || !map.value) return
  map.value.flyTo(marker.getLatLng(), Math.max(map.value.getZoom(), 10), { duration: 0.6 })
  map.value.once('moveend', () => marker.openPopup())
}

const escapeAttr = (value: string) =>
  value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')

/**
 * A rounded photo thumbnail as the marker. An HTML string, not a node: the repeated-marker layer
 * clones markers with the same icon, and one node can only sit in one place.
 */
function pin(photo: MapPhoto) {
  const src = escapeAttr(photoUrl(photo.photo_id, 'renderThumbnail'))
  return L.divIcon({
    html: `<div class="photo-pin"><img src="${src}" alt=""></div>`,
    className: '',
    iconSize: [44, 44],
    iconAnchor: [22, 22],
    popupAnchor: [0, -22],
  })
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<HTMLElementTagNameMap[K]> = {},
) {
  return Object.assign(document.createElement(tag), props)
}

/** Built from DOM nodes, not an HTML string, so photo names can't inject markup. */
function popupFor(photo: MapPhoto) {
  const thumb = el('img', { src: photoUrl(photo.photo_id, 'renderThumbnail'), alt: '' })
  thumb.onerror = () => {
    thumb.onerror = null
    thumb.src = photoUrl(photo.photo_id)
  }
  const mapLink = el('a', {
    href: `https://maps.google.com/?q=${photo.extra_photo_gps_lat},${photo.extra_photo_gps_lon}`,
    target: '_blank',
    textContent: 'Open in Map',
  })
  const seePhoto = el('button', { textContent: 'See Photo', type: 'button' })
  seePhoto.onclick = () => (viewing.value = photo)

  const actions = el('div', { className: 'photo-popup-actions' })
  actions.append(seePhoto, mapLink)
  const popup = el('div', { className: 'photo-popup' })
  popup.append(
    el('h2', { textContent: photo.photo_name }),
    el('p', { textContent: photo.extra_photo_taken_date ?? '' }),
    thumb,
    actions,
  )
  return popup
}

onMounted(async () => {
  map.value?.zoomControl.setPosition('bottomright')
  try {
    all.value = await call<MapPhoto[]>({ url: '/photo-extra/getGlobalMapPhoto' })
    const layer = repeatedMarkers().addTo(map.value!)
    for (const photo of located.value) {
      const marker = L.marker([photo.extra_photo_gps_lat!, photo.extra_photo_gps_lon!], {
        draggable: false,
        title: photo.photo_name,
        icon: pin(photo),
      })
        .addTo(map.value!)
        .bindPopup(popupFor(photo), { closeButton: false, minWidth: 220, maxWidth: 240 })
      markers.set(photo.photo_id, marker)
      layer.addMarker(marker)
    }
    // Start where the photos are.
    if (located.value.length)
      map.value!.fitBounds(
        L.latLngBounds(located.value.map((p) => [p.extra_photo_gps_lat!, p.extra_photo_gps_lon!])),
        { padding: [64, 64], maxZoom: 11 },
      )
    map.value!.on('moveend', updateInView)
    updateInView()
  } catch {
    toast.warning('No Info')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="flex h-full min-h-96 flex-col md:flex-row">
    <aside
      class="bg-card flex shrink-0 flex-col gap-1 border-b px-4 py-4 md:w-80 md:gap-5 md:overflow-y-auto md:border-r md:border-b-0 md:px-6 md:pt-8"
    >
      <div class="flex flex-col gap-1">
        <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Map</h1>
        <p class="text-muted-foreground text-sm">
          <template v-if="loading">Finding your photos…</template>
          <template v-else
            >{{ located.length }} of {{ all.length }}
            {{ all.length === 1 ? 'item has' : 'items have' }} a location</template
          >
        </p>
      </div>
      <section v-if="!loading" class="hidden flex-col gap-1 md:flex" aria-labelledby="in-view-h">
        <h2
          id="in-view-h"
          class="text-muted-foreground pb-1 text-xs font-semibold tracking-[0.025em] uppercase"
        >
          In view <span class="font-normal tabular-nums">{{ inView.length }}</span>
        </h2>
        <ul class="flex flex-col gap-0.5">
          <li
            v-for="(p, i) in inView.slice(0, 50)"
            :key="p.photo_id"
            class="animate-rise"
            :style="{ animationDelay: `${Math.min(i, 10) * 25}ms` }"
          >
            <button
              type="button"
              class="hover:bg-accent flex w-full items-center gap-3 rounded-lg p-1.5 text-left transition-colors"
              @click="focus(p)"
            >
              <img
                :src="photoUrl(p.photo_id, 'renderThumbnail')"
                alt=""
                loading="lazy"
                class="bg-muted size-10 shrink-0 rounded-md object-cover"
              />
              <span class="flex min-w-0 flex-col">
                <span class="truncate text-sm font-medium">{{ p.photo_name }}</span>
                <span class="text-muted-foreground text-xs">{{
                  p.extra_photo_taken_date ?? 'No date'
                }}</span>
              </span>
            </button>
          </li>
        </ul>
        <p v-if="!inView.length" class="text-muted-foreground flex items-center gap-2 text-sm">
          <MapPin class="size-4" aria-hidden="true" />
          {{
            located.length ? 'Pan or zoom out to find photos.' : 'No photos have a location yet.'
          }}
        </p>
      </section>
    </aside>

    <div class="relative min-h-96 flex-1">
      <div ref="mapEl" class="absolute inset-0" />
      <Skeleton v-if="loading" class="absolute inset-0 z-[1000] rounded-none opacity-80" />
    </div>

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

<style scoped>
:deep(.photo-pin) {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  overflow: hidden;
  border: 2px solid #fff;
  background: var(--muted);
  box-shadow: 0 2px 8px rgb(0 0 0 / 0.25);
  transition:
    transform 0.2s var(--ease-out-quart),
    border-color 0.2s;
}
:deep(.photo-pin:hover) {
  transform: scale(1.12);
  border-color: var(--primary);
}
:deep(.photo-pin img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
:deep(.leaflet-popup-content-wrapper) {
  border-radius: 12px;
  background: var(--popover);
  color: var(--popover-foreground);
  box-shadow: 0 10px 30px rgb(0 0 0 / 0.18);
}
:deep(.leaflet-popup-tip) {
  background: var(--popover);
}
:deep(.leaflet-popup-content) {
  margin: 12px;
}
:deep(.photo-popup h2) {
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
}
:deep(.photo-popup p) {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--muted-foreground);
}
:deep(.photo-popup img) {
  display: block;
  width: 100%;
  height: 120px;
  object-fit: cover;
  border-radius: 8px;
}
:deep(.photo-popup-actions) {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
  font-size: 13px;
  font-weight: 500;
}
:deep(.photo-popup-actions a),
:deep(.photo-popup-actions button) {
  color: var(--primary);
}
:deep(.photo-popup-actions button:hover),
:deep(.photo-popup-actions a:hover) {
  text-decoration: underline;
}
</style>
