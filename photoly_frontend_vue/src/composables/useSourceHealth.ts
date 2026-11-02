import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { call, ApiError } from '@/lib/http'
export interface SourceHealth {
  sourceId: number
  state: string
  checkedAt: string
}
export function sourceHealthMessage(state?: string) {
  const messages: Record<string, string> = {
    unreachable:
      'Photoly cannot connect to this storage source. Cached content is still available.',
    access_denied:
      'Storage authentication or permission was denied. Check the source credentials and permissions.',
    configuration_error:
      'This storage source needs attention. Check its connection settings and configured location.',
    disabled: 'This storage source is disabled.',
    read_failed: 'The storage source could not be read. Please retry.',
    file_missing: 'The file could not be found in storage.',
  }
  return messages[state ?? ''] ?? ''
}
export function mediaErrorMessage(error: unknown) {
  if (error instanceof ApiError && typeof error.detail === 'string') {
    if (error.detail === 'exif_parse_failed') return 'Could not parse this photo’s EXIF metadata.'
    return (
      sourceHealthMessage(error.detail.replace(/^storage_/, '')) || 'Could not read this media.'
    )
  }
  return 'Could not reach Photoly. Check your connection and retry.'
}
export function useSourceHealth(ids: Ref<number[]>) {
  const states = ref<Record<number, SourceHealth>>({}),
    error = ref(''),
    busy = ref(false)
  let sequence = 0,
    disposed = false
  const key = computed(() =>
    [...new Set(ids.value)]
      .filter((id) => id > 0)
      .sort((a, b) => a - b)
      .join(','),
  )
  async function refresh(force = false) {
    if (document.hidden || !key.value || disposed) return
    const seq = ++sequence
    busy.value = true
    try {
      const wanted = key.value.split(',')
      const result: SourceHealth[] = []
      for (let offset = 0; offset < wanted.length; offset += 100) {
        const batch = await call<SourceHealth[]>({
          url: '/user/storage/health',
          method: force ? 'post' : 'get',
          params: { ids: wanted.slice(offset, offset + 100).join(',') },
        })
        if (seq !== sequence || disposed) return
        result.push(...batch)
      }
      if (seq !== sequence || disposed) return
      states.value = Object.fromEntries(result.map((s) => [s.sourceId, s]))
      error.value = ''
    } catch {
      if (seq === sequence)
        error.value = 'Could not reach Photoly to check storage. Check your connection.'
    } finally {
      if (seq === sequence) busy.value = false
    }
  }
  watch(
    key,
    () => {
      sequence++
      states.value = {}
      error.value = ''
      busy.value = false
      void refresh()
    },
    { immediate: true },
  )
  const interval = window.setInterval(() => {
    if (!busy.value) void refresh()
  }, 30000)
  const visible = () => {
    if (!document.hidden) void refresh()
  }
  document.addEventListener('visibilitychange', visible)
  onBeforeUnmount(() => {
    disposed = true
    sequence++
    clearInterval(interval)
    document.removeEventListener('visibilitychange', visible)
  })
  return { states, error, busy, refresh }
}
