import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'
import { http, type Envelope } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

export interface PickedFile {
  file: File
  path: string
}
export interface UploadItem {
  id: string
  batchId: string
  path: string
  size: number
  state: string
  error: string | null
  photoId: number | null
  nsId: number | null
  sourceId: string | null
}
export interface UploadBatch {
  id: string
  name: string
  nsId: number | null
  sealed: boolean
  cancelled: boolean
  createdAt: string
  counts: Record<string, { count: number; bytes: number }>
}
interface LocalItem extends PickedFile {
  id: string
  batchId: string
  state: string
  sent: number
  error?: string
}
const media = new Set(
  'jpg jpeg gif png webp ico bmp tif tiff svg psd raw arw pcd jfif mp4 avi wmv mpeg m4v mov flv asf f4v rmvb vob rm zip'.split(
    ' ',
  ),
)
export const supportedUpload = (path: string) =>
  media.has(path.split('.').at(-1)?.toLowerCase() ?? '')
export const bytesLabel = (bytes: number) =>
  bytes >= 1e9
    ? `${(bytes / 1e9).toFixed(1)} GB`
    : bytes >= 1e6
      ? `${(bytes / 1e6).toFixed(1)} MB`
      : `${Math.ceil(bytes / 1000)} KB`
export const uploadError = (reason: string | null) =>
  ({
    source_changed: 'The selected file differs from the original upload',
    source_expired: 'Source expired. Reselect the original file or ZIP.',
    unsupported_file_type: 'Skipped: not a supported photo or video',
    storage_quota_exceeded: 'Storage limit reached',
    file_too_large: 'File exceeds the request size limit',
    destination_deleted: 'Destination folder was deleted',
    unsafe_path: 'Unsafe file path',
    unsafe_or_encrypted_zip: 'ZIP contains unsafe or encrypted entries',
    zip_expansion_limit: 'ZIP exceeds the extraction budget',
    invalid_zip: 'Invalid or damaged ZIP',
    processing_failed: 'Could not process this file',
    insufficient_disk_space: 'Server disk space is low',
    transmission_expired: 'Transmission expired. Reselect files to retry.',
    file_size_mismatch: 'File size did not match',
  })[reason ?? ''] ??
  reason ??
  ''

export async function uploadCall<T>(
  url: string,
  method = 'get',
  data?: unknown,
  signal?: AbortSignal,
) {
  const response = await http.request<Envelope<T>>({ url, method, data, signal })
  if (response.data.msgCode !== 200) throw new Error(uploadError(String(response.data.t)))
  return response.data.t
}

export const useUploadsStore = defineStore('uploads', () => {
  const auth = useAuthStore()
  const open = ref(false)
  const pickerOpen = ref(false)
  const target = ref<{ nsId: number; name: string } | null>(null)
  const resumeBatch = ref<string | null>(null)
  const batches = ref<UploadBatch[]>([])
  const total = ref(0)
  const page = ref(1)
  const local = ref<LocalItem[]>([])
  const maxFileBytes = ref(600 * 1024 ** 2)
  const revision = ref(0)
  const error = ref('')
  const submitting = ref(false)
  let generation = 0
  let refreshing = false
  let running = 0
  let refreshSequence = 0
  const controllers = new Map<string, AbortController>()
  let sessionController = new AbortController()
  const serverActive = ref(0)
  const serverFailed = ref(0)
  const active = computed(() =>
    Math.max(
      local.value.filter((i) => ['pending', 'sending'].includes(i.state)).length,
      serverActive.value,
      batches.value.reduce(
        (n, b) =>
          n + ['pending', 'ready', 'processing'].reduce((s, k) => s + (b.counts[k]?.count ?? 0), 0),
        0,
      ),
    ),
  )
  const failed = computed(
    () =>
      serverFailed.value > 0 ||
      local.value.some((i) => i.state === 'failed') ||
      batches.value.some((b) => b.counts.failed?.count),
  )
  const progress = computed(() => {
    const bytes = local.value.reduce((n, i) => n + i.file.size, 0)
    return bytes ? Math.round((local.value.reduce((n, i) => n + i.sent, 0) / bytes) * 100) : 0
  })

  watch(
    () => auth.token,
    () => {
      generation++
      refreshSequence++
      sessionController.abort()
      sessionController = new AbortController()
      controllers.forEach((c) => c.abort())
      controllers.clear()
      running = 0
      refreshing = false
      submitting.value = false
      local.value = []
      batches.value = []
      serverActive.value = 0
      serverFailed.value = 0
      total.value = 0
      page.value = 1
      open.value = false
      pickerOpen.value = false
      target.value = null
      error.value = ''
    },
    { flush: 'sync' },
  )

  async function refresh() {
    if (!auth.token || refreshing) return
    refreshing = true
    const seq = ++refreshSequence
    try {
      const data = await uploadCall<{
        batches: UploadBatch[]
        total: number
        maxFileBytes: number
        activeCount?: number
        failedCount?: number
      }>(`/uploads?page=${page.value}`, 'get', undefined, sessionController.signal)
      if (seq !== refreshSequence) return
      if (
        JSON.stringify(data.batches) !== JSON.stringify(batches.value) ||
        (data.activeCount ?? 0) !== serverActive.value
      )
        revision.value++
      serverActive.value = data.activeCount ?? 0
      serverFailed.value = data.failedCount ?? 0
      batches.value = data.batches
      total.value = data.total
      maxFileBytes.value = data.maxFileBytes
      error.value = ''
    } catch (e) {
      if (seq === refreshSequence)
        error.value = e instanceof Error ? e.message : 'Could not load uploads'
    } finally {
      if (seq === refreshSequence) refreshing = false
    }
  }

  function start(nsId: number, name: string, resume: string | null = null) {
    target.value = { nsId, name }
    resumeBatch.value = resume
    open.value = false
    pickerOpen.value = true
  }

  async function rootPicker() {
    try {
      const root = await uploadCall<{ nsId: number }>('/namespace/getRoot')
      start(root.nsId, 'Library')
    } catch {
      toast.error('Could not open your library')
    }
  }

  async function allItems(batchId: string) {
    const items: UploadItem[] = []
    let p = 1
    do {
      const data = await uploadCall<{ items: UploadItem[]; total: number }>(
        `/uploads/${batchId}/items?page=${p++}`,
        'get',
        undefined,
        sessionController.signal,
      )
      items.push(...data.items)
      if (items.length >= data.total) break
    } while (true)
    return items
  }

  async function enqueue(picked: PickedFile[]) {
    if (!target.value || !picked.length || submitting.value) return
    submitting.value = true
    const gen = generation
    const signal = sessionController.signal
    try {
      const batchId = resumeBatch.value ?? crypto.randomUUID()
      let jobs: LocalItem[]
      if (resumeBatch.value) {
        const pending = (await allItems(batchId)).filter(
          (i) => i.state === 'pending' && !i.sourceId,
        )
        const matches = new Map(pending.map((i) => [`${i.path}\0${i.size}`, i]))
        jobs = picked.flatMap((p) => {
          const item = matches.get(`${p.path}\0${p.file.size}`)
          if (!item) return []
          matches.delete(`${p.path}\0${p.file.size}`)
          return [{ ...p, id: item.id, batchId, state: 'pending', sent: 0 }]
        })
        if (!jobs.length) throw new Error('No selected files match the pending paths and sizes.')
      } else {
        const name = picked.length === 1 ? picked[0]!.path.split('/')[0]! : `${picked.length} files`
        await uploadCall('/uploads', 'post', { id: batchId, nsId: target.value.nsId, name }, signal)
        jobs = picked.map((p) => ({
          ...p,
          id: crypto.randomUUID(),
          batchId,
          state: 'pending',
          sent: 0,
        }))
        for (let offset = 0; offset < jobs.length; offset += 500) {
          await uploadCall(
            `/uploads/${batchId}/items`,
            'post',
            {
              items: jobs
                .slice(offset, offset + 500)
                .map((i) => ({ id: i.id, path: i.path, size: i.file.size })),
            },
            signal,
          )
        }
      }
      if (gen !== generation) return
      const eligible = jobs.filter(
        (i) => supportedUpload(i.path) && i.file.size <= maxFileBytes.value,
      )
      const keys = new Set(eligible.map((i) => i.id))
      local.value = [...local.value.filter((i) => !keys.has(i.id)), ...eligible]
      await uploadCall(`/uploads/${batchId}/finish`, 'post', undefined, signal)
      if (gen !== generation) return
      pickerOpen.value = false
      open.value = true
      page.value = 1
      toast.success('Upload queued. You can keep browsing.')
      pump()
      await refresh()
    } catch (e) {
      if (gen === generation) toast.error(e instanceof Error ? e.message : 'Could not queue upload')
    } finally {
      if (gen === generation) submitting.value = false
    }
  }

  function pump() {
    while (running < 2) {
      const item = local.value.find((i) => i.state === 'pending')
      if (!item) break
      item.state = 'sending'
      running++
      void send(item, generation)
    }
  }
  async function send(item: LocalItem, gen: number) {
    const controller = new AbortController()
    controllers.set(item.id, controller)
    try {
      const { data } = await http.put<Envelope<UploadItem>>(
        `/uploads/${item.batchId}/items/${item.id}/content`,
        item.file,
        {
          headers: { 'Content-Type': 'application/octet-stream' },
          signal: controller.signal,
          onUploadProgress: (e) => {
            if (gen === generation) item.sent = Math.min(item.file.size, e.loaded)
          },
        },
      )
      if (data.msgCode !== 200) throw new Error(uploadError(String(data.t)))
      if (gen !== generation) return
      item.state = data.t.state
      item.sent = item.file.size
      // Release potentially large File objects as soon as the server owns the bytes.
      local.value = local.value.filter((i) => i.id !== item.id)
    } catch (e) {
      if (gen === generation && item.state !== 'cancelled') {
        item.state = 'failed'
        item.error = e instanceof Error ? e.message : 'Transmission failed'
      }
    } finally {
      if (gen === generation) {
        controllers.delete(item.id)
        running--
        pump()
        if (!local.value.some((i) => ['pending', 'sending'].includes(i.state))) void refresh()
      }
    }
  }

  async function action(batchId: string, action: 'cancel' | 'retry') {
    try {
      if (action === 'cancel')
        for (const item of local.value.filter((i) => i.batchId === batchId)) {
          item.state = 'cancelled'
          controllers.get(item.id)?.abort()
        }
      await uploadCall(`/uploads/${batchId}/${action}`, 'post', undefined, sessionController.signal)
      if (action === 'retry') {
        const pending = new Set(
          (await allItems(batchId)).filter((i) => i.state === 'pending').map((i) => i.id),
        )
        for (const item of local.value.filter((i) => i.batchId === batchId && pending.has(i.id))) {
          item.state = 'pending'
          item.sent = 0
          item.error = undefined
        }
        pump()
      } else local.value = local.value.filter((i) => i.batchId !== batchId)
      await refresh()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not update upload')
    }
  }
  return {
    open,
    pickerOpen,
    target,
    resumeBatch,
    batches,
    total,
    page,
    local,
    maxFileBytes,
    revision,
    error,
    submitting,
    active,
    failed,
    progress,
    refresh,
    start,
    rootPicker,
    enqueue,
    action,
  }
})
