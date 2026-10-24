import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { newOperationKey, operationStorageKey } from '@/lib/operationIdentity'
import {
  activeJob,
  aiCall,
  AiError,
  aiMessage,
  type AdminCapabilities,
  type AiModelKind,
  type AiChanges,
  type AiJob,
  type AiSettings,
  type AiStatus,
  type JobList,
  type ProfileList,
} from '@/lib/ai'
interface Pending {
  key: string
  body: { command: string; profileId?: number; provider?: string; generation: number }
}
export function useAiAdmin() {
  const auth = useAuthStore()
  const status = ref<AiStatus | null>(null),
    capabilities = ref<AdminCapabilities | null>(null),
    profiles = ref<ProfileList>({ registry: [], profiles: [] })
  const jobs = ref<AiJob[]>([]),
    history = ref<AiJob[]>([]),
    nextBefore = ref<number | null>(null)
  const error = ref(''),
    saving = ref(false),
    pending = ref<Pending | null>(null),
    loaded = ref(false)
  const active = computed(() => jobs.value.filter(activeJob))
  const locked = computed(
    () =>
      saving.value ||
      pending.value !== null ||
      active.value.some((job) => !job.command?.startsWith('ai.')),
  )
  let disposed = false,
    refreshing = false,
    failures = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let storageKey = ''
  function remember() {
    if (storageKey) {
      if (pending.value) sessionStorage.setItem(storageKey, JSON.stringify(pending.value))
      else sessionStorage.removeItem(storageKey)
    }
  }
  async function refresh() {
    if (refreshing || disposed || !auth.token) return
    refreshing = true
    const token = auth.token
    try {
      const [s, c, p, recent, queued, running] = await Promise.all([
        aiCall<AiStatus>({ url: '/admin/ai/status', timeout: 10000 }),
        aiCall<AdminCapabilities>({ url: '/admin/ai/capabilities', timeout: 10000 }),
        aiCall<ProfileList>({ url: '/admin/ai/profiles', timeout: 10000 }),
        aiCall<JobList>({
          url: '/admin/ai/jobs',
          params: { kind: 'operation', order: 'desc', limit: 30 },
        }),
        aiCall<JobList>({
          url: '/admin/ai/jobs',
          params: { kind: 'operation', status: 'queued', limit: 100 },
        }),
        aiCall<JobList>({
          url: '/admin/ai/jobs',
          params: { kind: 'operation', status: 'running', limit: 100 },
        }),
      ])
      if (disposed || token !== auth.token) return
      if (!status.value || s.settings.generation >= status.value.settings.generation)
        status.value = s
      capabilities.value = c
      profiles.value = p
      jobs.value = [
        ...new Map(
          [...recent.items, ...queued.items, ...running.items].map((j) => [j.operationId, j]),
        ).values(),
      ].sort((a, b) => b.operationId - a.operationId)
      nextBefore.value = history.value.length ? nextBefore.value : recent.nextBefore
      loaded.value = true
      failures = 0
    } catch (e) {
      if (token === auth.token && !disposed) {
        error.value = aiMessage(e)
        failures++
      }
    } finally {
      refreshing = false
    }
  }
  async function save(generation: number, settings: AiChanges) {
    if (locked.value) return false
    saving.value = true
    error.value = ''
    try {
      const saved = await aiCall<AiSettings>({
        method: 'patch',
        url: '/admin/ai/settings',
        data: { generation, settings },
      })
      if (status.value) status.value = { ...status.value, settings: saved }
      await refresh()
      return true
    } catch (e) {
      error.value = aiMessage(e)
      await refresh()
      return false
    } finally {
      saving.value = false
    }
  }
  async function sendPending() {
    const request = pending.value
    if (!request || saving.value) return
    saving.value = true
    error.value = ''
    try {
      const job = await aiCall<AiJob>({
        method: 'post',
        url: '/admin/ai/operations',
        headers: { 'Idempotency-Key': request.key },
        data: request.body,
        timeout: 15000,
      })
      jobs.value = [job, ...jobs.value.filter((x) => x.operationId !== job.operationId)]
      pending.value = null
      remember()
      await refresh()
    } catch (e) {
      if (e instanceof AiError && e.status >= 400 && e.status < 500) {
        pending.value = null
        remember()
      }
      error.value = pending.value
        ? 'Operation outcome unknown. Retry the same request to recover its status.'
        : aiMessage(e)
      await refresh()
    } finally {
      saving.value = false
    }
  }
  /** Loads a model in the AI worker; `force` loads it although the server may be short of memory. */
  async function loadModel(kind: AiModelKind, force = false) {
    const result = await aiCall<unknown>({
      method: 'post',
      url: `/admin/ai/models/${kind}/load${force ? '?force=1' : ''}`,
      timeout: 120000,
    })
    await refresh()
    return result
  }
  async function unloadModel(kind: AiModelKind) {
    const result = await aiCall<unknown>({
      method: 'post',
      url: `/admin/ai/models/${kind}/unload`,
      timeout: 30000,
    })
    await refresh()
    return result
  }
  async function operate(command: string, profileId?: number, provider?: string) {
    if (
      (command === 'ai.pause' ? saving.value || pending.value !== null : locked.value) ||
      !status.value
    )
      return
    pending.value = {
      key: newOperationKey(),
      body: {
        command,
        generation: status.value.settings.generation,
        ...(profileId ? { profileId } : {}),
        ...(provider ? { provider } : {}),
      },
    }
    remember()
    await sendPending()
  }
  async function older() {
    if (!nextBefore.value) return
    try {
      const page = await aiCall<JobList>({
        url: '/admin/ai/jobs',
        params: { kind: 'operation', order: 'desc', before: nextBefore.value, limit: 30 },
      })
      history.value.push(...page.items)
      nextBefore.value = page.nextBefore
    } catch (e) {
      error.value = aiMessage(e)
    }
  }
  async function poll() {
    await refresh()
    if (!disposed)
      timer = setTimeout(
        poll,
        failures
          ? Math.min(30000, 2000 * 2 ** failures)
          : document.hidden
            ? 30000
            : active.value.length ||
                (status.value?.settings.enabled && status.value?.startup?.stage !== 'ready')
              ? 2000
              : 5000,
      )
  }
  onMounted(async () => {
    const token = auth.token
    if (token) {
      storageKey = operationStorageKey(token)
      try {
        pending.value = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
      } catch {
        sessionStorage.removeItem(storageKey)
      }
    }
    await poll()
    if (pending.value) await sendPending()
  })
  watch(
    () => auth.token,
    () => {
      if (storageKey) sessionStorage.removeItem(storageKey)
      storageKey = ''
      pending.value = null
      status.value = null
      profiles.value = { profiles: [], registry: [] }
      jobs.value = []
      history.value = []
      capabilities.value = null
      clearTimeout(timer)
    },
  )
  onBeforeUnmount(() => {
    disposed = true
    clearTimeout(timer)
  })
  return {
    status,
    capabilities,
    profiles,
    jobs,
    history,
    nextBefore,
    error,
    loaded,
    saving,
    pending,
    active,
    locked,
    refresh,
    save,
    operate,
    loadModel,
    unloadModel,
    older,
    sendPending,
  }
}
