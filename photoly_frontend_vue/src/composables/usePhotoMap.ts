import { onMounted, onBeforeUnmount, shallowRef, ref, watch, type Ref } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { mapProjection } from './useMapProjection'
import { mapBackground } from './useMapBackground'
import { mapDayNight } from './useMapDayNight'
import type { MapEngine, Camera } from '@/lib/mapEngine'
import '@/lib/photoMap.css'

export function usePhotoMap(el: Ref<HTMLElement | null>) {
  watch(
    [el, mapBackground],
    ([element, background]) => {
      if (element) element.dataset.background = background
    },
    { immediate: true },
  )
  const map = shallowRef<MapEngine>()
  const fallback = ref(false),
    failed = ref(false)
  const dayNightUnavailable = ref(false)
  let disposed = false,
    replacing = false
  let removeContextListener: (() => void) | undefined
  async function compatibility(saved?: Camera) {
    if (disposed || replacing) return
    replacing = true
    fallback.value = true
    removeContextListener?.()
    map.value?.destroy()
    map.value = undefined
    try {
      const { createLeafletEngine } = await import('@/lib/leafletEngine')
      if (disposed || !el.value) return
      el.value.replaceChildren()
      map.value = createLeafletEngine(el.value, saved)
      el.value.dataset.engine = 'leaflet'
      el.value.dataset.projection = 'mercator'
    } catch {
      failed.value = true
    } finally {
      replacing = false
    }
  }
  onMounted(async () => {
    try {
      const { createMapLibreEngine } = await import('@/lib/maplibreEngine')
      if (disposed || !el.value) return
      const engine = await createMapLibreEngine(el.value, mapProjection.value, () => {
        dayNightUnavailable.value = true
      })
      if (disposed) {
        engine.destroy()
        return
      }
      engine.projection(mapProjection.value)
      engine.dayNight(mapDayNight.value)
      map.value = engine
      el.value.dataset.engine = 'maplibre'
      el.value.dataset.projection = mapProjection.value
      const canvas = el.value.querySelector('canvas')!
      const lost = (event: Event) => {
        event.preventDefault()
        void compatibility(engine.camera())
      }
      canvas.addEventListener('webglcontextlost', lost)
      removeContextListener = () => canvas.removeEventListener('webglcontextlost', lost)
    } catch {
      await compatibility()
    }
  })
  watch(mapProjection, (value) => {
    map.value?.projection(value)
    if (el.value && map.value) el.value.dataset.projection = fallback.value ? 'mercator' : value
  })
  watch(mapDayNight, (value) => map.value?.dayNight(value))
  useResizeObserver(el, () => map.value?.resize())
  onBeforeUnmount(() => {
    disposed = true
    removeContextListener?.()
    map.value?.destroy()
  })
  return { map, fallback, failed, dayNightUnavailable }
}
