import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useBrowserAiStore } from '@/stores/browserAi'
import {
  aiCall,
  aiMessage,
  type AiCapabilities,
  type AiFilters,
  type AiSearchResponse,
} from '@/lib/ai'

export function useAiSearch() {
  const auth = useAuthStore()
  const browser = useBrowserAiStore()
  const capabilities = ref<AiCapabilities | null>(null)
  const result = ref<AiSearchResponse | null>(null)
  const busy = ref(false)
  const error = ref('')
  const notice = ref('')
  let sequence = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  let last: { mode: 'text' | 'image' | 'similar'; value: string | File | number; filters: AiFilters } | null = null
  /** "Search on the server this time": repeats the last query without the browser model. */
  function searchOnServer() {
    if (last) return search(last.mode, last.value, last.filters, { server: true })
  }
  function cancel() {
    sequence++
    controller?.abort()
    busy.value = false
  }
  function clear() {
    cancel()
    result.value = null
    error.value = ''
    notice.value = ''
  }
  async function refreshCapabilities() {
    const token = auth.token
    if (!token) return
    try {
      const next = await aiCall<AiCapabilities>({ url: '/ai/capabilities', timeout: 8000 })
      if (disposed || token !== auth.token) return
      if (
        !next.ready ||
        !next.enabled ||
        !next.globalEnabled ||
        (capabilities.value && capabilities.value.profileId !== next.profileId)
      )
        clear()
      capabilities.value = next
      browser.setCapabilities(next)
    } catch (e) {
      if (!disposed && token === auth.token) error.value = aiMessage(e)
    }
  }
  async function search(
    mode: 'text' | 'image' | 'similar',
    value: string | File | number,
    filters: AiFilters,
    options: { server?: boolean } = {},
  ) {
    last = { mode, value, filters }
    cancel()
    result.value = null
    error.value = ''
    notice.value = ''
    const id = sequence,
      token = auth.token
    controller = new AbortController()
    busy.value = true
    try {
      const data = new FormData()
      if (mode === 'image') {
        data.set('file', value as File)
        Object.entries(filters).forEach(([k, v]) => v !== undefined && data.set(k, String(v)))
      }
      // Text queries may be embedded in this browser; image search and "similar" stay on the server.
      const response =
        mode === 'text'
          ? await browser.searchText(value as string, { ...filters }, {
              signal: controller.signal,
              caps: capabilities.value,
              server: options.server,
            })
          : await aiCall<AiSearchResponse>({
              method: 'post',
              signal: controller.signal,
              url: mode === 'similar' ? `/ai/photos/${value}/similar` : `/ai/search/${mode}`,
              data: mode === 'image' ? data : { ...filters },
            })
      if (id === sequence && token === auth.token) result.value = response
    } catch (e) {
      if (id === sequence && token === auth.token) {
        error.value = aiMessage(e)
        void refreshCapabilities()
      }
    } finally {
      if (id === sequence) busy.value = false
    }
  }
  async function poll() {
    await refreshCapabilities()
    if (!disposed) timer = setTimeout(poll, document.hidden ? 30000 : 15000)
  }
  watch(
    () => auth.token,
    () => {
      clear()
      capabilities.value = null
      void refreshCapabilities()
    },
  )
  onMounted(poll)
  onBeforeUnmount(() => {
    disposed = true
    clearTimeout(timer)
    cancel()
  })
  return { capabilities, result, busy, error, notice, search, searchOnServer, cancel, clear, refreshCapabilities }
}
