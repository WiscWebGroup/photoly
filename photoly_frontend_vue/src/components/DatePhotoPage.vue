<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import MediaGrid, { type MediaGridActions } from './MediaGrid.vue'
import { Button } from '@/components/ui/button'
import { viewMode } from '@/composables/useViewMode'
import type { DatePage } from '@/composables/useDateTimeline'

const props = defineProps<{
  page: DatePage
  actions: MediaGridActions
  selectMode: boolean
  suspended?: boolean
}>()
const emit = defineEmits<{ visibility: [visible: boolean]; retry: [] }>()
const root = ref<HTMLElement>()
const content = ref<HTMLElement>()
const near = ref(false)
const height = ref(160)
let intersection: IntersectionObserver | undefined
let resize: ResizeObserver | undefined
let viewportResize: ResizeObserver | undefined
const heights = new Map<string, number>()
let width = 0
let measurementFrame = 0
function measure() {
  cancelAnimationFrame(measurementFrame)
  measurementFrame = requestAnimationFrame(() => {
    if (!root.value) return
    const nextWidth = Math.round(root.value.getBoundingClientRect().width)
    if (nextWidth !== width) {
      width = nextWidth
      height.value = heights.get(heightKey()) ?? 160
    }
    const measured = content.value?.getBoundingClientRect().height
    if (measured) {
      height.value = measured
      heights.set(heightKey(), measured)
    }
  })
}
const heightKey = () => `${viewMode.value}:${width}`
function observe() {
  intersection?.disconnect()
  if (!root.value || props.suspended) {
    near.value = false
    return
  }
  const scrollRoot = root.value.closest('main')
  const overscan = scrollRoot?.clientHeight || window.innerHeight
  intersection = new IntersectionObserver(
    ([entry]) => {
      near.value = !!entry?.isIntersecting
    },
    { root: scrollRoot, rootMargin: `${overscan}px 0px` },
  )
  intersection.observe(root.value)
}
watch([near, () => props.suspended], () => emit('visibility', near.value && !props.suspended))
watch(() => props.suspended, observe)
watch(content, (el) => {
  resize?.disconnect()
  if (!el) return
  resize = new ResizeObserver(measure)
  resize.observe(el)
})
watch(viewMode, () => {
  height.value = heights.get(heightKey()) ?? 160
})
onMounted(() => {
  observe()
  viewportResize = new ResizeObserver(measure)
  if (root.value) viewportResize.observe(root.value)
})
onBeforeUnmount(() => {
  cancelAnimationFrame(measurementFrame)
  intersection?.disconnect()
  resize?.disconnect()
  viewportResize?.disconnect()
  emit('visibility', false)
})
</script>

<template>
  <div ref="root" :data-date-page="page.key" :style="{ minHeight: `${height}px` }">
    <div v-if="near && !suspended && page.items" ref="content">
      <MediaGrid :items="page.items" :actions="actions" :select-mode="selectMode" timeline />
      <p v-if="!page.items.length" class="text-muted-foreground py-4 text-sm">
        No photos on this day.
      </p>
    </div>
    <div
      v-else-if="near && !suspended"
      class="text-muted-foreground flex min-h-40 items-center gap-3 text-sm"
    >
      <template v-if="page.error">
        <span role="alert">Could not load photos.</span>
        <Button variant="outline" @click="emit('retry')">Retry photos</Button>
      </template>
      <span v-else role="status">Loading photos…</span>
    </div>
  </div>
</template>
