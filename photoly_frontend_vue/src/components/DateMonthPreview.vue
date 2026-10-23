<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { call } from '@/lib/http'
import { photoUrl } from '@/lib/photoUrl'

const props = defineProps<{ month: string; order: string; count: number; revision: number }>()
const root = ref<HTMLElement>()
const visible = ref(false)
const ids = ref<number[]>([])
const failed = ref(false)
let loadedKey = ''
let observer: IntersectionObserver | undefined
onMounted(() => {
  observer = new IntersectionObserver(
    ([entry]) => {
      visible.value = !!entry?.isIntersecting
    },
    { rootMargin: '150px' },
  )
  if (root.value) observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())
watch(
  [visible, () => props.month, () => props.order, () => props.revision],
  async (_, __, cleanup) => {
    const key = `${props.month}:${props.order}:${props.revision}`
    if (!visible.value || loadedKey === key) return
    ids.value = []
    const controller = new AbortController()
    cleanup(() => controller.abort())
    failed.value = false
    try {
      const result = await call<number[]>({
        url: '/photo/getMonthPreview',
        params: { month: props.month, order: props.order },
        signal: controller.signal,
      })
      if (!controller.signal.aborted) {
        ids.value = result
        loadedKey = key
      }
    } catch {
      if (!controller.signal.aborted) failed.value = true
    }
  },
  { immediate: true },
)
</script>

<template>
  <span
    ref="root"
    class="mt-3 flex min-w-0 gap-1.5 overflow-hidden"
    data-testid="month-preview"
    aria-hidden="true"
  >
    <span v-if="failed" class="text-muted-foreground flex h-14 items-center text-xs"
      >Preview unavailable</span
    >
    <template v-else>
      <span
        v-for="(_, index) in Math.min(count, 10)"
        :key="index"
        class="bg-muted h-14 min-w-0 max-w-18 flex-1 overflow-hidden rounded-md sm:h-16 nth-[n+6]:hidden sm:nth-[n+6]:block"
      >
        <img
          v-if="ids[index]"
          :src="photoUrl(ids[index]!, 'renderThumbnail')"
          alt=""
          loading="lazy"
          decoding="async"
          class="size-full object-cover"
        />
      </span>
    </template>
  </span>
</template>
