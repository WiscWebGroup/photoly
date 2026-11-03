import { watch } from 'vue'
import { useStorage } from '@vueuse/core'
export type MapProjection = 'globe' | 'mercator'
export const mapProjection = useStorage<MapProjection>('photoly-map-projection', 'globe')
watch(
  mapProjection,
  (value) => {
    if (value !== 'globe' && value !== 'mercator') mapProjection.value = 'globe'
  },
  { immediate: true, flush: 'sync' },
)
