import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import {
  aiCall,
  AiError,
  type AiCapabilities,
  type AiModelKind,
  type AiSearchResponse,
  type BrowserTextModel,
  type Placement,
} from '@/lib/ai'
import { configFromCapabilities, createBrowserAi, type BrowserAi } from '@/lib/browserAi'

/** Capabilities are re-read at most this often when a caller just needs "the current ones". */
const FRESH_MS = 30_000

let shared: BrowserAi | undefined
/** One engine per page: models are large and WebAssembly memory is never handed back. */
export const sharedBrowserAi = () => (shared ??= createBrowserAi())
/** Test hook. */
export const setSharedBrowserAi = (ai: BrowserAi | undefined) => (shared = ai)

export const useBrowserAiStore = defineStore('browserAi', () => {
  const auth = useAuthStore()
  const ai = sharedBrowserAi()
  const state = ai.state
  const capabilities = ref<AiCapabilities | null>(null)
  const fetchedAt = ref(0)
  const saved = ref({ text: false, image: false })
  const storage = ref<{ usage: number; quota: number; persisted: boolean } | null>(null)

  function placementOf(kind: AiModelKind, caps = capabilities.value): Placement {
    return caps?.placement?.[kind] ?? 'server'
  }
  const config = computed(() =>
    configFromCapabilities(capabilities.value?.browser, capabilities.value?.modelBaseUrl),
  )
  const usable = (kind: AiModelKind, caps = capabilities.value) =>
    Boolean(
      caps?.ready &&
      caps.enabled &&
      caps.globalEnabled &&
      placementOf(kind, caps) === 'browser' &&
      configFromCapabilities(caps.browser, caps.modelBaseUrl) &&
      ai.supported(kind),
    )
  const imageInBrowser = computed(() => usable('image'))
  const textInBrowser = computed(() => usable('text'))
  const choosable = computed(() => ({
    image: capabilities.value?.placement?.choosable?.image ?? false,
    text: capabilities.value?.placement?.choosable?.text ?? false,
  }))

  /** Feeds fresh capabilities in (the search composables already poll them). */
  function setCapabilities(next: AiCapabilities | null) {
    capabilities.value = next
    fetchedAt.value = Date.now()
    ai.configure(next ? configFromCapabilities(next.browser, next.modelBaseUrl) : null)
  }
  async function refreshCapabilities(force = false) {
    if (!auth.token) return null
    if (!force && capabilities.value && Date.now() - fetchedAt.value < FRESH_MS)
      return capabilities.value
    const token = auth.token
    try {
      const next = await aiCall<AiCapabilities>({ url: '/ai/capabilities', timeout: 8000 })
      if (token === auth.token) setCapabilities(next)
      return next
    } catch {
      return capabilities.value
    }
  }

  function failure(kind: AiModelKind, e: unknown) {
    if (e instanceof AiError) return e
    console.error(e)
    return new AiError(kind === 'text' ? 'browser_search_failed' : 'browser_index_failed')
  }

  /**
   * Text search: embeds the query here when the model runs in this browser and sends only the
   * vector; otherwise (or with `server`) the server embeds it.
   */
  async function searchText(
    text: string,
    extra: Record<string, unknown>,
    options: { signal?: AbortSignal; caps?: AiCapabilities | null; server?: boolean } = {},
  ): Promise<AiSearchResponse> {
    const caps = options.caps ?? capabilities.value
    if (!options.server && caps && usable('text', caps)) {
      if (!state.configured || ai.getConfig()?.fingerprint !== caps.browser?.fingerprint)
        setCapabilities(caps)
      let vector: number[]
      try {
        vector = await ai.embedText(text)
      } catch (e) {
        throw failure('text', e)
      }
      return aiCall<AiSearchResponse>({
        method: 'post',
        url: '/ai/search/vector',
        signal: options.signal,
        data: { ...extra, fingerprint: caps.browser!.fingerprint, vector },
      })
    }
    return aiCall<AiSearchResponse>({
      method: 'post',
      url: '/ai/search/text',
      signal: options.signal,
      data: { ...extra, text },
    })
  }

  /** Embeds a photo in this browser. Throws when the model cannot run here. */
  async function embedPhoto(file: Blob) {
    const fingerprint = capabilities.value?.browser?.fingerprint
    if (!fingerprint) throw new AiError('browser_index_failed')
    try {
      return { fingerprint, vector: await ai.embedImage(file) }
    } catch (e) {
      throw failure('image', e)
    }
  }

  /** Saves this user's choice where the admin allows one, then re-reads what is effective. */
  async function setPreference(kind: AiModelKind, where: Placement) {
    await aiCall({ method: 'put', url: '/ai/preference', data: { [kind]: where } })
    await refreshCapabilities(true)
  }

  async function setTextModel(size: BrowserTextModel) {
    await ai.setTextModel(size)
    await refreshSaved()
  }

  async function refreshSaved() {
    saved.value = await ai.savedOnDevice().catch(() => ({ text: false, image: false }))
    if (navigator.storage?.estimate) {
      try {
        const [{ usage, quota }, persisted] = await Promise.all([
          navigator.storage.estimate(),
          navigator.storage.persisted(),
        ])
        storage.value = { usage: usage ?? 0, quota: quota ?? 0, persisted }
      } catch {
        storage.value = null
      }
    }
  }
  /** Asks the browser to keep the saved models when disk space runs low. */
  async function keepOnDevice() {
    const granted = (await navigator.storage?.persist?.()) ?? false
    await refreshSaved()
    return granted
  }
  async function removeFromDevice() {
    const hadMemory = state.needsReload || state.ready.image || state.ready.text
    await ai.removeFromDevice()
    await refreshSaved()
    // WebAssembly memory only comes back with a reload.
    if (hadMemory) ai.reloadPage('Saved models removed; the page reloaded to give browser memory back.')
  }
  async function unload() {
    await ai.unload()
  }

  watch(
    () => auth.token,
    () => {
      capabilities.value = null
      fetchedAt.value = 0
      ai.configure(null)
    },
  )

  return {
    ai,
    state,
    capabilities,
    config,
    saved,
    storage,
    imageInBrowser,
    textInBrowser,
    choosable,
    placementOf,
    setCapabilities,
    refreshCapabilities,
    searchText,
    embedPhoto,
    setPreference,
    setTextModel,
    refreshSaved,
    keepOnDevice,
    removeFromDevice,
    unload,
  }
})
