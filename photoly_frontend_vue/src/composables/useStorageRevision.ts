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
  scanAutomaticRetries?: number
  scanBlocked?: number
  capabilities?: { scan: boolean; write: boolean }
  scanDiscovered?: number
  scanWaiting?: number
  scanUnchanged?: number
  scanQueued?: boolean
}
const sources = ref<StorageStatus[]>([])
export const storageFolder = ref<{ nsId: number; sourceId: number; name: string } | null>(null)
type Subscriber = {
  refresh: () => unknown | Promise<unknown>
  busy: () => boolean
  revision?: string
  refreshing?: boolean
}
const subscribers = new Set<Subscriber>()
let timer: ReturnType<typeof setInterval> | undefined
let lastPoll = 0
let pending: Promise<void> | undefined
let generation = 0
let hasSnapshot = false

export function resetStorageState() {
  generation++
  hasSnapshot = false
  sources.value = []
  storageFolder.value = null
  lastPoll = 0
  for (const subscriber of subscribers) subscriber.revision = undefined
}
async function deliver() {
  if (!hasSnapshot) return
  const revision = JSON.stringify(sources.value.map((s) => [s.sourceId, s.revision, s.enabled]))
  for (const sub of subscribers) {
    if (
      sub.busy() ||
      sub.refreshing ||
      [...document.querySelectorAll('[role="dialog"], [role="alertdialog"]')].some(
        (el) => !el.closest('[data-storage-panel]'),
      )
    )
      continue
    if (sub.revision !== undefined && sub.revision !== revision) {
      sub.refreshing = true
      try {
        await sub.refresh()
      } catch {
        continue
      } finally {
        sub.refreshing = false
      }
    }
    sub.revision = revision
  }
}
async function poll(force = false) {
  if (document.hidden) return
  if (pending) return pending
  const interval = sources.value.some((s) => s.scanStatus === 'scanning' || s.scanQueued)
    ? 2000
    : 15000
  if (!force && Date.now() - lastPoll < interval) return deliver()
  const started = generation
  lastPoll = Date.now()
  pending = (async () => {
    try {
      const result = await call<StorageStatus[]>({ url: '/user/storage/sources' })
      if (generation !== started || !subscribers.size) return
      hasSnapshot = true
      sources.value = result
      await deliver()
    } catch {
      /* Keep the last status and retry without interrupting browsing. */
    } finally {
      pending = undefined
    }
  })()
  return pending
}
/** One polling loop per signed-in application, with independent busy/revision observers. */
export function useStorageRevision(refresh: () => unknown | Promise<unknown>, busy: () => boolean) {
  const subscriber: Subscriber = { refresh, busy }
  onMounted(() => {
    subscribers.add(subscriber)
    if (!timer) {
      void poll(true)
      timer = setInterval(() => void poll(), 2000)
    } else void deliver()
  })
  onUnmounted(() => {
    subscribers.delete(subscriber)
    if (!subscribers.size) {
      clearInterval(timer)
      timer = undefined
      resetStorageState()
    }
  })
  return { sources, poll: () => poll(true) }
}
