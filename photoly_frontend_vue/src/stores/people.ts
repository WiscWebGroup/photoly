import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { call, http } from '@/lib/http'
import type { FaceDetection } from '@/lib/peopleInference'
import type { FaceManifest } from '@/lib/people.worker'

export interface PeoplePreference {
  enabled: boolean
  paused: boolean
  placement: 'browser' | 'server'
  generation: number
  available: boolean
  visible: boolean
  choosable: boolean
  reason: string
}
export interface PeopleStatus {
  waitReason?: string
  preference: PeoplePreference
  total: number
  ready: number
  failed: number
  processing: number
  pending: number
}
export interface Person {
  id: number
  name: string
  hidden: boolean
  count: number
  coverFaceId: number | null
}
export interface PersonFace {
  id: number
  photoId: number
  personId: number
  name: string
  box: number[]
  dismissed: boolean
}
interface FaceJob {
  id: number
  token: string
  photoId: number
  version: number
  fingerprint: string
  generation: number
  placement: string
  status: string
}
export interface PeoplePage<T> {
  items: T[]
  nextAfter: number | null
}
export const peopleLabel = (reason: string) =>
  ({
    ai_disabled: 'AI is disabled by the administrator.',
    people_disabled: 'People is disabled by the administrator.',
    ai_permission_required: 'AI access is required.',
    user_disabled: 'Enable People to organize faces in your library.',
    paused: 'Processing paused.',
    cpu_busy_or_unknown: 'Waiting for available CPU resources.',
    memory_pressure: 'Waiting for available memory.',
    quota_exceeded: 'Waiting for more AI database storage budget.',
    disk_pressure: 'Waiting for available disk space.',
    interval_wait: 'Waiting for the next scheduled processing slot.',
    rate_limited: 'Waiting for the administrator’s processing rate limit.',
    maintenance: 'Waiting for maintenance to finish.',
    scheduler_paused: 'Waiting for the AI scheduler to resume.',
    scheduler_stopped: 'Waiting for the AI scheduler to start.',
  })[reason] ?? reason

export const usePeopleStore = defineStore('people', () => {
  const auth = useAuthStore(),
    preference = ref<PeoplePreference | null>(null),
    status = ref<PeopleStatus | null>(null),
    model = ref<FaceManifest | null>(null)
  const device = ref(''),
    message = ref(''),
    error = ref(''),
    revision = ref(0),
    working = ref(false)
  const visible = computed(() => Boolean(auth.token && preference.value?.visible))
  let timer: ReturnType<typeof setTimeout> | undefined,
    worker: Worker | undefined,
    epoch = 0,
    started = false,
    refreshPromise: Promise<void> | null = null
  let loop = 0
  let lastRefresh = 0
  let cancelInference: (() => void) | undefined
  function stopWork() {
    epoch++
    cancelInference?.()
    cancelInference = undefined
    worker?.terminate()
    worker = undefined
    working.value = false
    device.value = ''
    message.value = ''
  }
  async function refresh() {
    if (refreshPromise) return refreshPromise
    const token = auth.token,
      requestEpoch = epoch
    if (!token) return
    refreshPromise = (async () => {
      try {
        const data = await call<{ preference: PeoplePreference; model: FaceManifest }>({
          url: '/ai/people/settings',
        })
        if (auth.token !== token || epoch !== requestEpoch) return
        if (
          typeof data?.preference?.visible !== 'boolean' ||
          typeof data.preference.enabled !== 'boolean'
        )
          throw Error('Invalid People settings')
        lastRefresh = Date.now()
        const previous = preference.value
        preference.value = data.preference
        model.value = data.model
        if (
          previous &&
          (previous.generation !== data.preference.generation ||
            previous.placement !== data.preference.placement ||
            !data.preference.visible ||
            data.preference.paused)
        )
          stopWork()
        const progressEpoch = epoch
        const progress = await call<PeopleStatus>({ url: '/ai/people/status' })
        if (auth.token === token && epoch === progressEpoch) {
          if (status.value?.ready !== progress.ready) revision.value++
          status.value = progress
        }
      } catch {
        if (auth.token === token) {
          preference.value = null
          error.value = 'Could not load People settings.'
          stopWork()
        }
      } finally {
        refreshPromise = null
      }
    })()
    return refreshPromise
  }
  function infer(image: ArrayBuffer, manifest: FaceManifest) {
    worker ??= new Worker(new URL('../lib/people.worker.ts', import.meta.url), { type: 'module' })
    return new Promise<FaceDetection[]>((resolve, reject) => {
      const deadline = setTimeout(() => {
        worker?.terminate()
        worker = undefined
        cancelInference = undefined
        reject(Error('Face processing timed out. Retry this photo from People settings.'))
      }, 360000)
      cancelInference = () => {
        clearTimeout(deadline)
        reject(Error('Processing stopped'))
      }
      worker!.onmessage = (e) => {
        if (e.data.type === 'progress') {
          message.value = e.data.message
          return
        }
        clearTimeout(deadline)
        cancelInference = undefined
        if (e.data.error) reject(Error(e.data.error))
        else {
          device.value = e.data.device
          resolve(e.data.faces)
        }
      }
      worker!.onerror = (e) => {
        clearTimeout(deadline)
        cancelInference = undefined
        reject(Error(e.message))
      }
      worker!.postMessage({ id: 1, image, manifest }, [image])
    })
  }
  async function processNext() {
    const pref = preference.value
    if (
      !started ||
      working.value ||
      !visible.value ||
      !pref ||
      pref.reason ||
      pref.placement !== 'browser' ||
      !model.value ||
      !navigator.onLine
    )
      return
    const run = epoch,
      token = auth.token
    working.value = true
    let job: FaceJob | null = null
    let heartbeat: ReturnType<typeof setInterval> | undefined
    try {
      const data = await call<{ job: FaceJob | null }>({
        method: 'post',
        url: '/ai/people/jobs/claim',
        data: {},
      })
      job = data.job
      if (run !== epoch || auth.token !== token || !job) return
      heartbeat = setInterval(() => {
        if (run === epoch)
          call({ method: 'post', url: '/ai/people/jobs/renew', data: job }).catch(() => stopWork())
      }, 30000)
      message.value = 'Finding people in your photos…'
      const { data: image } = await http.post<ArrayBuffer>('/ai/people/jobs/image', job, {
        responseType: 'arraybuffer',
        timeout: 120000,
      })
      if (run !== epoch) return
      const faces = await infer(image, model.value)
      if (run !== epoch || auth.token !== token) return
      await call({ method: 'post', url: '/ai/people/jobs/result', data: { job, faces } })
      error.value = ''
      message.value = ''
    } catch (e) {
      if (run === epoch && auth.token === token) {
        error.value = e instanceof Error ? e.message : 'Face processing failed.'
        if (job)
          await call({
            method: 'post',
            url: '/ai/people/jobs/fail',
            data: { job, error: error.value },
          }).catch(() => {})
      }
    } finally {
      if (heartbeat) clearInterval(heartbeat)
      if (run === epoch) working.value = false
    }
  }
  async function tick(run: number) {
    if (!started || run !== loop) return
    if (Date.now() - lastRefresh > 10000) await refresh()
    if (!started || run !== loop) return
    await processNext()
    if (started && run === loop) timer = setTimeout(() => tick(run), 2500)
  }
  function start() {
    if (started) return
    started = true
    void tick(++loop)
  }
  function stop() {
    started = false
    loop++
    if (timer) clearTimeout(timer)
    stopWork()
  }
  async function change(data: Partial<Pick<PeoplePreference, 'enabled' | 'paused' | 'placement'>>) {
    await call({ method: 'patch', url: '/ai/people/settings', data })
    stopWork()
    if (refreshPromise) await refreshPromise
    await refresh()
    revision.value++
  }
  async function retry() {
    await call({ method: 'post', url: '/ai/people/retry', data: {} })
    await refresh()
  }
  async function clear() {
    await call({ method: 'delete', url: '/ai/people/data', data: { confirm: true } })
    stopWork()
    if (refreshPromise) await refreshPromise
    await refresh()
    revision.value++
  }
  watch(
    () => auth.token,
    () => {
      stopWork()
      lastRefresh = 0
      preference.value = null
      status.value = null
      model.value = null
      error.value = ''
      revision.value++
    },
  )
  return {
    preference,
    status,
    model,
    device,
    message,
    error,
    revision,
    working,
    visible,
    start,
    stop,
    refresh,
    change,
    retry,
    clear,
  }
})
