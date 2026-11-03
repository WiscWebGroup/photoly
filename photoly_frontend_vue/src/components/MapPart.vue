<script setup lang="ts">
import ResizableSidebar from './ResizableSidebar.vue'
import { computed, onMounted, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useMediaQuery } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import PhotoViewer from '@/components/PhotoViewer.vue'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import type { Photo } from '@/composables/usePhotos'
import { useStorageRevision } from '@/composables/useStorageRevision'
import MapDates, { type DateRange } from '@/components/MapDates.vue'
import { Flame, Images, MapPin } from '@lucide/vue'
import { usePhotoMap } from '@/composables/usePhotoMap'
import MapProjectionControl from './MapProjectionControl.vue'
import { coordinate, hasLocation, groupPhotos } from '@/lib/mapGeometry'
import { call } from '@/lib/http'
import { heatGradient } from '@/lib/mapEngine'
import { photoUrl } from '@/lib/photoUrl'
import { cn } from '@/lib/utils'

interface MapPhoto {
  photo_id: number
  photo_name: string
  format?: string
  ns_id?: number
  source_id?: number
  visibility?: number
  token?: string
  photo_uuid?: string
  upload_date?: string
  extra_photo_gps_lat?: number
  extra_photo_gps_lon?: number
  extra_photo_taken_date?: string
}

/** The nearest town to a photo, from the server's offline place list. */
interface PhotoPlace {
  photoId: number
  place: string
  country: string
}

/** Photos close together on screen, drawn as one pin with a count. */
interface Place {
  key: string
  photos: MapPhoto[]
  lat: number
  lng: number
}

const route = useRoute()
const mobile = useMediaQuery('(max-width: 767px)')
const mapEl = ref<HTMLElement | null>(null)
const { map, fallback, failed } = usePhotoMap(mapEl)
const loading = ref(true)
const locationsFailed = ref(false)
const requestController = new AbortController()
const viewing = ref<Photo | null>(null)
const viewerPhotos = ref<Photo[]>([])
const viewerPath = ref('')
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
const moving = ref<Photo | null>(null)
const asPhoto = (p: MapPhoto): Photo => ({
  photoId: p.photo_id,
  photoName: p.photo_name,
  format: p.format,
  nsId: p.ns_id,
  sourceId: p.source_id,
  visibility: p.visibility,
  token: p.token,
  photoUuid: p.photo_uuid,
  uploadDate: p.upload_date,
})
async function navigatePhoto(photo: Photo) {
  viewing.value = photo
  viewerPath.value = ''
  if (photo.nsId === undefined) return
  try {
    const path = await call<{ nsName: string }[]>({
      url: '/namespace/trace',
      params: { nsId: photo.nsId },
    })
    if (viewing.value?.photoId === photo.photoId)
      viewerPath.value =
        '/root' +
        path
          .slice(1)
          .map((n) => `/${n.nsName}`)
          .join('')
  } catch {
    /* The viewer still works if its directory is no longer available. */
  }
}
function viewMapPhoto(photo: MapPhoto) {
  viewerPhotos.value = (
    browsing.value?.photos.some((p) => p.photo_id === photo.photo_id)
      ? browsing.value.photos
      : placeOf(photo.photo_id)?.photos || [photo]
  ).map(asPhoto)
  browsing.value = null
  void navigatePhoto(asPhoto(photo))
}
async function reloadMap() {
  const selectedId = selected.value?.photos[0]?.photo_id
  all.value = await call<MapPhoto[]>({
    url: '/photo-extra/getGlobalMapPhoto',
    signal: requestController.signal,
  })
  const byId = new Map(all.value.map((p) => [p.photo_id, asPhoto(p)]))
  viewerPhotos.value = viewerPhotos.value.flatMap((p) => byId.get(p.photoId) || [])
  if (viewing.value) viewing.value = byId.get(viewing.value.photoId) || null
  rebuild()
  if (selectedId !== undefined) select(placeOf(selectedId) || null)
}
async function viewerChanged() {
  try {
    await reloadMap()
  } catch {
    toast.error('Could not refresh map photos')
  }
}
function askMove(photo: Photo) {
  moving.value = photo
  movePicker.value?.openModal()
}
async function moveChosen(nsId: number) {
  if (!moving.value) return
  try {
    await call({
      method: 'post',
      url: '/photo/changeNamespace',
      params: { photoId: moving.value.photoId, nsId },
    })
    await reloadMap()
    if (viewing.value) await navigatePhoto(viewing.value)
  } catch {
    toast.error('Could not move photo')
  }
}
async function deleteViewed(photo: Photo) {
  const at = viewerPhotos.value.findIndex((p) => p.photoId === photo.photoId)
  const next = viewerPhotos.value[at + 1] || viewerPhotos.value[at - 1]
  try {
    await call({ method: 'post', url: '/photo/delete', params: { photoId: photo.photoId } })
    await reloadMap()
    if (next) await navigatePhoto(next)
    else viewing.value = null
  } catch {
    toast.error('Could not delete photo')
  }
}
useStorageRevision(reloadMap, () => !!viewing.value || !!browsing.value)
const all = shallowRef<MapPhoto[]>([])
const towns = shallowRef(new Map<number, string>())
const styles = [
  { value: 'photos', label: 'Photos', icon: Images },
  { value: 'heat', label: 'Heat', icon: Flame },
] as const
const style = ref<'photos' | 'heat'>('photos')
const located = computed(() => all.value.filter(hasLocation))
const range = ref<DateRange>(null)
const shown = computed(() => {
  const r = range.value
  if (!r) return located.value
  return located.value.filter((p) => {
    const day = p.extra_photo_taken_date?.slice(0, 10)
    return !!day && day >= r.from && day <= r.to
  })
})
const takenDays = computed(() =>
  located.value.flatMap((p) =>
    p.extra_photo_taken_date ? [p.extra_photo_taken_date.slice(0, 10)] : [],
  ),
)

const places = shallowRef<Place[]>([])
/** The places inside the current map view, biggest first, for the side list. */
const inView = shallowRef<Place[]>([])
/** The place whose card is open: a popup on desktop, the bottom sheet on phones. */
const selected = shallowRef<Place | null>(null)
/** All of one place's photos, from "View all". */
const browsing = shallowRef<Place | null>(null)
const placeOf = (photoId: number) =>
  places.value.find((pl) => pl.photos.some((p) => p.photo_id === photoId))

/** "Jun 9 – 14", in the reader's locale, with the year only when it isn't this year. */
function dateSpan(photos: MapPhoto[]) {
  const days = photos.flatMap((p) => p.extra_photo_taken_date?.slice(0, 10) ?? []).sort()
  if (!days.length) return 'No date'
  const [first, last] = [days[0]!, days.at(-1)!].map((d) => new Date(`${d}T00:00:00`)) as [
    Date,
    Date,
  ]
  const thisYear = new Date().getFullYear()
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    ...((first.getFullYear() !== thisYear || last.getFullYear() !== thisYear) && {
      year: 'numeric',
    }),
  }).formatRange(first, last)
}
/** The town most of the place's photos were taken in; else the photo's name or the coordinates. */
function placeName(pl: Place) {
  const counts = new Map<string, number>()
  for (const p of pl.photos) {
    const town = towns.value.get(p.photo_id)
    if (town) counts.set(town, (counts.get(town) ?? 0) + 1)
  }
  let best: string | undefined
  for (const [town, n] of counts) if (!best || n > counts.get(best)!) best = town
  if (best) return best
  return pl.photos.length === 1
    ? pl.photos[0]!.photo_name
    : `${Math.abs(pl.lat).toFixed(2)}° ${pl.lat >= 0 ? 'N' : 'S'}, ${Math.abs(pl.lng).toFixed(2)}° ${pl.lng >= 0 ? 'E' : 'W'}`
}
const countLabel = (pl: Place) =>
  `${pl.photos.length} ${pl.photos.length === 1 ? 'photo' : 'photos'}`
const mapsLink = (pl: Place) =>
  pl.photos.length === 1
    ? `https://maps.google.com/?q=${pl.photos[0]!.extra_photo_gps_lat},${pl.photos[0]!.extra_photo_gps_lon}`
    : `https://maps.google.com/?q=${pl.lat.toFixed(5)},${pl.lng.toFixed(5)}`

/** Only instantiated for a newly visible cluster. */
function pin(pl: Place) {
  const size = pl.photos.length > 1 ? 52 : 44
  const element = el('div', { className: 'photo-pin' })
  element.style.width = element.style.height = `${size}px`
  element.append(thumb(pl.photos[0]!))
  if (pl.photos.length > 1)
    element.append(
      el('span', { className: 'photo-pin-count', textContent: String(pl.photos.length) }),
    )
  return element
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<HTMLElementTagNameMap[K]> = {},
) {
  return Object.assign(document.createElement(tag), props)
}
function thumb(photo: MapPhoto) {
  const img = el('img', { src: photoUrl(photo.photo_id, 'renderThumbnail'), alt: '' })
  img.onerror = () => {
    img.onerror = null
    img.src = photoUrl(photo.photo_id)
  }
  return img
}

/** Built from DOM nodes, not an HTML string, so photo names can't inject markup. */
function popupFor(pl: Place) {
  const popup = el('div', { className: 'photo-popup' })
  const title = el('div', { className: 'photo-popup-title' })
  title.append(
    el('h2', { textContent: placeName(pl) }),
    el('span', {
      textContent:
        pl.photos.length === 1 ? dateSpan(pl.photos) : `${countLabel(pl)} · ${dateSpan(pl.photos)}`,
    }),
  )
  const grid = el('div', {
    className: pl.photos.length === 1 ? 'photo-popup-single' : 'photo-popup-grid',
  })
  for (const photo of pl.photos.slice(0, 4)) {
    const open = el('button', { type: 'button' })
    open.setAttribute('aria-label', `View ${photo.photo_name}`)
    open.append(thumb(photo))
    open.onclick = () => viewMapPhoto(photo)
    grid.append(open)
  }
  const actions = el('div', { className: 'photo-popup-actions' })
  if (pl.photos.length === 1) {
    const see = el('button', { type: 'button', textContent: 'See Photo' })
    see.onclick = () => viewMapPhoto(pl.photos[0]!)
    actions.append(see)
  } else {
    const all = el('button', { type: 'button', textContent: `View all ${pl.photos.length}` })
    all.onclick = () => (browsing.value = pl)
    actions.append(all)
  }
  actions.append(el('a', { href: mapsLink(pl), target: '_blank', textContent: 'Open in Map' }))
  popup.append(title, grid, actions)
  return popup
}

let closingPopup = false
function closePopup() {
  closingPopup = true
  map.value?.closePopup()
  closingPopup = false
}
function select(pl: Place | null) {
  closePopup()
  selected.value = pl
  map.value?.selectPin(pl?.key)
  if (pl && !mobile.value)
    map.value?.popup([pl.lng, pl.lat], popupFor(pl), () => {
      if (!closingPopup) {
        selected.value = null
        map.value?.selectPin()
      }
    })
}
function rebuild() {
  const m = map.value
  if (!m) return
  const previous = selected.value
  const keep = previous?.photos[0]?.photo_id
  places.value = groupPhotos(shown.value, (p) => m.projectVisible(p))
  inView.value = [...places.value].sort((a, b) => b.photos.length - a.photos.length)
  m.syncPins(
    style.value === 'heat'
      ? []
      : places.value.map((pl) => ({
          key: pl.key,
          coordinate: [pl.lng, pl.lat],
          title: pl.photos.length === 1 ? pl.photos[0]!.photo_name : countLabel(pl),
          element: () => pin(pl),
          onClick: () => select(pl),
        })),
  )
  const current = keep === undefined ? null : (placeOf(keep) ?? null)
  if (previous?.key !== current?.key) select(current)
  else {
    selected.value = current
    m.selectPin(current?.key)
  }
}
let frame = 0
function scheduleRebuild() {
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(rebuild)
}
function updateHeat() {
  map.value?.heat(shown.value.map(coordinate), style.value === 'heat')
}
function focus(pl: Place) {
  const m = map.value
  if (!m) return
  style.value = 'photos'
  const target = pl.photos[0]!.photo_id
  m.flyTo(
    [pl.lng, pl.lat],
    Math.max(m.camera().zoom, pl.photos.length > 1 ? m.camera().zoom + 2 : 10),
    () => {
      rebuild()
      openPlaceOf(target)
    },
  )
}
function openPlaceOf(photoId: number) {
  const pl = placeOf(photoId)
  if (pl) select(pl)
}
watch(shown, () => {
  updateHeat()
  rebuild()
})
watch(style, () => {
  select(null)
  updateHeat()
  rebuild()
})
watch(mobile, () => {
  const keep = selected.value
  select(keep)
  rebuild()
})
let unsubscribe: (() => void) | undefined
watch(map, (m) => {
  unsubscribe?.()
  if (m) unsubscribe = m.onChange(scheduleRebuild)
})
watch([map, loading], ([m, busy]) => {
  if (!m || busy) return
  const selectedId = selected.value?.photos[0]?.photo_id
  m.initialize(located.value.map(coordinate))
  updateHeat()
  rebuild()
  if (selectedId !== undefined) openPlaceOf(selectedId)
  const wanted = located.value.find((p) => p.photo_id === Number(route.query.photo))
  if (wanted)
    m.flyTo(coordinate(wanted), 12, () => {
      rebuild()
      openPlaceOf(wanted.photo_id)
    })
})
async function loadLocations() {
  locationsFailed.value = false
  loading.value = true
  try {
    const named = call<PhotoPlace[]>({
      url: '/photo-extra/getMapPlaces',
      signal: requestController.signal,
    }).then(
      (list) => (towns.value = new Map(list.map((p) => [p.photoId, p.place]))),
      () => undefined,
    )
    all.value = await call<MapPhoto[]>({
      url: '/photo-extra/getGlobalMapPhoto',
      signal: requestController.signal,
    })
    await named
  } catch {
    if (!requestController.signal.aborted) locationsFailed.value = true
  } finally {
    loading.value = false
  }
}
onMounted(loadLocations)
onBeforeUnmount(() => {
  requestController.abort()
  cancelAnimationFrame(frame)
  unsubscribe?.()
})

const sheetPeers = computed(() => inView.value.slice(0, 8))
</script>

<template>
  <div class="relative flex h-full min-h-96 flex-col md:flex-row">
    <ResizableSidebar
      storage-key="photoly-map-sidebar-width"
      label="Resize map sidebar"
      desktop-only
    >
      <aside
        class="bg-card hidden h-full w-full shrink-0 flex-col gap-4 overflow-y-auto border-r px-4 pt-8 pb-6 md:flex"
      >
        <div class="flex flex-col gap-1 px-2">
          <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Map</h1>
          <p class="text-muted-foreground text-sm">
            <template v-if="loading">Finding your photos…</template>
            <template v-else
              >{{ located.length }} of {{ all.length }}
              {{ all.length === 1 ? 'item has' : 'items have' }} a location</template
            >
          </p>
        </div>
        <section v-if="!loading" class="flex flex-col gap-0.5" aria-labelledby="in-view-h">
          <h2
            id="in-view-h"
            class="text-muted-foreground px-2 pt-2 pb-1.5 text-xs font-semibold tracking-[0.025em] uppercase"
          >
            Places in view
          </h2>
          <ul class="flex flex-col gap-0.5">
            <li
              v-for="(pl, i) in inView.slice(0, 50)"
              :key="pl.key"
              class="animate-rise"
              :style="{ animationDelay: `${Math.min(i, 10) * 25}ms` }"
            >
              <button
                type="button"
                :class="
                  cn(
                    'hover:bg-accent flex w-full items-center gap-3 rounded-[10px] p-2 text-left transition-colors',
                    selected?.key === pl.key && 'bg-primary/10 hover:bg-primary/15',
                  )
                "
                @click="focus(pl)"
              >
                <img
                  :src="photoUrl(pl.photos[0]!.photo_id, 'renderThumbnail')"
                  alt=""
                  loading="lazy"
                  class="bg-muted size-11 shrink-0 rounded-lg object-cover"
                />
                <span class="flex min-w-0 flex-1 flex-col">
                  <span class="truncate text-sm font-medium">{{ placeName(pl) }}</span>
                  <span class="text-muted-foreground text-xs">{{ dateSpan(pl.photos) }}</span>
                </span>
                <span class="text-muted-foreground text-xs tabular-nums">{{
                  pl.photos.length
                }}</span>
              </button>
            </li>
          </ul>
          <p
            v-if="!inView.length"
            class="text-muted-foreground flex items-center gap-2 px-2 text-sm"
          >
            <MapPin class="size-4" aria-hidden="true" />
            {{
              !located.length
                ? 'No photos have a location yet.'
                : shown.length
                  ? 'Pan or zoom out to find photos.'
                  : 'No photos in these dates.'
            }}
          </p>
        </section>
      </aside>
    </ResizableSidebar>

    <div class="relative isolate min-h-96 flex-1 overflow-hidden">
      <div ref="mapEl" class="absolute inset-0 isolate" style="position: absolute" />
      <p v-if="failed" role="alert" class="bg-card absolute inset-x-4 top-28 z-[1000] rounded p-3">
        Could not initialize the map. Reload this page to retry.
      </p>
      <div
        class="pointer-events-none absolute inset-x-4 top-3 z-[1000] flex flex-wrap items-start gap-2 md:top-4"
      >
        <h1
          class="bg-card pointer-events-auto flex h-11 items-center rounded-xl border px-4 font-semibold shadow-[0_2px_8px_rgb(0_0_0/0.08)] md:hidden"
        >
          Map
        </h1>
        <MapProjectionControl :fallback="fallback" />
        <MapDates
          v-model="range"
          :dates="takenDays"
          class="pointer-events-auto max-md:ml-auto max-md:h-11 max-md:rounded-xl"
        />
        <div
          role="radiogroup"
          aria-label="Map style"
          class="bg-card pointer-events-auto flex h-9 gap-0.5 rounded-lg border p-0.5 shadow-[0_2px_6px_rgb(0_0_0/0.08)] max-md:h-11 max-md:rounded-xl"
        >
          <button
            v-for="s in styles"
            :key="s.value"
            type="button"
            role="radio"
            :aria-checked="style === s.value"
            :aria-label="s.label"
            :class="
              cn(
                'text-muted-foreground hover:text-foreground flex items-center gap-1.5 rounded-md px-2.5 text-sm font-medium transition-[background-color,color] max-md:rounded-[10px]',
                style === s.value && 'bg-muted text-foreground',
              )
            "
            @click="style = s.value"
          >
            <component :is="s.icon" class="size-4" aria-hidden="true" />
            <span class="max-md:sr-only">{{ s.label }}</span>
          </button>
        </div>
      </div>
      <div
        v-if="style === 'heat'"
        class="bg-card text-muted-foreground absolute bottom-10 left-4 z-[1000] flex items-center gap-2 rounded-lg border px-3 py-2 text-xs shadow-[0_2px_6px_rgb(0_0_0/0.08)]"
      >
        Fewer
        <span class="h-1.5 w-24 rounded-full" :style="{ background: heatGradient }" />
        More
      </div>
      <div
        v-if="locationsFailed"
        role="alert"
        class="bg-card absolute inset-x-4 top-28 z-[1000] flex items-center justify-between rounded-lg border p-3 text-sm"
      >
        <span>Could not load photo locations.</span>
        <button type="button" class="text-primary underline" @click="loadLocations">
          Retry map
        </button>
      </div>
      <Skeleton v-if="loading" class="absolute inset-0 z-[1000] rounded-none opacity-80" />

      <!-- Phones: the selected place as a bottom sheet. -->
      <Transition
        enter-from-class="translate-y-full"
        leave-to-class="translate-y-full"
        enter-active-class="transition-transform duration-300 ease-out"
        leave-active-class="transition-transform duration-200 ease-in"
      >
        <section
          v-if="mobile && selected"
          :aria-label="placeName(selected)"
          class="bg-card absolute inset-x-0 bottom-6 z-[1000] flex flex-col gap-3 rounded-t-[20px] px-4 pt-2 pb-4 shadow-[0_-8px_24px_rgb(0_0_0/0.1)]"
        >
          <button
            type="button"
            aria-label="Close"
            class="bg-border h-1 w-9 self-center rounded-full"
            @click="select(null)"
          />
          <div class="flex flex-wrap items-baseline gap-x-2">
            <h2 class="max-w-full min-w-0 truncate text-lg leading-[26px] font-semibold">
              {{ placeName(selected) }}
            </h2>
            <span class="text-muted-foreground text-sm">
              {{
                selected.photos.length === 1
                  ? dateSpan(selected.photos)
                  : `${countLabel(selected)} · ${dateSpan(selected.photos)}`
              }}
            </span>
            <button
              v-if="selected.photos.length > 1"
              type="button"
              class="text-primary ml-auto shrink-0 text-sm font-medium"
              @click="browsing = selected"
            >
              View all
            </button>
          </div>
          <div class="grid grid-cols-4 gap-1">
            <button
              v-for="p in selected.photos.slice(0, 4)"
              :key="p.photo_id"
              type="button"
              :aria-label="`View ${p.photo_name}`"
              @click="viewMapPhoto(p)"
            >
              <img
                :src="photoUrl(p.photo_id, 'renderThumbnail')"
                alt=""
                class="bg-muted aspect-square w-full rounded-lg object-cover"
              />
            </button>
          </div>
          <div class="flex items-center gap-2 overflow-x-auto [scrollbar-width:none]">
            <a
              :href="mapsLink(selected)"
              target="_blank"
              class="text-primary shrink-0 pr-2 text-sm font-medium"
              >Open in Map</a
            >
            <button
              v-for="pl in sheetPeers"
              :key="pl.key"
              type="button"
              :class="
                cn(
                  'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-sm',
                  pl.key === selected.key &&
                    'bg-primary/10 border-transparent font-medium text-foreground',
                )
              "
              @click="focus(pl)"
            >
              <span class="max-w-32 truncate">{{ placeName(pl) }}</span>
              <span class="text-muted-foreground tabular-nums">{{ pl.photos.length }}</span>
            </button>
          </div>
        </section>
      </Transition>
    </div>

    <Dialog :open="!!browsing" @update:open="browsing = null">
      <DialogContent class="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
        <DialogTitle>{{ browsing && placeName(browsing) }}</DialogTitle>
        <DialogDescription>
          {{ browsing && `${countLabel(browsing)} · ${dateSpan(browsing.photos)}` }}
        </DialogDescription>
        <div class="grid grid-cols-3 gap-1 sm:grid-cols-4">
          <button
            v-for="p in browsing?.photos"
            :key="p.photo_id"
            type="button"
            :aria-label="`View ${p.photo_name}`"
            @click="viewMapPhoto(p)"
          >
            <img
              :src="photoUrl(p.photo_id, 'renderThumbnail')"
              alt=""
              loading="lazy"
              class="bg-muted aspect-square w-full rounded-md object-cover"
            />
          </button>
        </div>
      </DialogContent>
    </Dialog>

    <PhotoViewer
      :photo="viewing"
      :photos="viewerPhotos"
      :ns-path="viewerPath"
      context="Map"
      manage
      @close="viewing = null"
      @navigate="navigatePhoto"
      @changed="viewerChanged"
      @move="askMove"
      @delete="deleteViewed"
    />
    <MoveToShowFolder
      ref="movePicker"
      :count="1"
      :source-id="moving?.sourceId"
      :original-ns-id="moving?.nsId"
      :item-name="moving?.photoName"
      @move-to-update="moveChosen"
    />
  </div>
</template>

<style scoped>
:deep(.photo-pin) {
  position: relative;
  border-radius: 10px;
  border: 3px solid #fff;
  background: var(--muted);
  box-shadow: 0 6px 18px rgb(0 0 0 / 0.2);
  transition:
    transform 0.2s var(--ease-out-quart),
    border-color 0.2s;
}
:deep(.photo-pin:hover) {
  transform: scale(1.08);
}
:deep(.photo-pin.is-active) {
  border-color: var(--primary);
  transform: scale(1.18);
  box-shadow: 0 6px 18px rgb(0 0 0 / 0.25);
}
:deep(.photo-pin img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 7px;
  display: block;
}
:deep(.photo-pin-count) {
  position: absolute;
  top: -10px;
  right: -10px;
  min-width: 24px;
  height: 24px;
  padding: 0 6px;
  box-sizing: border-box;
  display: grid;
  place-items: center;
  border-radius: 999px;
  background: #0a0a0a;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  box-shadow: 0 0 0 2px #fff;
}
:deep(.photo-pin.is-active .photo-pin-count) {
  background: var(--primary);
}
:deep(.leaflet-control-zoom) {
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 6px rgb(0 0 0 / 0.08);
  margin: 0 16px 24px 0;
}
:deep(.leaflet-control-zoom a) {
  width: 40px;
  height: 40px;
  line-height: 40px;
  background: var(--card);
  color: var(--foreground);
  border-bottom-color: var(--border);
}
:deep(.leaflet-control-zoom a:hover) {
  background: var(--accent);
}
:deep(.leaflet-popup-content-wrapper) {
  border-radius: 12px;
  background: var(--popover);
  color: var(--popover-foreground);
  border: 1px solid var(--border);
  box-shadow: 0 10px 30px rgb(0 0 0 / 0.15);
}
:deep(.leaflet-popup-tip) {
  background: var(--popover);
}
:deep(.leaflet-popup-content) {
  margin: 12px;
  font-size: 14px;
  line-height: 20px;
}
:deep(.photo-popup-title) {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 10px;
}
:deep(.photo-popup-title h2) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
:deep(.photo-popup-title span) {
  flex: none;
  font-size: 12px;
  color: var(--muted-foreground);
}
:deep(.photo-popup-grid) {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 3px;
}
:deep(.photo-popup-grid img) {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 4px;
  display: block;
}
:deep(.photo-popup-single img) {
  display: block;
  width: 100%;
  height: 128px;
  object-fit: cover;
  border-radius: 8px;
}
:deep(.photo-popup-actions) {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
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
