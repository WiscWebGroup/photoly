<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useIntersectionObserver, useResizeObserver } from '@vueuse/core'
import type { Map as LeafletMap, Marker } from 'leaflet'
import { attribution, tileURL } from '@/lib/mapEngine'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  latitude: number
  longitude: number
  active: boolean
  ready: boolean
}>()
const open = defineModel<boolean>('open', { default: true })
const container = ref<HTMLElement | null>(null)
const visible = ref(false)
const loading = ref(false)
const error = ref(false)
const retry = ref(0)
let map: LeafletMap | undefined
let marker: Marker | undefined
let generation = 0
let frame = 0
let deadline: ReturnType<typeof setTimeout> | undefined

useIntersectionObserver(container, ([entry]) => {
  visible.value = !!entry?.isIntersecting
})
useResizeObserver(container, () => map?.invalidateSize({ animate: false }))
const enabled = computed(() => open.value && props.active && props.ready && visible.value)

function dispose() {
  generation++
  cancelAnimationFrame(frame)
  clearTimeout(deadline)
  map?.remove()
  map = undefined
  marker = undefined
  loading.value = false
}

watch(
  [enabled, retry],
  ([enabled]) => {
    dispose()
    error.value = false
    if (!enabled || !container.value) return
    const current = generation
    loading.value = true
    const fail = () => {
      if (current !== generation) return
      clearTimeout(deadline)
      loading.value = false
      error.value = true
    }
    const initialize = async () => {
      deadline = setTimeout(fail, 15000)
      try {
        // Keep Leaflet and its styles off the viewer's initial loading path.
        const [L] = await Promise.all([import('leaflet'), import('leaflet/dist/leaflet.css')])
        if (current !== generation || !container.value || error.value) return
        map = L.map(container.value, {
          attributionControl: false,
          scrollWheelZoom: false,
        }).setView([props.latitude, props.longitude], 13)
        L.control.attribution({ prefix: false }).addTo(map)
        marker = L.marker([props.latitude, props.longitude], {
          title: 'Photo location',
          alt: 'Photo location',
          keyboard: false,
          icon: L.divIcon({
            className: 'location-map-pin',
            html: '<span></span>',
            iconSize: [24, 24],
            iconAnchor: [12, 12],
          }),
        }).addTo(map)
        L.tileLayer(tileURL, { maxZoom: 19, attribution })
          .on('tileerror', fail)
          .on('load', () => {
            if (current !== generation) return
            clearTimeout(deadline)
            loading.value = false
          })
          .addTo(map)
      } catch {
        if (current !== generation) return
        map?.remove()
        map = undefined
        marker = undefined
        fail()
      }
    }
    // Let the completed opening animation paint before parsing or initializing the map.
    frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        void initialize()
      })
    })
  },
  { flush: 'post' },
)
watch(
  () => [props.latitude, props.longitude] as const,
  ([lat, lon]) => {
    map?.setView([lat, lon], 13, { animate: false })
    marker?.setLatLng([lat, lon])
  },
)
onBeforeUnmount(dispose)
</script>

<template>
  <details :open="open" class="border-t px-5 py-4 md:px-6" data-location-section>
    <!-- Commit the preference synchronously: a queued native toggle can arrive after a fast switch. -->
    <summary class="cursor-pointer text-sm font-medium" @click.prevent="open = !open">
      Location
    </summary>
    <div
      v-show="open"
      class="bg-muted relative isolate mt-3 h-[200px] overflow-hidden rounded-lg border"
      data-location-map
      @touchstart.stop
      @touchmove.stop
      @touchend.stop
    >
      <div ref="container" role="region" aria-label="Photo location map" class="h-full w-full" />
      <p
        v-if="!error && (loading || !enabled)"
        role="status"
        class="bg-card/90 pointer-events-none absolute inset-0 z-1000 grid place-items-center text-sm"
      >
        Loading map…
      </p>
      <div
        v-if="error"
        class="bg-card/95 absolute inset-0 z-1000 flex flex-col items-center justify-center gap-2 p-3 text-sm"
      >
        <p role="alert">Could not load the map.</p>
        <Button variant="outline" size="sm" @click="retry++">Retry map</Button>
      </div>
    </div>
  </details>
</template>

<style scoped>
:deep(.location-map-pin span) {
  display: block;
  width: 24px;
  height: 24px;
  border: 3px solid white;
  border-radius: 50%;
  background: #16827d;
  box-shadow: 0 1px 6px #0008;
}
:deep(.leaflet-control-attribution) {
  font-size: 10px;
  line-height: 14px;
}
</style>
