<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { Download } from '@lucide/vue'
import MediaTaskDialog from '@/components/MediaTaskDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { uploadCall } from '@/stores/uploads'
const auth = useAuthStore()
const enabled = ref(false)
const open = ref(false)
let controller: AbortController | undefined
async function refresh() {
  controller?.abort()
  controller = new AbortController()
  const signal = controller.signal
  if (!auth.token) {
    enabled.value = false
    open.value = false
    return
  }
  try {
    const result = await uploadCall<{ onlineDownload: boolean }>(
      '/photo/mediaCapabilities',
      'get',
      undefined,
      signal,
    )
    if (!signal.aborted) enabled.value = result.onlineDownload === true
  } catch {
    if (!signal.aborted) enabled.value = false
  }
}
watch(() => auth.token, refresh)
let timer: ReturnType<typeof setInterval>
onMounted(() => {
  void refresh()
  window.addEventListener('focus', refresh)
  timer = setInterval(refresh, 30000)
})
onUnmounted(() => {
  controller?.abort()
  clearInterval(timer)
  window.removeEventListener('focus', refresh)
})
</script>
<template>
  <MediaTaskDialog v-if="enabled" v-model:open="open" popover>
    <template #trigger>
      <button
        type="button"
        aria-label="Online Download"
        title="Online Download"
        class="hover:bg-accent grid size-9 shrink-0 place-items-center rounded-lg"
      >
        <Download class="size-4" />
      </button>
    </template>
  </MediaTaskDialog>
</template>
