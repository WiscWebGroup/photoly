import { computed, ref } from 'vue'
import { call } from '@/lib/http'

export interface StorageUsage {
  usedBytes: number
  /** Effective per-account limit; null means unlimited. */
  capacityBytes: number | null
  quotaSource?: 'user' | 'group'
  overQuota?: boolean
}

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** "48.2 of 200 GB": both numbers in the unit that suits the capacity. */
export function storageLabel({ usedBytes, capacityBytes }: StorageUsage) {
  if (capacityBytes === null) return `${(usedBytes / 1e9).toFixed(1)} GB · Unlimited`
  const power = Math.max(
    0,
    Math.min(UNITS.length - 1, Math.floor(Math.log10(capacityBytes || 1) / 3)),
  )
  const scale = 1000 ** power
  const digits = (n: number) => (n >= 100 || n === 0 ? 0 : 1)
  const used = usedBytes / scale
  const capacity = capacityBytes / scale
  return `${used.toFixed(digits(used))} of ${capacity.toFixed(digits(capacity))} ${UNITS[power]}`
}

/** The signed-in account's storage, re-read on demand (the shell refreshes it as pages change). */
export function useStorageUsage() {
  const usage = ref<StorageUsage | null>(null)
  let sequence = 0
  async function refresh() {
    const id = ++sequence
    try {
      const value = await call<StorageUsage>({ url: '/user/getStorage' })
      if (id === sequence) usage.value = typeof value?.usedBytes === 'number' ? value : null
    } catch {
      if (id === sequence) usage.value = null
    }
  }
  const percent = computed(() => {
    const value = usage.value
    if (!value || value.capacityBytes === null) return 0
    if (value.capacityBytes === 0) return value.usedBytes > 0 ? 100 : 0
    return Math.min(100, Math.round((value.usedBytes / value.capacityBytes) * 100))
  })
  return { usage, percent, refresh }
}
