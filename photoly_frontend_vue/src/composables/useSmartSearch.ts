import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Photo } from './usePhotos'
import { useAuthStore } from '@/stores/auth'
import { useBrowserAiStore } from '@/stores/browserAi'
import { call } from '@/lib/http'
import { aiCall, type AiCapabilities } from '@/lib/ai'
import { fuseSearchResults, SMART_SEARCH_LIMIT } from '@/lib/smartSearch'

export function useSmartSearch() {
  const auth = useAuthStore()
  const browser = useBrowserAiStore()
  let lastQuery = ''
  const textResults = ref<Photo[]>([])
  const semanticResults = ref<Photo[]>([])
  const textBusy = ref(false)
  const semanticBusy = ref(false)
  const textNotice = ref('')
  const semanticNotice = ref('')
  const error = ref('')
  const submitted = ref(false)
  let sequence = 0
  let controller: AbortController | undefined
  const results = computed(() => fuseSearchResults(textResults.value, semanticResults.value))
  const busy = computed(() => textBusy.value || semanticBusy.value)
  const limited = computed(() => results.value.length === SMART_SEARCH_LIMIT)

  function clear() {
    sequence++
    controller?.abort()
    textResults.value = []
    semanticResults.value = []
    textBusy.value = semanticBusy.value = false
    textNotice.value = semanticNotice.value = error.value = ''
    submitted.value = false
  }
  async function search(query: string, options: { server?: boolean } = {}) {
    clear()
    lastQuery = query
    if (query.length > 512) {
      error.value = 'Keep your search to 512 characters or fewer.'
      return
    }
    const text = query.trim()
    if (!text) return
    submitted.value = true
    const id = sequence,
      token = auth.token
    const current = () => id === sequence && token === auth.token
    controller = new AbortController()
    const signal = controller.signal
    textBusy.value = semanticBusy.value = true
    await Promise.all([
      (async () => {
        try {
          const photos = await call<Photo[]>({
            url: '/photo/searchSmart',
            params: { query: text },
            signal,
            timeout: 15000,
          })
          if (current()) textResults.value = photos
        } catch {
          if (current())
            textNotice.value = 'Name, tag and gallery search is unavailable. Try again.'
        } finally {
          if (current()) textBusy.value = false
        }
      })(),
      (async () => {
        try {
          const caps = await aiCall<AiCapabilities>({
            url: '/ai/capabilities',
            signal,
            timeout: 8000,
          })
          if (!current()) return
          if (!caps.ready || !caps.enabled || !caps.globalEnabled) {
            semanticNotice.value =
              'Semantic search is unavailable. Showing name, tag and gallery matches.'
            return
          }
          browser.setCapabilities(caps)
          const response = await browser.searchText(
            text,
            { topK: SMART_SEARCH_LIMIT },
            { signal, caps, server: options.server },
          )
          if (current()) semanticResults.value = response.items.map((hit) => hit.photo)
        } catch {
          if (current())
            semanticNotice.value =
              'Semantic search could not finish. Showing name, tag and gallery matches.'
        } finally {
          if (current()) semanticBusy.value = false
        }
      })(),
    ])
  }
  watch(() => auth.token, clear)
  onBeforeUnmount(clear)
  return {
    results,
    busy,
    semanticBusy,
    limited,
    submitted,
    error,
    textNotice,
    semanticNotice,
    search,
    searchOnServer: () => search(lastQuery, { server: true }),
    clear,
  }
}
