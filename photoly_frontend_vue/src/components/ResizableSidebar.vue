<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMediaQuery, useResizeObserver, useStorage } from '@vueuse/core'
const props = defineProps<{ storageKey: string; label: string; desktopOnly?: boolean }>()
const desktop = useMediaQuery('(min-width: 768px)')
const preferred = useStorage(props.storageKey, 320)
const root = ref<HTMLElement>()
const available = ref(800)
useResizeObserver(
  computed(() => root.value?.parentElement),
  ([entry]) => {
    if (entry) available.value = entry.contentRect.width
  },
)
const max = computed(() => Math.max(200, Math.min(480, available.value - 280)))
const width = computed(() => Math.min(max.value, Math.max(200, Number(preferred.value) || 320)))
const dragging = ref(false)
let startX = 0
let startWidth = 320
function start(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  startX = event.clientX
  startWidth = width.value
  dragging.value = true
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}
function move(event: PointerEvent) {
  if (dragging.value)
    preferred.value = Math.min(max.value, Math.max(200, startWidth + event.clientX - startX))
}
function key(event: KeyboardEvent) {
  const value =
    event.key === 'ArrowLeft'
      ? width.value - 10
      : event.key === 'ArrowRight'
        ? width.value + 10
        : event.key === 'Home'
          ? 200
          : event.key === 'End'
            ? max.value
            : undefined
  if (value === undefined) return
  event.preventDefault()
  preferred.value = Math.min(max.value, Math.max(200, value))
}
</script>
<template>
  <div
    ref="root"
    :class="['relative shrink-0', desktopOnly && 'max-md:hidden']"
    :style="desktop ? { width: `${width}px` } : undefined"
  >
    <slot />
    <div
      v-if="desktop"
      role="separator"
      tabindex="0"
      aria-orientation="vertical"
      :aria-label="label"
      :aria-valuemin="200"
      :aria-valuemax="max"
      :aria-valuenow="Math.round(width)"
      title="Drag to resize; double-click to reset"
      :class="[
        'group absolute inset-y-0 -right-1 z-30 w-2 cursor-col-resize touch-none outline-none focus-visible:bg-primary/25',
        dragging && 'bg-primary/25',
      ]"
      @pointerdown="start"
      @pointermove="move"
      @pointerup="dragging = false"
      @pointercancel="dragging = false"
      @lostpointercapture="dragging = false"
      @keydown="key"
      @dblclick="preferred = 320"
    >
      <span
        class="bg-primary/0 group-hover:bg-primary/40 absolute inset-y-0 left-[3px] w-0.5 transition-colors"
      />
    </div>
  </div>
</template>
