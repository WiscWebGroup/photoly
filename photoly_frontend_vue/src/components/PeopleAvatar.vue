<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useIntersectionObserver } from '@vueuse/core'
import { UserRound } from '@lucide/vue'
import { http } from '@/lib/http'
import { usePeopleStore } from '@/stores/people'
const props = defineProps<{ faceId: number | null; label?: string }>()
const store = usePeopleStore(),
  url = ref(''),
  element = ref<HTMLElement>(),
  near = ref(false),
  loading = ref(false)
const { stop } = useIntersectionObserver(
  element,
  ([entry]) => {
    near.value = !!entry?.isIntersecting
  },
  { rootMargin: `${typeof window === 'undefined' ? 800 : window.innerHeight}px` },
)
let sequence = 0
function clear() {
  sequence++
  if (url.value) URL.revokeObjectURL(url.value)
  url.value = ''
  loading.value = false
}
watch(
  () => [props.faceId, store.visible, store.revision, near.value],
  async () => {
    clear()
    if (!props.faceId || !store.visible || !near.value) return
    const seq = sequence
    const face = props.faceId
    loading.value = true
    try {
      const blob = await store.avatars.get(String(face), async (signal) => {
        const response = await http.get<Blob>(`/ai/people/faces/${face}/thumbnail`, {
          responseType: 'blob',
          signal,
        })
        return response.data
      })
      if (seq === sequence) url.value = URL.createObjectURL(blob)
    } catch {
      /* Keep the placeholder when an original is unavailable. */
    } finally {
      if (seq === sequence) loading.value = false
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  stop()
  clear()
})
</script>
<template>
  <span
    ref="element"
    :aria-busy="loading"
    :class="loading && 'animate-pulse motion-reduce:animate-none'"
    class="bg-muted inline-flex aspect-square shrink-0 items-center justify-center overflow-hidden rounded-full"
  >
    <img v-if="url" :src="url" :alt="label ?? ''" class="size-full object-cover" />
    <UserRound v-else class="size-1/2 text-muted-foreground" aria-hidden="true" />
  </span>
</template>
