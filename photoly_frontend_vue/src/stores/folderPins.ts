import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'
import { call } from '@/lib/http'
import type { Folder } from '@/composables/useFolders'
import { useAuthStore } from '@/stores/auth'

export interface PinnedFolder extends Folder {
  path: string
}
export interface FolderPins {
  items: PinnedFolder[]
  defaultNsId: number | null
}

export const useFolderPinsStore = defineStore('folderPins', () => {
  const auth = useAuthStore()
  const items = ref<PinnedFolder[]>([])
  const defaultNsId = ref<number | null>(null)
  // One-shot startup intent; ordinary navigation to the same folder never falls back.
  const startupFolderId = ref<number | null>(null)
  const loaded = ref(false)
  const saving = ref(false)
  let revision = 0
  let pending: Promise<FolderPins> | undefined
  let mutationController: AbortController | undefined

  watch(
    () => auth.token,
    () => {
      mutationController?.abort()
      mutationController = undefined
      revision++
      items.value = []
      defaultNsId.value = startupFolderId.value = null
      loaded.value = saving.value = false
      pending = undefined
    },
    { flush: 'sync' },
  )

  const snapshot = (): FolderPins => ({ items: items.value, defaultNsId: defaultNsId.value })
  function apply(result: FolderPins) {
    items.value = result.items
    defaultNsId.value = result.defaultNsId
    loaded.value = true
  }
  function load(force = false): Promise<FolderPins> {
    if (saving.value || (loaded.value && !force)) return Promise.resolve(snapshot())
    if (pending) return pending
    const started = revision
    const token = auth.token
    const request = call<FolderPins>({ url: '/namespace/pins', timeout: 10000 })
      .then((result) => {
        if (token !== auth.token) throw new Error('Account changed')
        if (started === revision) apply(result)
        return snapshot()
      })
      .finally(() => {
        if (pending === request) pending = undefined
      })
    pending = request
    return request
  }
  async function change(action: 'pin' | 'unpin' | 'defaultLocation', nsId: number | null) {
    if (saving.value) return
    const started = ++revision
    const controller = new AbortController()
    mutationController = controller
    pending = undefined
    saving.value = true
    try {
      const result = await call<FolderPins>({
        method: 'post',
        url: `/namespace/${action}`,
        data: { nsId },
        timeout: 10000,
        signal: controller.signal,
      })
      if (started === revision) apply(result)
    } catch {
      if (started === revision) toast.error('Could not save pinned folders. Please try again.')
    } finally {
      if (mutationController === controller) mutationController = undefined
      if (started === revision) saving.value = false
    }
  }
  const has = (id: number) => items.value.some((item) => item.nsId === id)
  return {
    items,
    defaultNsId,
    startupFolderId,
    loaded,
    saving,
    load,
    has,
    pin: (id: number) => change('pin', id),
    unpin: (id: number) => change('unpin', id),
    setDefault: (id: number | null) => change('defaultLocation', id),
  }
})
