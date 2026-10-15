import { useStorage } from '@vueuse/core'

export type ViewMode = 'grid' | 'list'

/** Grid or list layout for folders and media. Shared by every view and remembered across visits. */
export const viewMode = useStorage<ViewMode>('photoly-view-mode', 'grid')
