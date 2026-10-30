import { onMounted, onUnmounted } from 'vue'
import { call } from '@/lib/http'

/** Refresh idle library views after app writes or filesystem reconciliation. */
export function useStorageRevision(refresh: () => unknown | Promise<unknown>, busy: () => boolean) {
  let revision: string | undefined
  let checking = false
  let stopped = false
  async function poll() {
    if (
      stopped ||
      checking ||
      document.hidden ||
      busy() ||
      document.querySelector('[role="dialog"], [role="alertdialog"]')
    )
      return
    checking = true
    try {
      const sources = await call<{ sourceId: number; revision: number; enabled: boolean }[]>({
        url: '/user/storage/sources',
      })
      const next = JSON.stringify(sources.map((s) => [s.sourceId, s.revision, s.enabled]))
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
    timer = setInterval(poll, 15_000)
  })
  onUnmounted(() => {
    stopped = true
    clearInterval(timer)
  })
}
