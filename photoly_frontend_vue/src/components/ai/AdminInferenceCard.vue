<script setup lang="ts">
// Admin · AI · "Where AI runs": per model Server / Browser / Let users choose, server memory,
// AI worker status with Unload, and the not-enough-memory confirm dialog.
import { computed, ref } from 'vue'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  aiMessage,
  type AiModelKind,
  type AiSettings,
  type AiStatus,
  type PlacementSetting,
} from '@/lib/ai'
import {
  fitState,
  gbLabel,
  isWorkerDied,
  kindName,
  lowMemory,
  memoryMeter,
  NEED_GB,
  placementLabels,
  type LowMemory,
} from '@/lib/aiInference'

const props = defineProps<{
  draft: AiSettings
  status: AiStatus
  stale: boolean
  busy: boolean
  load: (kind: AiModelKind, force: boolean) => Promise<unknown>
  unload: (kind: AiModelKind) => Promise<unknown>
}>()
const emit = defineEmits<{ set: [patch: Partial<AiSettings>]; error: [message: string] }>()

const options: PlacementSetting[] = ['server', 'browser', 'user']
const rows: {
  kind: AiModelKind
  key: 'placement_image' | 'placement_text'
  title: string
  desc: string
  cost: string
}[] = [
  {
    kind: 'image',
    key: 'placement_image',
    title: 'Photo indexing',
    desc: 'Turns each new photo into a search vector.',
    cost: 'Browser: 372 MB download once; Server: about 0.9 GB memory',
  },
  {
    kind: 'text',
    key: 'placement_text',
    title: 'Search queries',
    desc: 'Turns what people type into a search vector.',
    cost: 'Browser: 318 MB download once, Server: 0.9–2.2 GB memory',
  },
]
const value = (key: 'placement_image' | 'placement_text') => props.draft[key] ?? 'server'

const memory = computed(() => memoryMeter(props.status, props.stale))
const worker = computed(() => props.status.worker)
const pct = (gb: number) => `${memory.value.totalGB ? Math.max(0, (gb / memory.value.totalGB) * 100) : 0}%`
const modelState = (kind: AiModelKind) => worker.value?.models?.[kind]
const fits = computed(() => [
  {
    name: 'Image model',
    need: NEED_GB.image,
    state: fitState(memory.value.freeGB, NEED_GB.image, !!modelState('image')?.loaded),
  },
  {
    name: 'Search model, full',
    need: NEED_GB.textFull,
    state: fitState(
      memory.value.freeGB,
      NEED_GB.textFull,
      !!modelState('text')?.loaded && modelState('text')?.variant !== 'int8',
    ),
  },
  {
    name: 'Search model, small',
    need: NEED_GB.textSmall,
    state: fitState(
      memory.value.freeGB,
      NEED_GB.textSmall,
      !!modelState('text')?.loaded && modelState('text')?.variant === 'int8',
    ),
  },
])
const fitStyle = {
  loaded: ['Loaded', 'bg-accent text-accent-foreground'],
  fits: ['Fits', 'bg-green-50 text-green-900 dark:bg-green-950/40 dark:text-green-200'],
  no: ["Doesn't fit", 'bg-orange-50 text-orange-900 dark:bg-orange-950/40 dark:text-orange-200'],
} as const

// Choosing Server loads the model at once, so a memory problem shows now, not mid-upload.
const pending = ref<{ kind: AiModelKind; key: 'placement_image' | 'placement_text'; info: LowMemory } | null>(null)
const working = ref(false)
async function choose(row: (typeof rows)[number], to: PlacementSetting) {
  if (to === value(row.key) || working.value) return
  if (to !== 'server') return emit('set', { [row.key]: to })
  working.value = true
  try {
    await props.load(row.kind, false)
    emit('set', { [row.key]: to })
  } catch (e) {
    const info = lowMemory(e)
    if (info) pending.value = { kind: row.kind, key: row.key, info }
    else emit('error', isWorkerDied(e) ? `${aiMessage(e)}` : aiMessage(e))
  } finally {
    working.value = false
  }
}
// Deferred so "Load Anyway" still sees the pending model when the dialog closes first.
const dismiss = () => queueMicrotask(() => (pending.value = null))
async function loadAnyway() {
  const p = pending.value
  if (!p) return
  pending.value = null
  working.value = true
  try {
    await props.load(p.kind, true)
    emit('set', { [p.key]: 'server' })
  } catch (e) {
    emit('error', aiMessage(e))
  } finally {
    working.value = false
  }
}
async function unloadModel(kind: AiModelKind) {
  working.value = true
  try {
    await props.unload(kind)
  } catch (e) {
    emit('error', aiMessage(e))
  } finally {
    working.value = false
  }
}
const mbToGb = (mb: number) => gbLabel(mb / 1000)
const loadedAgo = (at?: string | null) => {
  if (!at) return ''
  const hours = Math.round((Date.now() - Date.parse(at)) / 3_600_000)
  return hours < 1 ? 'less than an hour ago' : `${hours} h ago`
}
const sources = computed(() => props.status.indexSources)
</script>

<template>
  <section class="bg-card space-y-5 rounded-xl border p-5 shadow-xs" aria-label="Inference">
    <div>
      <h2 class="text-base font-semibold">Where AI runs</h2>
    </div>

    <div v-for="row in rows" :key="row.kind" class="flex flex-wrap items-center gap-x-6 gap-y-2">
      <div class="min-w-0 flex-1 basis-60">
        <div :id="`inf-${row.kind}`" class="text-sm font-medium">{{ row.title }}</div>
        <div class="text-muted-foreground text-sm">{{ row.desc }}</div>
        <div class="text-muted-foreground text-xs">{{ row.cost }}</div>
      </div>
      <div
        role="radiogroup"
        :aria-labelledby="`inf-${row.kind}`"
        class="bg-muted flex rounded-lg p-0.5"
      >
        <button
          v-for="o in options"
          :key="o"
          type="button"
          role="radio"
          :aria-checked="value(row.key) === o"
          :disabled="busy || working"
          :class="[
            'h-8 rounded-md px-3 text-sm font-medium',
            value(row.key) === o ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground',
          ]"
          @click="choose(row, o)"
        >
          {{ placementLabels[o] }}
        </button>
      </div>
    </div>

    <div class="flex flex-col gap-1.5 border-t pt-4">
      <label for="inf-browser-text" class="text-sm font-medium">Search model in the browser</label>
      <select
        id="inf-browser-text"
        :value="draft.browser_text_model ?? 'small'"
        class="border-input bg-card h-9 max-w-xs rounded-md border px-2.5 text-sm shadow-xs"
        @change="
          emit('set', {
            browser_text_model: ($event.target as HTMLSelectElement).value as 'small' | 'full',
          })
        "
      >
        <option value="small">Small, 318 MB download (recommended)</option>
        <option value="full">Full, 1.13 GB download</option>
      </select>
      <span class="text-muted-foreground text-xs"
        >Same results on our tests; small uses a quarter of the memory.</span
      >
    </div>

    <p class="text-muted-foreground border-t pt-4 text-sm">
      <span class="text-foreground font-medium">Server finishes what browsers miss</span> ZIP imports,
      API uploads and tabs closed early are indexed here.
    </p>

    <div v-if="sources" class="space-y-1 text-sm">
      <div class="flex gap-4">
        <span>Server: {{ sources.server.toLocaleString() }}</span>
        <span>Browsers: {{ sources.browser.toLocaleString() }}</span>
        <span>Waiting: {{ sources.waiting.toLocaleString() }}</span>
      </div>
      <p v-if="sources.spotCheck" class="text-muted-foreground text-xs">
        The server re-checks 1 in 100 browser results when idle. Last check:
        {{ sources.spotCheck.matched }} of {{ sources.spotCheck.checked }} matched.
      </p>
    </div>

    <div class="space-y-3 border-t pt-4" aria-label="Server memory">
      <div class="flex items-baseline justify-between text-sm">
        <h3 class="font-medium">Server memory</h3>
        <span class="text-muted-foreground">{{
          memory.known ? `${gbLabel(memory.freeGB)} free of ${Math.round(memory.totalGB)} GB` : 'Unknown'
        }}</span>
      </div>
      <div
        v-if="memory.known"
        class="bg-muted flex h-2 overflow-hidden rounded-full"
        role="img"
        :aria-label="`Photoly ${gbLabel(memory.appGB)}, AI worker ${gbLabel(memory.workerGB)}`"
      >
        <span class="bg-primary h-full" :style="{ width: pct(memory.appGB) }" />
        <span class="bg-primary/60 h-full" :style="{ width: pct(memory.workerGB) }" />
        <span class="bg-foreground/25 h-full" :style="{ width: pct(memory.otherGB) }" />
      </div>
      <ul class="space-y-1.5 text-sm">
        <li v-for="f in fits" :key="f.name" class="flex items-center gap-3">
          <span class="flex-1">{{ f.name }}</span>
          <span class="text-muted-foreground text-xs">Needs about {{ gbLabel(f.need) }}</span>
          <span
            v-if="memory.known || f.state === 'loaded'"
            :class="['rounded-full px-2 py-0.5 text-xs font-medium', fitStyle[f.state][1]]"
            >{{ fitStyle[f.state][0] }}</span
          >
        </li>
      </ul>
      <p class="text-muted-foreground text-xs">
        When the full search model doesn't fit, the server uses the small one. When nothing fits,
        Photoly asks before loading.
      </p>
    </div>

    <div class="space-y-3 border-t pt-4" aria-label="AI worker">
      <div class="flex items-baseline justify-between text-sm">
        <h3 class="font-medium">AI worker</h3>
        <span class="text-muted-foreground">{{
          worker?.running
            ? `Running${worker.memoryMB ? `, ${mbToGb(worker.memoryMB)}` : ''}`
            : 'Not running'
        }}</span>
      </div>
      <ul class="space-y-2 text-sm">
        <li v-for="kind in ['image', 'text'] as const" :key="kind" class="flex items-center gap-3">
          <span class="flex min-w-0 flex-1 flex-col">
            <span class="font-medium">{{ kind === 'image' ? 'Image model' : 'Search model' }}</span>
            <span class="text-muted-foreground text-xs">{{
              modelState(kind)?.loaded
                ? `${modelState(kind)?.variant === 'int8' ? 'Small (int8)' : 'Full precision'}, loaded ${loadedAgo(modelState(kind)?.loadedAt)}`.trim()
                : kind === 'text' && value('placement_text') !== 'server'
                  ? 'Not loaded, search runs in browsers'
                  : 'Not loaded, loads on demand'
            }}</span>
          </span>
          <Button
            v-if="modelState(kind)?.loaded"
            type="button"
            variant="outline"
            size="sm"
            :disabled="busy || working"
            :aria-label="`Unload ${kind === 'image' ? 'image' : 'search'} model`"
            @click="unloadModel(kind)"
            >Unload</Button
          >
        </li>
      </ul>
      <p v-if="worker?.restartedAt" class="text-muted-foreground text-xs">
        Restarted {{ new Date(worker.restartedAt).toLocaleString() }}
        {{ worker.restartReason ? `after ${worker.restartReason}` : '' }}.
      </p>
    </div>

    <AlertDialog :open="!!pending" @update:open="(o: boolean) => !o && dismiss()">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Not enough memory for {{ pending ? kindName(pending.kind) : '' }}</AlertDialogTitle>
          <AlertDialogDescription v-if="pending">
            This model needs about {{ mbToGb(pending.info.needMB) }}, and the server has
            {{ mbToGb(pending.info.availableMB) }} free. If you load it anyway, the system may stop
            the AI worker. Photoly keeps running, work moves back to browsers, and the worker
            restarts later.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Keep in Browsers</AlertDialogCancel>
          <AlertDialogAction @click="loadAnyway">Load Anyway</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </section>
</template>
