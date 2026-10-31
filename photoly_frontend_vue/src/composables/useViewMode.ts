import { watch } from 'vue'
import { useStorage } from '@vueuse/core'

export type ViewMode = 'grid' | 'compact' | 'list'

/** Large cards, small cards or list layout for folders and media. Shared by every view and remembered across visits. */
export const viewMode = useStorage<ViewMode>('photoly-view-mode', 'grid')

watch(
  viewMode,
  (value) => {
    if (!['grid', 'compact', 'list'].includes(value)) viewMode.value = 'grid'
  },
  { immediate: true },
)
