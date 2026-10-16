<script setup lang="ts">
import { smartReason } from '@/lib/smartOrganization'
import { computed, ref, watch } from 'vue'
import AdminLayout from '@/components/AdminLayout.vue'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
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
const selectedProvider = ref('siglip2'),
  selectedId = ref<number>(),
  confirmation = ref<{ command: string; profileId: number | undefined } | null>(null)
const inputClass = 'h-9 rounded-md border bg-background px-3 text-sm w-full'
const profile = computed(() =>
  profiles.value.profiles.find((p) => p.profileId === selectedId.value),
)
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
    ([key]) => !['quota_bytes', 'similarity_threshold'].includes(key),
  ),
)
const loadedModel = computed(
  () =>
    status.value?.settings.runtime === 'loaded' ||
    ['ready', 'loading', 'warming', 'draining'].includes(runtime.value?.observed.state ?? ''),
)
const labels: Record<string, string> = {
  background_interval_seconds: 'Minimum background interval (seconds)',
  background_max_per_minute: 'Maximum attempts per minute (0 = unlimited)',
  cpu_threads: 'CPU compute threads',
  cpu_pause_percent: 'CPU pause (%)',
  cpu_resume_percent: 'CPU resume (%)',
  gpu_pause_percent: 'GPU pause (%)',
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
watch(profiles, (value) => {
  if (!value.profiles.some((p) => p.profileId === selectedId.value))
    selectedId.value = value.profiles[0]?.profileId
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
    'enabled',
    'new_user_default',
    'smart_tag_enabled',
    'device',
    'precision',
    ...Object.keys(capabilities.value.fields),
  ]
  for (const key of keys)
    if (draft.value[key as keyof AiSettings] !== original.value[key as keyof AiSettings])
      Object.assign(changes, { [key]: draft.value[key as keyof AiSettings] })
  if (!Object.keys(changes).length) {
    error.value = 'No settings changed.'
    return
  }
  if (await admin.save(original.value.generation, changes)) resetDraft()
}
function command(name: string, pid?: number) {
  if (['purge', 'rebuild', 'maintenance.enter'].includes(name))
    confirmation.value = { command: name, profileId: pid }
  else void admin.operate(name, pid)
}
function confirmed() {
  const value = confirmation.value
  confirmation.value = null
  if (value) void admin.operate(value.command, value.profileId)
}
const numberLabel = (value: number | null | undefined) =>
  value == null || staleResources.value ? 'Unknown' : `${value.toFixed(1)}%`
const time = (value: string | null | undefined) => (value ? new Date(value).toLocaleString() : '—')
</script>
<template>
  <AdminLayout>
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold">AI Management</h1>
        <p class="text-sm text-muted-foreground">
          Search, model lifecycle and background indexing.
        </p>
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
      <section class="rounded-lg border bg-card p-4 space-y-3" aria-label="AI overview">
        <h2 class="text-lg font-semibold">Overview</h2>
        <p>
          AI {{ status.settings.enabled ? 'enabled' : 'disabled' }} · Maintenance
          {{ status.settings.maintenance ? 'on' : 'off' }} · Configuration generation
          {{ status.settings.generation }}
        </p>
        <p class="text-sm">
          Search profile: {{ status.settings.search_profile_id ?? 'None' }} · Building:
          {{ status.settings.building_profile_id ?? 'None' }} · Desired loaded profile:
          {{ status.settings.loaded_profile_id ?? 'None' }}
        </p>
        <div class="grid gap-3 sm:grid-cols-3">
          <div v-for="(service, name) in status.services" :key="name" class="rounded-md border p-3">
            <h3 class="font-medium capitalize">
              {{ name === 'maintenance' ? 'Index maintenance service' : name }}
            </h3>
            <p>
              {{
                service.online
                  ? name === 'maintenance' && service.observed.state === 'running'
                    ? 'Online'
                    : service.observed.state
                  : 'Offline / unknown'
              }}
            </p>
            <p v-if="name === 'maintenance'" class="text-xs text-muted-foreground">
              Service availability is separate from maintenance mode, shown above.
            </p>
            <p class="text-xs text-muted-foreground">Heartbeat: {{ time(service.heartbeatAt) }}</p>
            <p v-if="service.observed.reason" class="text-sm">
              {{
                Array.isArray(service.observed.reason)
                  ? service.observed.reason.join(', ')
                  : service.observed.reason
              }}
            </p>
          </div>
        </div>
        <p class="text-xs text-muted-foreground">
          Start and stop control work inside running services. Offline containers must be restored
          by the deployment operator. Ordinary library features do not depend on AI readiness.
        </p>
      </section>
      <section class="rounded-lg border bg-card p-4 space-y-4" aria-label="Model and index">
        <h2 class="text-lg font-semibold">Model and index</h2>
        <ol class="list-decimal pl-5 space-y-1 text-sm">
          <li>Create or select a profile and wait for its index.</li>
          <li>Choose the device in settings, then load and validate the model.</li>
          <li>
            Enable AI and grant access in
            <router-link class="underline" to="/thepersonincharge/usermanage"
              >User Management</router-link
            >.
          </li>
          <li>Start the scheduler and wait for supported photos and videos to be indexed.</li>
          <li>
            Activate explicitly. To switch or roll back, enter maintenance first, load the target,
            complete indexing and activate.
          </li>
        </ol>
        <div class="flex flex-wrap items-end gap-2">
          <label class="text-sm"
            >Registered model<select
              v-model="selectedProvider"
              aria-label="Registered model"
              :class="inputClass"
            >
              <option v-for="spec in profiles.registry" :key="spec.provider" :value="spec.provider">
                {{ spec.model_id }}
              </option>
            </select></label
          ><Button :disabled="locked" @click="admin.createProfile(selectedProvider)"
            >Create profile</Button
          >
        </div>
        <label class="block text-sm"
          >Selected profile<select
            v-model.number="selectedId"
            aria-label="Selected profile"
            :class="inputClass"
          >
            <option v-for="p in profiles.profiles" :key="p.profileId" :value="p.profileId">
              #{{ p.profileId }} · {{ p.spec.model_id }} · {{ p.indexStatus }}
            </option>
          </select></label
        >
        <div v-if="profile" class="space-y-2 text-sm">
          <p class="break-all">
            Revision: {{ profile.spec.revision }} · {{ profile.dimension }} dimensions · halfvec
            storage
          </p>
          <p class="break-all text-xs text-muted-foreground">
            Fingerprint: {{ profile.fingerprint }}
          </p>
          <p>
            Coverage: {{ profile.coverage.ready }} / {{ profile.coverage.supported }} supported ·
            {{ profile.coverage.pending }} pending · {{ profile.coverage.failed }} failed ·
            {{ profile.coverage.unsupported }} unsupported
          </p>
          <p v-if="profile.activationBlockers.length">
            Activation blocked: {{ profile.activationBlockers.map(aiMessage).join(', ') }}
          </p>
          <div class="flex flex-wrap gap-2">
            <Button
              :disabled="locked || !runtime?.online"
              @click="command('model.load', selectedId)"
              >Load model</Button
            >
            <Button
              variant="outline"
              :disabled="locked || !status.settings.loaded_profile_id || !runtime?.online"
              @click="command('model.restart', selectedId)"
              >Restart model</Button
            >
            <Button
              variant="outline"
              :disabled="locked || !loadedModel"
              @click="command('model.unload')"
              >Unload model</Button
            >
            <Button
              :disabled="locked || !!profile.activationBlockers.length"
              @click="command('activate', selectedId)"
              >Activate profile</Button
            >
          </div>
          <p>
            Actual model state: {{ runtime?.online ? runtime.observed.state : 'Unknown' }} ·
            {{ runtime?.observed.config?.device ?? 'Unknown' }} /
            {{ runtime?.observed.config?.precision ?? 'Unknown' }} · CPU threads
            {{ runtime?.observed.config?.cpu_threads ?? 'Unknown' }}
          </p>
          <p>
            Validation:
            {{
              capabilities?.verified.some(
                (v) =>
                  v.fingerprint === profile?.fingerprint &&
                  v.device === status?.settings.device &&
                  v.precision === status?.settings.precision,
              )
                ? 'Passed model load and text/image warmup in this runtime session'
                : 'Not yet verified in this runtime session'
            }}
          </p>
        </div>
        <details class="rounded-md border p-3">
          <summary class="cursor-pointer font-medium">Advanced lifecycle controls</summary>
          <div class="mt-3 flex flex-wrap gap-2">
            <Button
              variant="outline"
              :disabled="locked || status.settings.maintenance"
              @click="command('maintenance.enter')"
              >Enter maintenance</Button
            >
            <Button
              variant="outline"
              :disabled="
                locked ||
                !status.settings.maintenance ||
                (!!status.settings.building_profile_id &&
                  status.settings.building_profile_id !== status.settings.search_profile_id)
              "
              @click="command('maintenance.exit')"
              >Exit maintenance</Button
            >
            <Button
              variant="outline"
              :disabled="locked || !profile"
              @click="command('backfill', selectedId)"
              >Backfill</Button
            >
            <Button
              variant="outline"
              :disabled="locked || !profile?.coverage.failed"
              @click="command('retry_failed', selectedId)"
              >Retry failed</Button
            >
            <Button
              variant="outline"
              :disabled="locked || !profile || !status.settings.maintenance"
              @click="command('rebuild', selectedId)"
              >Rebuild</Button
            >
            <Button
              variant="destructive"
              :disabled="locked || !profile || !!profile.purgeBlockers.length"
              @click="command('purge', selectedId)"
              >Delete profile and vectors</Button
            >
          </div>
          <p v-if="profile?.purgeBlockers.length" class="mt-2 text-sm">
            Cleanup blocked: {{ profile.purgeBlockers.map(aiMessage).join(', ') }}
          </p>
          <p class="mt-2 text-xs text-muted-foreground">
            Only one model is resident. Maintenance interrupts AI search. Activation exits
            maintenance. Retained profiles count toward the storage budget.
          </p>
        </details>
      </section>
      <section class="rounded-lg border bg-card p-4 space-y-3" aria-label="Scheduler">
        <h2 class="text-lg font-semibold">Background scheduler</h2>
        <p>
          Desired: {{ status.settings.scheduler }} · Actual:
          {{
            status.services.scheduler?.online ? status.services.scheduler.observed.state : 'Unknown'
          }}
        </p>
        <div class="flex flex-wrap gap-2">
          <Button
            :disabled="
              locked ||
              !status.services.scheduler?.online ||
              status.settings.scheduler !== 'stopped'
            "
            @click="command('scheduler.start')"
            >Start scheduler</Button
          ><Button
            variant="outline"
            :disabled="locked || status.settings.scheduler !== 'running'"
            @click="command('scheduler.pause')"
            >Pause scheduler</Button
          ><Button
            variant="outline"
            :disabled="locked || status.settings.scheduler !== 'paused'"
            @click="command('scheduler.resume')"
            >Resume scheduler</Button
          ><Button
            variant="outline"
            :disabled="locked || status.settings.scheduler === 'stopped'"
            @click="command('scheduler.stop')"
            >Stop scheduler</Button
          >
        </div>
        <p class="text-sm text-muted-foreground">
          Pause/stop prevents new background work; the current media item may finish. The control
          loop stays online and the model stays loaded.
        </p>
        <p class="text-sm">
          Jobs:
          <span v-for="(count, state) in status.jobs" :key="state" class="mr-3"
            >{{ state }} {{ count }}</span
          >
        </p>
        <p class="text-sm">
          Concurrency {{ status.backgroundLimits.maxConcurrency }} · Batch
          {{ status.backgroundLimits.batchSize }} (read-only)
        </p>
        <p class="text-sm">
          {{ status.backgroundLimits.startsLastMinute }} attempts in the last minute · Last start
          {{ time(status.backgroundLimits.lastStartedAt) }} · Earliest next start
          {{ time(status.backgroundLimits.nextAllowedAt) }}
        </p>
        <p v-if="status.backgroundLimits.reason" role="status">
          Waiting: {{ aiMessage(status.backgroundLimits.reason) }}
        </p>
      </section>
      <section
        v-if="draft && capabilities"
        class="rounded-lg border bg-card p-4 space-y-4"
        aria-label="AI settings"
      >
        <h2 class="text-lg font-semibold">Settings and resource limits</h2>
        <div
          v-if="status.smartTag"
          class="rounded-md border p-3 text-sm"
          aria-label="Smart Tag progress"
        >
          <p>
            Smart Tag: {{ status.smartTag.processed }} / {{ status.smartTag.total }} media inspected
            · {{ status.smartTag.failed }} failed
          </p>
          <p v-if="status.smartTag.reason" role="status">
            {{ smartReason(status.smartTag.reason) }}
          </p>
          <p class="text-xs text-muted-foreground">
            Uses the existing scheduler. Users must also have AI access and opt in.
          </p>
        </div>
        <p v-if="original?.generation !== status.settings.generation" role="status" class="text-sm">
          The server configuration changed. Your draft is preserved; reload saved settings before
          editing again.
        </p>
        <form class="space-y-4" @submit.prevent="save">
          <div class="flex flex-wrap gap-5">
            <label class="flex gap-2 text-sm"
              ><input
                v-model="draft.smart_tag_enabled"
                type="checkbox"
                aria-label="Enable Smart Tag"
              />Enable Smart Tag</label
            >
            <label class="flex gap-2 text-sm"
              ><input v-model="draft.enabled" type="checkbox" aria-label="Enable AI search" />Enable
              AI search</label
            ><label class="flex gap-2 text-sm"
              ><input
                v-model="draft.new_user_default"
                type="checkbox"
                aria-label="Enable AI for new users"
              />Enable AI for new users</label
            >
          </div>
          <p class="text-xs text-muted-foreground">
            New-user defaults affect future accounts only. Administrators also require individual AI
            access.
          </p>
          <div class="rounded-md border p-3 space-y-2">
            <label class="block text-sm" for="similarity-threshold"
              >Similarity threshold (optional)</label
            >
            <div class="flex flex-wrap items-center gap-2">
              <input
                id="similarity-threshold"
                :value="draft.similarity_threshold ?? ''"
                type="number"
                step="any"
                :min="capabilities.fields.similarity_threshold?.minimum ?? -1"
                :max="capabilities.fields.similarity_threshold?.maximum ?? 1"
                placeholder="No threshold"
                :class="inputClass"
                aria-describedby="similarity-threshold-help"
                @input="setThreshold"
              />
              <Button type="button" variant="outline" @click="draft.similarity_threshold = null"
                >No threshold</Button
              >
            </div>
            <p id="similarity-threshold-help" class="text-xs text-muted-foreground">
              Leave blank for no threshold (default). Similar photos and videos and image queries
              only show cosine scores at or above this value (-1 to 1). This is not a probability.
              Save below to apply; no reindex or model restart is needed. Natural language search is
              unchanged.
            </p>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="text-sm"
              >Device / inference precision<select
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
                  {{ device.device === 'mps' ? 'Apple Silicon GPU (MPS)' : device.device }} /
                  {{ device.precision
                  }}{{ device.available !== true ? ' — unavailable or unknown' : '' }}
                </option>
                <option disabled>NPU — not supported</option>
              </select></label
            ><label class="text-sm"
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
          <p class="text-xs text-muted-foreground">
            Unload → save device/threads → load again. Available hardware is validated by a real
            model load. Storage precision (halfvec) is separate from inference precision.
          </p>
          <details open class="rounded-md border p-3">
            <summary class="cursor-pointer font-medium">Advanced scheduling limits</summary>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
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
                /><span class="text-xs text-muted-foreground">{{
                  rule.apply === 'model_reload'
                    ? 'Applies on model load'
                    : 'Applies on the next scheduling cycle'
                }}</span></label
              >
            </div>
          </details>
          <p class="text-xs text-muted-foreground">
            Intervals measure claim start times; rate limits include retries. These thresholds
            control new background work, not hard CPU/GPU utilization caps. Online searches have
            separate limits.
          </p>
          <div class="flex flex-wrap gap-2">
            <Button
              type="submit"
              :disabled="locked || original?.generation !== status.settings.generation"
              >Save AI settings</Button
            ><Button type="button" variant="outline" :disabled="saving" @click="resetDraft"
              >Reload saved settings (discard draft)</Button
            >
          </div>
        </form>
        <div class="border-t pt-3 text-sm space-y-1">
          <p>
            Estimated vectors: {{ bytesLabel(status.quota.estimated_bytes) }} · Reserved:
            {{ bytesLabel(status.quota.reserved_bytes) }} · Actual tables/indexes:
            {{ bytesLabel(status.quota.physical_bytes) }}
          </p>
          <p>
            CPU: {{ numberLabel(resources?.cpu_percent) }} · GPU:
            {{ numberLabel(resources?.gpu_percent) }}
          </p>
          <p>
            Available RAM:
            {{ staleResources ? 'Unknown' : bytesLabel(resources?.memory_available) }} · VRAM:
            {{ staleResources ? 'Unknown' : bytesLabel(resources?.vram_available) }} · Runtime disk:
            {{ staleResources ? 'Unknown' : bytesLabel(resources?.disk_free) }}
          </p>
          <p>
            Resource sample: {{ time(resources?.sampled_at) }}
            {{ staleResources ? '(unknown or stale)' : '' }}
          </p>
          <p
            v-if="runtime?.observed.config?.device === 'mps'"
            class="text-xs text-muted-foreground"
          >
            Apple Silicon uses unified memory. RAM limits apply; GPU utilization and dedicated VRAM
            metrics are unavailable and do not block background work.
          </p>
          <p v-if="resources?.missing?.length">
            Unavailable metrics: {{ resources.missing.join(', ') }}
          </p>
          <p class="text-xs text-muted-foreground">
            The budget limits scheduling, not physical disk usage. Deleting vectors may leave
            reusable database space. Runtime disk space is not database disk space.
          </p>
        </div>
      </section>
      <section class="rounded-lg border bg-card p-4 space-y-3" aria-label="Operations">
        <h2 class="text-lg font-semibold">Operations</h2>
        <p v-if="active.length" role="status">
          {{ active.length }} operation(s) pending. Further changes wait for completion.
        </p>
        <p v-if="!allJobs.length" class="text-sm">No operations yet.</p>
        <div v-for="job in allJobs" :key="job.operationId" class="rounded-md border p-3 text-sm">
          <p class="font-medium">#{{ job.operationId }} {{ job.command }} · {{ job.status }}</p>
          <p>
            Profile {{ job.profileId ?? '—' }} · Generation {{ job.generation }} · Stage
            {{ job.stage }} · Attempts {{ job.attempts }}
          </p>
          <p>{{ time(job.createdAt) }} → {{ time(job.finishedAt) }}</p>
          <p v-if="job.result.reason" role="status">{{ aiMessage(job.result.reason) }}</p>
        </div>
        <Button v-if="nextBefore" variant="outline" @click="admin.older">Older operations</Button>
      </section>
    </template>
    <AlertDialog :open="!!confirmation" @update:open="(open) => !open && (confirmation = null)"
      ><AlertDialogContent
        ><AlertDialogHeader
          ><AlertDialogTitle>Confirm {{ confirmation?.command }}</AlertDialogTitle
          ><AlertDialogDescription>{{
            confirmation?.command === 'purge'
              ? 'This deletes the selected profile, its index and stored vectors. Your photos and videos remain in the library.'
              : confirmation?.command === 'rebuild'
                ? 'Rebuild the selected profile. Existing results will be invalidated and computed again.'
                : 'AI search will be unavailable during maintenance. Ordinary library features remain available.'
          }}</AlertDialogDescription></AlertDialogHeader
        ><AlertDialogFooter
          ><AlertDialogCancel>Cancel</AlertDialogCancel
          ><Button @click="confirmed">Confirm operation</Button></AlertDialogFooter
        ></AlertDialogContent
      ></AlertDialog
    >
  </AdminLayout>
</template>
