import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'
import { http, type Envelope } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'
import { newUuid } from '@/lib/uuid'
import { useBrowserAiStore } from '@/stores/browserAi'

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
  acknowledged?: boolean
  /** Embed this photo in the browser once the server has the bytes. */
  index?: boolean
}
/** Photos too odd for createImageBitmap or not photos at all stay with the server. */
const browserIndexable = (path: string) =>
  /\.(jpe?g|png|webp|gif|bmp|ico|jfif)$/i.test(path)
export interface IndexingProgress {
  total: number
  done: number
  /** Photos this browser gave up on; the server indexes them. */
  failed: number
  current: string
}
const media = new Set(
  'jpg jpeg gif png webp ico bmp tif tiff svg psd raw arw pcd jfif heic heif mp4 avi wmv mpeg m4v mov flv asf f4v rmvb vob rm zip'.split(
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
    upload_still_active: 'Cancel remaining work before clearing this activity',
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
    online_download_disabled: 'Online Download is disabled by the administrator',
    non_public_image_url: 'Only public HTTP or HTTPS image links are allowed',
    invalid_image_url: 'Enter a public HTTP or HTTPS image link',
    invalid_image: 'The source is not a supported image',
    download_timeout: 'The image download timed out',
    download_too_large: 'The source image exceeds the download limit',
    download_failed: 'Could not download the image',
    download_http_error: 'The image server did not return a successful response',
    too_many_redirects: 'The image link redirects too many times',
    conversion_failed: 'Could not convert the image',
    video_conversion_failed: 'Could not generate the Live Photo playback copy',
    conversion_timeout: 'Image conversion timed out',
    animation_too_large: 'This animation exceeds the conversion budget',
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
  const currentFolder = ref<{ nsId: number; name: string } | null>(null)
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
  const managing = ref(false)
  let generation = 0
  let refreshing = false
  let running = 0
  let refreshSequence = 0
  const controllers = new Map<string, AbortController>()
  const browserAi = useBrowserAiStore()
  const indexing = ref<IndexingProgress>({ total: 0, done: 0, failed: 0, current: '' })
  const indexQueue: { id: string; batchId: string; path: string; file: File }[] = []
  let indexRunning = false
  let sessionController = new AbortController()
  const serverActive = ref(0)
  const serverFailed = ref(0)
  const active = computed(() =>
    Math.max(
      local.value.filter((i) => ['pending', 'sending'].includes(i.state)).length,
      serverActive.value,
      batches.value.reduce(
        (n, b) =>
          n +
          ['pending', 'ready', 'processing', 'waiting'].reduce(
            (s, k) => s + (b.counts[k]?.count ?? 0),
            0,
          ),
        0,
      ),
    ),
  )
  const failed = computed(
    () =>
      serverFailed.value > 0 || local.value.some((i) => i.state === 'failed' && !i.acknowledged),
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
      managing.value = false
      local.value = []
      batches.value = []
      serverActive.value = 0
      serverFailed.value = 0
      indexQueue.length = 0
      indexing.value = { total: 0, done: 0, failed: 0, current: '' }
      total.value = 0
      page.value = 1
      open.value = false
      pickerOpen.value = false
      target.value = null
      currentFolder.value = null
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
      for (const batch of data.batches) {
        const old = batches.value.find((b) => b.id === batch.id)
        const pending = (b: UploadBatch) =>
          ['pending', 'ready', 'processing', 'waiting'].some((state) => b.counts[state]?.count)
        if (old && pending(old) && !pending(batch) && !batch.cancelled) {
          if (batch.counts.failed?.count)
            toast.error(`${batch.name}: some items failed. Open Upload activity to retry.`)
          else toast.success(`${batch.name} complete`)
        }
      }
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
      const batchId = resumeBatch.value ?? newUuid()
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
          id: newUuid(),
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
      // Placement can change between uploads, so ask once per batch.
      await browserAi.refreshCapabilities(true)
      if (browserAi.imageInBrowser) {
        // A finished run starts over; counts only grow while photos are still being indexed.
        if (!indexQueue.length && !indexRunning)
          indexing.value = { total: 0, done: 0, failed: 0, current: '' }
        for (const job of eligible) job.index = browserIndexable(job.path)
      }
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
      if (item.index) queueIndex(item, gen)
      // Release potentially large File objects as soon as the server owns the bytes.
      local.value = local.value.filter((i) => i.id !== item.id)
    } catch (e) {
      if (gen === generation && item.state !== 'cancelled') {
        item.state = 'failed'
        item.acknowledged = false
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

  /** Embeds photos here, one at a time, after the server has their bytes (plan section 5). */
  function queueIndex(item: LocalItem, gen: number) {
    indexQueue.push({ id: item.id, batchId: item.batchId, path: item.path, file: item.file })
    indexing.value.total++
    void runIndex(gen)
  }
  async function runIndex(gen: number) {
    if (indexRunning) return
    indexRunning = true
    try {
      while (indexQueue.length && gen === generation) {
        const job = indexQueue.shift()!
        indexing.value.current = job.path
        try {
          // Once the model cannot run here, the server takes everything that is left.
          if (browserAi.state.phase === 'error') throw new Error('browser model unavailable')
          const { fingerprint, vector } = await browserAi.embedPhoto(job.file)
          await uploadCall(
            `/uploads/${job.batchId}/items/${job.id}/embedding`,
            'put',
            { fingerprint, vector },
            sessionController.signal,
          )
          if (gen === generation) indexing.value.done++
        } catch {
          // The server queue indexes anything the browser did not.
          if (gen === generation) indexing.value.failed++
        }
      }
    } finally {
      indexRunning = false
      if (gen === generation) indexing.value.current = ''
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
      } else {
        local.value = local.value.filter((i) => i.batchId !== batchId)
        for (let i = indexQueue.length - 1; i >= 0; i--)
          if (indexQueue[i]!.batchId === batchId) {
            indexQueue.splice(i, 1)
            indexing.value.total--
          }
      }
      await refresh()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not update upload')
    }
  }
  async function manageActivity(kind: 'clear' | 'acknowledge', batchId?: string) {
    if (managing.value) return false
    managing.value = true
    const gen = generation
    // Only acknowledge failures that were already visible when the action started.
    const failures = local.value.filter((i) => i.state === 'failed')
    try {
      const removed = await uploadCall<string[]>(
        kind === 'clear'
          ? `/uploads${batchId ? `/${batchId}` : ''}`
          : '/uploads/acknowledge-failures',
        kind === 'clear' ? 'delete' : 'post',
        undefined,
        sessionController.signal,
      )
      if (gen !== generation) return false
      if (kind === 'clear') {
        local.value = local.value.filter((i) => !removed.includes(i.batchId))
        page.value = 1
      } else {
        failures.forEach((i) => {
          if (i.state === 'failed') i.acknowledged = true
        })
      }
      // Invalidate an older poll so it cannot restore a cleared badge/history row.
      refreshSequence++
      refreshing = false
      await refresh()
      return true
    } catch (e) {
      if (gen === generation)
        toast.error(e instanceof Error ? e.message : 'Could not update activity')
      return false
    } finally {
      if (gen === generation) managing.value = false
    }
  }
  return {
    manageActivity,
    managing,
    open,
    pickerOpen,
    target,
    currentFolder,
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
    indexing,
    refresh,
    start,
    rootPicker,
    enqueue,
    action,
  }
})
