<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { call } from '@/lib/http'
const props = defineProps<{ refreshKey?: number }>()
const seconds = ref<number>()
const stale = ref(false)
const started = ref('')
let baseline = 0
let received = 0
let attempted = 0
let disposed = false
let inFlight = false
const label = computed(() => {
  if (seconds.value == null) return 'Unavailable'
  const n = seconds.value
  return `${Math.floor(n / 86400)}d ${Math.floor((n % 86400) / 3600)}h ${Math.floor((n % 3600) / 60)}m ${n % 60}s`
})
async function load() {
  if (document.hidden || inFlight || disposed) return
  inFlight = true
  attempted = performance.now()
  try {
    const v = await call<{ uptimeSeconds: number; startedAt: string }>({ url: '/admin/runtime' })
    if (!disposed) {
      baseline = v.uptimeSeconds
      seconds.value = baseline
      received = performance.now()
      started.value = v.startedAt
      stale.value = false
    }
  } catch {
    if (!disposed) stale.value = true
  } finally {
    inFlight = false
  }
}
let timer: ReturnType<typeof setInterval>
const visible = () => {
  if (!document.hidden) void load()
}
onMounted(() => {
  void load()
  timer = setInterval(() => {
    if (!document.hidden && seconds.value != null && !stale.value)
      seconds.value = baseline + Math.floor((performance.now() - received) / 1000)
    if (performance.now() - attempted >= 30000) void load()
  }, 1000)
  document.addEventListener('visibilitychange', visible)
})
onUnmounted(() => {
  disposed = true
  clearInterval(timer)
  document.removeEventListener('visibilitychange', visible)
})
watch(() => props.refreshKey, load)
</script>
<template>
  <div
    class="text-muted-foreground text-xs"
    :title="started ? `Started ${new Date(started).toLocaleString()}` : ''"
  >
    Uptime <span class="text-foreground tabular-nums">{{ label }}</span
    ><span v-if="stale"> · Last known status</span>
  </div>
</template>
