import { onMounted, onUnmounted, ref } from 'vue'
import { call } from '@/lib/http'

export interface StorageStatus {
  name?: string
  sourceId: number
  sourceType?: string
  enabled: boolean
  revision: number
  scanStatus: string
  scanPhase?: string
  scanError: string | null
  scannedAt: string | null
  scanCompleted: number
  scanTotal: number
  scanRetries: number
  scanDiscovered?: number
  scanWaiting?: number
  scanUnchanged?: number
  scanQueued?: boolean
}
/** Refresh idle library views after app writes or filesystem reconciliation. */
export function useStorageRevision(refresh: () => unknown | Promise<unknown>, busy: () => boolean) {
  const sources = ref<StorageStatus[]>([])
  let lastPoll = 0
  let revision: string | undefined
  let checking = false
  let stopped = false
  async function poll() {
    if (stopped || checking || document.hidden) return
    if (
      Date.now() - lastPoll <
      (sources.value.some((s) => s.scanStatus === 'scanning' || s.scanQueued) ? 2000 : 15000)
    )
      return
    lastPoll = Date.now()
    checking = true
    try {
      sources.value = await call<StorageStatus[]>({
        url: '/user/storage/sources',
      })
      const next = JSON.stringify(sources.value.map((s) => [s.sourceId, s.revision, s.enabled]))
      // Recheck after the request: selecting/editing may have started meanwhile.
      if (stopped || busy() || document.querySelector('[role="dialog"], [role="alertdialog"]'))
        return
      if (revision !== undefined && revision !== next) await refresh()
      revision = next
    } catch {
      /* Retry on the next poll without interrupting browsing. */
    } finally {
      checking = false
    }
  }
  let timer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    void poll()
    timer = setInterval(poll, 2000)
  })
  onUnmounted(() => {
    stopped = true
    clearInterval(timer)
  })
  return {
    sources,
    poll: () => {
      lastPoll = 0
      return poll()
    },
  }
}
