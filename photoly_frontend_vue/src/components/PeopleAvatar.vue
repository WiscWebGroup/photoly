<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { UserRound } from '@lucide/vue'
import { http } from '@/lib/http'
import { usePeopleStore } from '@/stores/people'
const props = defineProps<{ faceId: number | null; label?: string }>()
const store = usePeopleStore(),
  url = ref('')
let controller: AbortController | undefined
function clear() {
  controller?.abort()
  if (url.value) URL.revokeObjectURL(url.value)
  url.value = ''
}
watch(
  () => [props.faceId, store.visible],
  async () => {
    clear()
    if (!props.faceId || !store.visible) return
    const current = new AbortController()
    controller = current
    try {
      const response = await http.get<Blob>(`/ai/people/faces/${props.faceId}/thumbnail`, {
        responseType: 'blob',
        signal: current.signal,
      })
      if (!current.signal.aborted) url.value = URL.createObjectURL(response.data)
    } catch {
      /* Placeholder for unavailable originals. */
    }
  },
  { immediate: true },
)
onBeforeUnmount(clear)
</script>
<template>
  <span
    class="bg-muted inline-flex aspect-square shrink-0 items-center justify-center overflow-hidden rounded-full"
    ><img v-if="url" :src="url" :alt="label ?? ''" class="size-full object-cover" /><UserRound
      v-else
      class="size-1/2 text-muted-foreground"
      aria-hidden="true"
  /></span>
</template>
