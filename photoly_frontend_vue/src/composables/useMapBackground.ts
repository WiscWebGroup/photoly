import { watch } from 'vue'
import { useStorage } from '@vueuse/core'

export type MapBackground = 'stars' | 'gradient' | 'simple'
export const mapBackground = useStorage<MapBackground>('photoly-map-background', 'stars')
watch(
  mapBackground,
  (value) => {
    if (!['stars', 'gradient', 'simple'].includes(value)) mapBackground.value = 'stars'
  },
  { immediate: true, flush: 'sync' },
)
