<script setup lang="ts">
import { smartReason } from '@/lib/smartOrganization'
import { computed, ref, watch } from 'vue'
import AdminLayout from '@/components/AdminLayout.vue'
import { Button } from '@/components/ui/button'
import { useAiAdmin } from '@/composables/useAiAdmin'
import { aiMessage, bytesLabel, type AiChanges, type AiSettings } from '@/lib/ai'
const admin = useAiAdmin()
const {
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
} = admin
const draft = ref<AiSettings | null>(null),
  original = ref<AiSettings | null>(null),
  budgetGiB = ref(10)
const selectedProvider = ref('')
const ramEdited = ref(false)
const inputClass = 'h-9 rounded-md border bg-background px-3 text-sm w-full'
const runtime = computed(() => status.value?.services.runtime)
const resources = computed(() => runtime.value?.observed.resources)
const staleResources = computed(
  () =>
    !runtime.value?.online ||
    !resources.value ||
    Date.now() - Date.parse(resources.value.sampled_at) > 60000,
)
const allJobs = computed(() =>
  [...new Map([...jobs.value, ...history.value].map((j) => [j.operationId, j])).values()].sort(
    (a, b) => b.operationId - a.operationId,
  ),
)
const numericFields = computed(() =>
  Object.entries(capabilities.value?.fields ?? {}).filter(
    ([key]) =>
      ![
        'quota_bytes',
        'similarity_threshold',
        'cpu_pause_percent',
        'cpu_resume_percent',
        'gpu_pause_percent',
        'gpu_resume_percent',
        'ram_max_percent',
        'memory_pause_free_mib',
        'memory_resume_free_mib',
      ].includes(key),
  ),
)
const loadedModel = computed(
  () =>
    status.value?.settings.runtime === 'loaded' ||
    ['ready', 'downloading', 'preparing', 'loading', 'warming', 'draining'].includes(
      runtime.value?.observed.state ?? '',
    ),
)
const labels: Record<string, string> = {
  background_interval_seconds: 'Minimum background interval (seconds)',
  background_max_per_minute: 'Maximum attempts per minute (0 = unlimited)',
  cpu_threads: 'CPU compute threads',
  ram_max_percent: 'RAM max (%)',
  cpu_pause_percent: 'CPU utilization (%)',
  cpu_resume_percent: 'CPU resume (%)',
  gpu_pause_percent: 'GPU utilization (%)',
  gpu_resume_percent: 'GPU resume (%)',
  memory_pause_free_mib: 'RAM pause below (MiB)',
  memory_resume_free_mib: 'RAM resume above (MiB)',
  vram_pause_free_mib: 'VRAM pause below (MiB)',
  vram_resume_free_mib: 'VRAM resume above (MiB)',
  disk_pause_free_mib: 'Disk pause below (MiB)',
  disk_resume_free_mib: 'Disk resume above (MiB)',
}
function resetDraft() {
  if (status.value) {
    ramEdited.value = false
    draft.value = { ...status.value.settings }
    original.value = { ...status.value.settings }
    budgetGiB.value = draft.value.quota_bytes / 1024 ** 3
  }
}
watch(status, () => {
  if (
    !draft.value ||
    (JSON.stringify(draft.value) === JSON.stringify(original.value) &&
      budgetGiB.value === (original.value?.quota_bytes ?? 0) / 1024 ** 3)
  )
    resetDraft()
})
function fieldValue(key: string) {
  return Number(draft.value?.[key as keyof AiSettings] ?? 0)
}
function setNumber(key: string, event: Event) {
  if (draft.value)
    Object.assign(draft.value, {
      [key]:
        (event.target as HTMLInputElement).value === ''
          ? NaN
          : Number((event.target as HTMLInputElement).value),
    })
}
function setThreshold(event: Event) {
  if (!draft.value) return
  const input = event.target as HTMLInputElement
  draft.value.similarity_threshold = input.validity.badInput
    ? NaN
    : input.value === ''
      ? null
      : Number(input.value)
}
function chooseDevice(event: Event) {
  if (!draft.value) return
  const selectedDevice = (event.target as HTMLSelectElement).value
  draft.value.device = selectedDevice
  draft.value.precision =
    capabilities.value?.devices.find((device) => device.device === selectedDevice)?.precision ??
    'float32'
}
async function save() {
  if (!draft.value || !original.value || !capabilities.value) return
  error.value = ''
  draft.value.quota_bytes = budgetGiB.value * 1024 ** 3
  const changes: AiChanges = {}
  for (const [key, rule] of Object.entries(capabilities.value.fields)) {
    if (key === 'similarity_threshold') {
      const value = draft.value.similarity_threshold
      if (
        value !== null &&
        (!Number.isFinite(value) || value < rule.minimum || value > rule.maximum)
      ) {
        error.value = aiMessage('invalid_similarity_threshold')
        return
      }
      continue
    }
    const value = fieldValue(key)
    if (!Number.isSafeInteger(value) || value < rule.minimum || value > rule.maximum) {
      error.value = `${labels[key] ?? key}: enter an integer between ${rule.minimum} and ${rule.maximum} ${rule.unit}.`
      return
    }
    if (
      rule.pairedWith &&
      (rule.relation === 'less'
        ? value >= fieldValue(rule.pairedWith)
        : value <= fieldValue(rule.pairedWith))
    ) {
      error.value = `${labels[key]} must be ${rule.relation} than ${labels[rule.pairedWith]}.`
      return
    }
  }
  const keys = [
    'search_enabled',
    'new_user_default',
    'smart_tag_enabled',
    'device',
    'precision',
    ...Object.keys(capabilities.value.fields),
  ]
  for (const key of keys)
    if (draft.value[key as keyof AiSettings] !== original.value[key as keyof AiSettings])
      Object.assign(changes, { [key]: draft.value[key as keyof AiSettings] })
  if (ramEdited.value) changes.ram_max_percent = draft.value.ram_max_percent
  if (!Object.keys(changes).length) {
    error.value = 'No settings changed.'
    return
  }
  if (await admin.save(original.value.generation, changes)) resetDraft()
}
function setResource(key: string, event: Event) {
  setNumber(key, event)
  if (!draft.value) return
  if (key === 'ram_max_percent') ramEdited.value = true
  else Object.assign(draft.value, { [key.replace('pause', 'resume')]: fieldValue(key) - 10 })
}
function recommended() {
  if (!draft.value) return
  Object.assign(draft.value, {
    cpu_pause_percent: 85,
    cpu_resume_percent: 75,
    gpu_pause_percent: 90,
    gpu_resume_percent: 80,
    ram_max_percent: 85,
  })
  ramEdited.value = true
}
function enableAi() {
  void admin.operate('ai.enable', undefined, selectedProvider.value || undefined)
}
async function feature(key: 'search_enabled' | 'smart_tag_enabled', event: Event) {
  if (!status.value) return
  const input = event.target as HTMLInputElement
  const generation = status.value.settings.generation
  const saved = await admin.save(generation, { [key]: input.checked })
  if (!saved && status.value) input.checked = status.value.settings[key]
  if (
    saved &&
    status.value?.settings.generation === generation + 1 &&
    draft.value &&
    original.value
  ) {
    // An immediate feature toggle must not discard an unsaved resource draft.
    const update = { generation: generation + 1, [key]: status.value.settings[key] }
    Object.assign(draft.value, update)
    Object.assign(original.value, update)
  }
}
const stage = computed(
  () =>
    status.value?.startup?.stage ??
    (status.value?.settings.enabled ? runtime.value?.observed.state : 'paused'),
)
const stageLabels: Record<string, string> = {
  preparing: 'Preparing AI',
  downloading: 'Downloading model',
  loading: 'Loading model',
  warming: 'Validating text and image inference',
  ready: 'AI ready',
  preparing_index: 'Preparing search index',
  paused: 'AI paused',
  unloaded: 'AI paused',
  pausing: 'Pausing AI and releasing resources',
  failed: 'AI startup failed',
  draining: 'Releasing previous model',
}
const starting = computed(
  () => !!status.value?.settings.enabled && !['ready', 'failed'].includes(stage.value ?? ''),
)
const downloadPercent = computed(() => {
  const p = status.value?.startup
  return p?.stage === 'downloading' && p.total && p.completed != null
    ? Math.min(100, Math.max(0, (p.completed / p.total) * 100))
    : undefined
})
const stalled = computed(
  () =>
    starting.value &&
    status.value?.startup?.updatedAt &&
    Date.now() - Date.parse(status.value.startup.updatedAt) > 120000,
)
const indexCoverage = computed(() => status.value?.buildingCoverage ?? status.value?.coverage)
const ramUsage = computed(() =>
  resources.value?.memory_total
    ? 100 * (1 - resources.value.memory_available / resources.value.memory_total)
    : null,
)
const numberLabel = (value: number | null | undefined) =>
  value == null || staleResources.value ? 'Unknown' : `${value.toFixed(1)}%`
const time = (value: string | null | undefined) => (value ? new Date(value).toLocaleString() : '—')
</script>
<template>
  <AdminLayout>
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold">AI Management</h1>
        <p class="text-sm text-muted-foreground">Search, Smart Tag and background processing.</p>
      </div>
      <Button variant="outline" @click="admin.refresh">Refresh status</Button>
    </header>
    <p
      v-if="error"
      role="alert"
      class="rounded-md border border-destructive p-3 text-sm text-destructive"
    >
      {{ error }}
    </p>
    <p v-if="!loaded" role="status">
      Loading AI controls… If the service is unavailable, use Refresh status to retry.
    </p>
    <Button v-if="pending" :disabled="saving" @click="admin.sendPending"
      >Recover operation status</Button
    >
    <template v-if="status">
      <section class="rounded-lg border bg-card p-5 space-y-4" aria-label="AI controls">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">
              {{ stageLabels[stage ?? ''] ?? 'AI status unknown' }}
            </h2>
            <p class="text-sm text-muted-foreground">
              {{
                status.settings.enabled
                  ? 'AI is enabled.'
                  : 'Enable AI to prepare your model automatically.'
              }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button
              v-if="!status.settings.enabled || stage === 'failed'"
              :disabled="locked"
              @click="enableAi"
              >Enable AI</Button
            >
            <Button
              v-if="status.settings.enabled || status.settings.runtime === 'loaded'"
              variant="outline"
              :disabled="saving || !!pending"
              @click="admin.operate('ai.pause')"
              >Pause AI</Button
            >
          </div>
        </div>
        <div v-if="starting" class="space-y-2" role="status">
          <progress
            class="w-full h-3 accent-primary"
            aria-label="AI startup progress"
            :value="downloadPercent"
            max="100"
          ></progress>
          <p v-if="downloadPercent !== undefined" class="text-sm">
            {{ status.startup?.completed }} / {{ status.startup?.total }} files available ·
            {{ downloadPercent.toFixed(0) }}% of download files
          </p>
          <p class="text-xs text-muted-foreground">
            Prepare → Download → Load → Validate → Ready. First startup may take longer while model
            files download.
          </p>
          <p v-if="stalled" class="text-sm">
            Still waiting for the current step. A large download or model load may take several
            minutes. You can pause and retry.
          </p>
        </div>
        <p v-if="status.startup?.reason" role="status" class="text-sm">
          {{ aiMessage(status.startup.reason) }}
        </p>
        <p v-if="!runtime?.online" role="status" class="text-sm">
          Offline / unknown — startup will continue when the AI services are available.
        </p>
        <div class="flex flex-wrap gap-5 border-t pt-4">
          <label class="flex items-center gap-2 text-sm"
            ><input
              type="checkbox"
              :checked="status.settings.search_enabled"
              :disabled="locked"
              aria-label="AI Search"
              @change="feature('search_enabled', $event)"
            />AI Search</label
          >
          <label class="flex items-center gap-2 text-sm"
            ><input
              type="checkbox"
              :checked="status.settings.smart_tag_enabled"
              :disabled="locked"
              aria-label="Smart Tag"
              @change="feature('smart_tag_enabled', $event)"
            />Smart Tag</label
          >
        </div>
        <p class="text-xs text-muted-foreground">
          Pause AI releases model memory and preserves your index and preferences. Individual access
          is managed in
          <router-link class="underline" to="/thepersonincharge/usermanage"
            >User Management</router-link
          >; Smart Tag also requires user opt-in.
        </p>
      </section>
      <section class="rounded-lg border bg-card p-5 space-y-4" aria-label="Scheduler">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">Background scheduler</h2>
            <p class="text-sm text-muted-foreground">
              {{
                status.settings.scheduler === 'running'
                  ? status.settings.enabled
                    ? 'Enabled'
                    : 'Enabled · waiting for AI'
                  : 'Paused'
              }}
            </p>
          </div>
          <Button
            :disabled="locked"
            :variant="status.settings.scheduler === 'running' ? 'outline' : 'default'"
            @click="
              admin.operate(
                status.settings.scheduler === 'running' ? 'scheduler.pause' : 'scheduler.start',
              )
            "
            >{{
              status.settings.scheduler === 'running' ? 'Pause Scheduler' : 'Enable Scheduler'
            }}</Button
          >
        </div>
        <p class="text-xs text-muted-foreground">
          Pausing stops new background tasks. The current item may finish; search remains available
          for indexed media.
        </p>
        <p v-if="!status.services.scheduler?.online" role="status" class="text-sm">
          Scheduler service offline. Restore the service to continue processing.
        </p>
        <p v-if="status.services.scheduler?.observed.reason" role="status" class="text-sm">
          Waiting:
          {{
            Array.isArray(status.services.scheduler.observed.reason)
              ? status.services.scheduler.observed.reason.map(aiMessage).join(', ')
              : aiMessage(status.services.scheduler.observed.reason)
          }}
        </p>
        <p v-if="status.backgroundLimits.reason" role="status" class="text-sm">
          Waiting: {{ aiMessage(status.backgroundLimits.reason) }}
        </p>
        <div v-if="indexCoverage" class="space-y-2">
          <p class="text-sm font-medium">
            Photo and video index · {{ indexCoverage.ready }} / {{ indexCoverage.supported }} ready
          </p>
          <progress
            aria-label="Media indexing progress"
            class="w-full h-2 accent-primary"
            :value="indexCoverage.ready"
            :max="Math.max(indexCoverage.supported, 1)"
          ></progress>
          <p class="text-xs text-muted-foreground">
            {{ indexCoverage.pending }} pending · {{ indexCoverage.failed }} failed ·
            {{ indexCoverage.unsupported }} unsupported. Search includes indexed content as it
            becomes ready.
          </p>
        </div>
        <div v-if="status.smartTag" class="text-sm" aria-label="Smart Tag progress">
          <p>
            Smart Tag: {{ status.smartTag.processed }} / {{ status.smartTag.total }} media inspected
            · {{ status.smartTag.failed }} failed
          </p>
          <p v-if="status.smartTag.reason">{{ smartReason(status.smartTag.reason) }}</p>
        </div>
      </section>
      <section
        v-if="draft && capabilities"
        class="rounded-lg border bg-card p-5 space-y-4"
        aria-label="AI settings"
      >
        <h2 class="text-lg font-semibold">Resources</h2>
        <p class="text-sm">
          CPU {{ numberLabel(resources?.cpu_percent) }} · GPU
          {{ numberLabel(resources?.gpu_percent) }} · RAM {{ numberLabel(ramUsage) }}
        </p>
        <p v-if="original?.generation !== status.settings.generation" role="status" class="text-sm">
          The server configuration changed. Your draft is preserved; reload saved settings before
          editing again.
        </p>
        <form class="space-y-4" @submit.prevent="save">
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <label
              v-for="key in ['cpu_pause_percent', 'gpu_pause_percent', 'ram_max_percent']"
              :key="key"
              class="text-sm"
              >{{ labels[key] }}
              <input
                :value="fieldValue(key)"
                :aria-label="labels[key]"
                type="number"
                step="1"
                :min="key === 'ram_max_percent' ? 6 : 11"
                max="100"
                :disabled="key === 'gpu_pause_percent' && draft.device !== 'cuda'"
                :class="inputClass"
                @input="setResource(key, $event)"
              />
            </label>
            <label class="text-sm"
              >Device<select
                :value="draft.device"
                aria-label="Inference device"
                :disabled="loadedModel"
                :class="inputClass"
                @change="chooseDevice"
              >
                <option
                  v-for="device in capabilities.devices"
                  :key="device.device"
                  :value="device.device"
                  :disabled="device.available !== true"
                >
                  {{
                    device.device === 'mps'
                      ? 'Apple Silicon GPU (MPS)'
                      : device.device.toUpperCase()
                  }}{{ device.available !== true ? ' — unavailable or unknown' : '' }}
                </option>
              </select></label
            >
          </div>
          <p class="text-xs text-muted-foreground">
            Background tasks wait above these limits. Changing a CPU/GPU limit sets recovery 10
            percentage points below; RAM recovers 5 points below. Existing custom CPU/GPU recovery
            values are preserved until changed. Limits are admission thresholds, not hard resource
            caps. Pause AI before changing Device.
          </p>
          <p v-if="draft.ram_policy === 'free' && !ramEdited" class="text-sm">
            Legacy RAM protection is active: pause below {{ draft.memory_pause_free_mib }} MiB free;
            resume above {{ draft.memory_resume_free_mib }} MiB. Save a RAM max value to use
            percentage limits.
          </p>
          <p v-if="draft.device === 'mps'" class="text-sm">
            Apple Silicon uses unified memory. GPU utilization is unavailable; CPU and RAM
            thresholds apply.
          </p>
          <Button type="button" variant="outline" @click="recommended"
            >Use recommended thresholds</Button
          >
          <details class="rounded-md border p-3">
            <summary class="cursor-pointer font-medium">Advanced settings</summary>
            <div class="mt-4 space-y-4">
              <div class="space-y-2">
                <label class="block text-sm"
                  >Model<select
                    v-model="selectedProvider"
                    aria-label="Registered model"
                    :class="inputClass"
                  >
                    <option value="">Current model / default SigLIP2</option>
                    <option
                      v-for="spec in profiles.registry"
                      :key="spec.provider"
                      :value="spec.provider"
                    >
                      {{ spec.model_id }}
                    </option>
                  </select></label
                >
                <Button
                  v-if="status.settings.enabled"
                  type="button"
                  variant="outline"
                  :disabled="locked || !selectedProvider"
                  @click="enableAi"
                  >Apply model</Button
                >
                <p class="text-xs text-muted-foreground">
                  {{
                    status.settings.enabled
                      ? 'Applying a model temporarily pauses search while the new model is prepared. Old indexes are retained.'
                      : 'Your selection will be used when you Enable AI.'
                  }}
                </p>
                <p class="text-sm">
                  Current model:
                  {{
                    profiles.profiles.find(
                      (p) =>
                        p.profileId ===
                        (status?.settings.loaded_profile_id ?? status?.settings.search_profile_id),
                    )?.spec.model_id ?? 'Not selected'
                  }}
                </p>
              </div>
              <div class="grid gap-3 sm:grid-cols-2">
                <label v-for="[key, rule] in numericFields" :key="key" class="text-sm"
                  >{{ labels[key] ?? key
                  }}<input
                    :value="fieldValue(key)"
                    :aria-label="labels[key] ?? key"
                    type="number"
                    step="1"
                    :min="rule.minimum"
                    :max="rule.maximum"
                    :disabled="key === 'cpu_threads' && loadedModel"
                    :class="inputClass"
                    @input="setNumber(key, $event)"
                /></label>
                <label class="text-sm"
                  >Embedding budget (GiB)<input
                    v-model.number="budgetGiB"
                    aria-label="Embedding budget (GiB)"
                    type="number"
                    step="any"
                    min="0.0009765625"
                    max="102400"
                    :class="inputClass"
                /></label>
              </div>
              <div class="space-y-2">
                <label class="block text-sm" for="similarity-threshold"
                  >Similarity threshold (optional)</label
                >
                <input
                  id="similarity-threshold"
                  :value="draft.similarity_threshold ?? ''"
                  type="number"
                  step="any"
                  min="-1"
                  max="1"
                  placeholder="No threshold"
                  :class="inputClass"
                  @input="setThreshold"
                />
                <Button type="button" variant="outline" @click="draft.similarity_threshold = null"
                  >No threshold</Button
                >
                <p class="text-xs text-muted-foreground">
                  Blank disables filtering. Cosine similarity (-1 to 1) applies to similar media and
                  image queries; natural language search is unchanged.
                </p>
              </div>
              <label class="flex gap-2 text-sm"
                ><input
                  v-model="draft.new_user_default"
                  type="checkbox"
                  aria-label="Enable AI for new users"
                />Enable AI for new users</label
              >
              <p class="text-xs text-muted-foreground">
                New-user defaults affect future accounts only. Pause AI before changing compute
                threads. Disk and VRAM guards remain active.
              </p>
            </div>
          </details>
          <div class="flex flex-wrap gap-2">
            <Button
              type="submit"
              :disabled="locked || original?.generation !== status.settings.generation"
              >Save AI settings</Button
            >
            <Button type="button" variant="outline" :disabled="saving" @click="resetDraft"
              >Reload saved settings (discard draft)</Button
            >
          </div>
        </form>
      </section>
      <details class="rounded-lg border bg-card p-4">
        <summary class="cursor-pointer font-medium">Diagnostics and operation history</summary>
        <div class="mt-4 space-y-3 text-sm">
          <p>
            Configuration generation {{ status.settings.generation }} · Search profile
            {{ status.settings.search_profile_id ?? 'None' }}
          </p>
          <div v-for="(service, name) in status.services" :key="name">
            <h3 class="font-medium">
              {{ name === 'maintenance' ? 'Index maintenance service' : name }}
            </h3>
            <p>
              {{ service.online ? service.observed.state : 'Offline / unknown' }} ·
              {{ time(service.heartbeatAt) }}
            </p>
          </div>
          <p>
            Concurrency {{ status.backgroundLimits.maxConcurrency }} · Batch
            {{ status.backgroundLimits.batchSize }} (read-only)
          </p>
          <p>
            Estimated vectors: {{ bytesLabel(status.quota.estimated_bytes) }} · Actual
            tables/indexes: {{ bytesLabel(status.quota.physical_bytes) }}
          </p>
          <p>
            Available RAM:
            {{ staleResources ? 'Unknown' : bytesLabel(resources?.memory_available) }} · VRAM:
            {{ staleResources ? 'Unknown' : bytesLabel(resources?.vram_available) }}
          </p>
          <p>
            Resource sample: {{ time(resources?.sampled_at) }}
            {{ staleResources ? '(unknown or stale)' : '' }}
          </p>
          <p v-if="active.length" role="status">{{ active.length }} operation(s) pending.</p>
          <p v-if="!allJobs.length">No operations yet.</p>
          <div v-for="job in allJobs" :key="job.operationId" class="rounded-md border p-3">
            <p>#{{ job.operationId }} {{ job.command }} · {{ job.status }} · {{ job.stage }}</p>
            <p>{{ time(job.createdAt) }}</p>
            <p v-if="job.result.reason">{{ aiMessage(job.result.reason) }}</p>
          </div>
          <Button v-if="nextBefore" variant="outline" @click="admin.older">Older operations</Button>
        </div>
      </details>
    </template>
  </AdminLayout>
</template>

<style scoped>
progress {
  appearance: none;
  border: none;
  border-radius: 999px;
  overflow: hidden;
  background: var(--muted);
}
progress::-webkit-progress-bar {
  border-radius: 999px;
  background: var(--muted);
}
progress::-webkit-progress-value {
  border-radius: 999px;
  background: var(--primary);
}
progress::-moz-progress-bar {
  border-radius: 999px;
  background: var(--primary);
}
progress:indeterminate {
  background: linear-gradient(90deg, var(--muted) 25%, var(--primary) 50%, var(--muted) 75%);
  background-size: 200% 100%;
  animation: pending-progress 2s linear infinite;
}
progress:indeterminate::-webkit-progress-bar {
  background: transparent;
}
@keyframes pending-progress {
  from {
    background-position: 200% 0;
  }
  to {
    background-position: -200% 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  progress:indeterminate {
    animation: none;
  }
}
</style>
