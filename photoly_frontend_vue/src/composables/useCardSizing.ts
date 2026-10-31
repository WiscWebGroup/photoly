import { computed, ref, type Ref } from 'vue'
import { useResizeObserver, useWindowSize } from '@vueuse/core'

/** Derive compact cells from the existing large-grid breakpoints and actual content width.
 * Halving only the minimum width would change auto-fill column rounding on wide screens.
 */
export function useCardSizing(root: Ref<HTMLElement | undefined>, kind: 'folder' | 'media') {
  const { width: viewport } = useWindowSize()
  const width = ref(0)
  useResizeObserver(root, ([entry]) => {
    if (entry) width.value = entry.contentRect.width
  })
  const columns = computed(() => {
    if (viewport.value >= 1536) {
      const gap = kind === 'folder' ? 16 : 4
      const minimum = kind === 'folder' ? 180 : 200
      return Math.max(1, Math.floor((width.value + gap) / (minimum + gap)))
    }
    if (kind === 'folder') return viewport.value >= 1024 ? 4 : 3
    return viewport.value >= 1280 ? 6 : viewport.value >= 1024 ? 5 : viewport.value >= 640 ? 4 : 3
  })
  const compactStyle = computed(() => {
    if (kind === 'folder' && viewport.value < 640) return { gap: '6px' }
    const n = columns.value
    const gap = kind === 'folder' ? 16 : viewport.value < 640 ? 2 : 4
    return {
      gridTemplateColumns: `repeat(${2 * n}, minmax(0, 1fr))`,
      // Exactly twice as many half-width cells without expanding the grid.
      columnGap: `${(gap * (n - 1)) / (2 * n - 1)}px`,
      rowGap: `${gap / 2}px`,
    }
  })
  return { compactStyle, mobile: computed(() => viewport.value < 640) }
}
