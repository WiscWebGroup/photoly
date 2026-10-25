<script setup lang="ts">
// "Account · AI on this device": where AI runs for this user, what is saved here, this device.
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Check, Info, Lock } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { aiMessage, type AiModelKind, type BrowserTextModel, type Placement } from '@/lib/ai'
import { threadCount } from '@/lib/browserAi'
import { modelSizes, sizeLabel } from '@/lib/browserAiCopy'
import { useBrowserAiStore } from '@/stores/browserAi'

const store = useBrowserAiStore()
const state = store.state
const caps = computed(() => store.capabilities)
const placement = computed(() => caps.value?.placement)
const sizes = computed(() => modelSizes(caps.value?.browser?.sizesMB))
const version = computed(() => caps.value?.browser?.fingerprint?.slice(0, 7) ?? '')
const library = computed(() => store.library)
const coveragePct = computed(() => Math.round((caps.value?.coverage?.ratio ?? 0) * 100))
const libraryStatus = computed(() => {
  const l = library.value
  if (l.phase === 'running')
    return l.placement === 'browser'
      ? `Indexing in this browser · ${l.done + l.failed} of ${l.total}`
      : 'Asking the server…'
  if (l.phase === 'error') return l.message
  if (l.phase !== 'done') return ''
  if (l.placement !== 'browser')
    return l.serverQueued ? `${l.serverQueued} queued on the server.` : 'Nothing left to index.'
  const failed = l.failed ? ` ${l.failed} could not be read here.` : ''
  return `Indexed ${l.done} in this browser.${failed}`
})
const busy = ref(false)
const graphics = ref('')
const threads = threadCount(globalThis.crossOriginIsolated === true, navigator.hardwareConcurrency)

const rows: { kind: AiModelKind; label: string; what: string }[] = [
  { kind: 'text', label: 'Search queries', what: 'search' },
  { kind: 'image', label: 'Photo indexing', what: 'image' },
]
const downloadMB = (kind: AiModelKind) =>
  kind === 'image'
    ? sizes.value.image
    : sizes.value[state.textModel === 'full' ? 'textFull' : 'textSmall']
const choices = (kind: AiModelKind): { id: Placement; title: string; desc: string }[] => [
  {
    id: 'browser',
    title: 'In this browser',
    desc: `Fast on this computer. Downloads ${sizeLabel(downloadMB(kind))} once and keeps it here.`,
  },
  {
    id: 'server',
    title: 'On the server',
    desc: 'Nothing to download. Searches may wait when the server is busy.',
  },
]
const locked = (kind: AiModelKind) => !store.choosable[kind]
const where = (kind: AiModelKind) => placement.value?.[kind] ?? 'server'

async function pick(kind: AiModelKind, to: Placement) {
  if (busy.value || where(kind) === to) return
  busy.value = true
  try {
    await store.setPreference(kind, to)
  } catch (e) {
    toast.error(aiMessage(e))
  } finally {
    busy.value = false
  }
}
async function chooseSize(event: Event) {
  await store.setTextModel((event.target as HTMLSelectElement).value as BrowserTextModel)
}
async function keep() {
  const ok = await store.keepOnDevice()
  if (ok) toast.success('Photoly will keep these models on this device.')
  else toast.error('This browser did not agree. Installing Photoly as an app usually helps.')
}
async function remove() {
  busy.value = true
  try {
    await store.removeFromDevice()
    toast.success('Removed from this device.')
  } finally {
    busy.value = false
  }
}
const storagePct = computed(() =>
  store.storage?.quota
    ? Math.min(100, Math.round((store.storage.usage / store.storage.quota) * 100))
    : 0,
)
const mbLabel = (bytes: number) =>
  bytes >= 1e9 ? `${(bytes / 1e9).toFixed(1)} GB` : `${Math.round(bytes / 1e6)} MB`
const speed = computed(() =>
  state.lastQueryMs == null
    ? 'Measured after your first search'
    : `About ${(state.lastQueryMs / 1000).toFixed(1)} s per search after the first`,
)

onMounted(async () => {
  await store.refreshCapabilities(true)
  await store.refreshSaved()
  const gpu = await store.ai.detectGpu()
  graphics.value =
    gpu.adapter && !gpu.software
      ? `WebGPU${gpu.name ? ` · ${gpu.name}` : ''}`
      : `Not used${gpu.reason ? ` (${gpu.reason})` : ''}`
})

const h2 = 'mb-1 text-base font-semibold'
const grid = 'grid gap-x-12 gap-y-5 border-b pb-8 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]'
</script>

<template>
  <div v-if="placement" class="flex flex-col gap-8">
    <section aria-labelledby="where-h" :class="grid">
      <div>
        <h2 id="where-h" :class="h2">Where AI runs for you</h2>
        <p class="text-muted-foreground text-sm">
          {{
            store.choosable.text || store.choosable.image
              ? 'Your admin lets you choose where AI runs.'
              : 'Your admin decides where AI runs for everyone.'
          }}
        </p>
      </div>
      <div class="flex max-w-[560px] flex-col gap-4">
        <template v-for="row in rows" :key="row.kind">
          <div v-if="!locked(row.kind)" class="flex flex-col gap-2">
            <span :id="`ai-${row.kind}-l`" class="text-sm font-medium">{{ row.label }}</span>
            <div
              role="radiogroup"
              :aria-labelledby="`ai-${row.kind}-l`"
              class="grid gap-2 sm:grid-cols-2"
            >
              <button
                v-for="c in choices(row.kind)"
                :key="c.id"
                type="button"
                role="radio"
                :aria-checked="where(row.kind) === c.id"
                :disabled="busy"
                :class="[
                  'flex items-start gap-2.5 rounded-[10px] border p-3 text-left text-sm',
                  where(row.kind) === c.id
                    ? 'border-primary bg-accent/50 ring-primary/15 ring-[3px]'
                    : 'bg-card',
                ]"
                @click="pick(row.kind, c.id)"
              >
                <span
                  aria-hidden="true"
                  :class="[
                    'mt-0.5 size-4 shrink-0 rounded-full border-[1px] bg-white',
                    where(row.kind) === c.id && 'border-primary border-[5px]',
                  ]"
                />
                <span class="flex flex-col gap-0.5"
                  ><span class="font-medium">{{ c.title }}</span
                  ><span class="text-muted-foreground">{{ c.desc }}</span></span
                >
              </button>
            </div>
          </div>
          <div
            v-else
            class="bg-muted text-muted-foreground flex items-center gap-2.5 rounded-[10px] p-3 text-sm"
          >
            <Lock class="size-4 shrink-0" aria-hidden="true" />
            <span class="flex flex-col"
              ><span class="text-foreground font-medium"
                >{{ row.label }} ·
                {{ where(row.kind) === 'browser' ? 'In the browser' : 'On the server' }}</span
              ><span>Set by your admin for everyone.</span></span
            >
          </div>
        </template>
      </div>
    </section>

    <section aria-labelledby="saved-h" :class="grid">
      <div>
        <h2 id="saved-h" :class="h2">Saved on this device</h2>
        <p class="text-muted-foreground text-sm">
          Models download once and load from here next time, even offline.
        </p>
      </div>
      <div class="flex max-w-[560px] flex-col gap-4">
        <ul class="bg-card divide-y rounded-[14px] border text-sm shadow-xs">
          <li class="flex items-center gap-3 px-4 py-3.5">
            <span class="flex min-w-0 flex-1 flex-col"
              ><span class="font-medium">Search model · {{ state.textModel }}</span
              ><span class="text-muted-foreground text-xs"
                >{{ sizeLabel(sizes[state.textModel === 'full' ? 'textFull' : 'textSmall']) }}
                <template v-if="version">· version {{ version }}</template></span
              ></span
            >
            <span
              v-if="store.saved.text"
              class="bg-accent text-accent-foreground inline-flex h-[22px] items-center gap-1.5 rounded-full px-2 text-xs font-medium"
              ><Check class="size-3" aria-hidden="true" />Saved</span
            >
            <span v-else class="text-muted-foreground text-xs">Not downloaded</span>
          </li>
          <li class="flex items-center gap-3 px-4 py-3.5">
            <span class="flex min-w-0 flex-1 flex-col"
              ><span class="font-medium">Image model</span
              ><span class="text-muted-foreground text-xs">{{
                where('image') === 'browser' || store.saved.image
                  ? sizeLabel(sizes.image)
                  : 'Not needed: photos are indexed on the server'
              }}</span></span
            >
            <span
              v-if="store.saved.image"
              class="bg-accent text-accent-foreground inline-flex h-[22px] items-center gap-1.5 rounded-full px-2 text-xs font-medium"
              ><Check class="size-3" aria-hidden="true" />Saved</span
            >
            <span v-else class="text-muted-foreground text-xs">Not downloaded</span>
          </li>
        </ul>
        <div v-if="store.storage" class="flex flex-col gap-2">
          <div class="flex justify-between text-xs">
            <span class="font-medium">Photoly storage in this browser</span>
            <span class="text-muted-foreground"
              >{{ mbLabel(store.storage.usage) }} of {{ mbLabel(store.storage.quota) }}</span
            >
          </div>
          <Progress :model-value="storagePct" aria-label="Browser storage used" class="h-1" />
        </div>
        <p
          v-if="store.storage && !store.storage.persisted"
          class="flex items-start gap-2.5 rounded-[10px] bg-orange-50 p-3 text-sm text-orange-900 dark:bg-orange-950/40 dark:text-orange-200"
        >
          <Info class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          This browser may clear saved models when disk space runs low. Installing Photoly as an app
          keeps them.
        </p>
        <div class="flex flex-wrap gap-2">
          <Button type="button" variant="outline" @click="keep">Keep on This Device</Button>
          <Button
            type="button"
            variant="ghost"
            class="text-destructive"
            :disabled="busy"
            @click="remove"
            >Remove from This Device</Button
          >
        </div>
      </div>
    </section>

    <section v-if="caps?.enabled" aria-labelledby="lib-h" :class="grid">
      <div>
        <h2 id="lib-h" :class="h2">Your library</h2>
        <p class="text-muted-foreground text-sm">
          Photos are indexed automatically. Index now if you switched between this browser and the
          server, or some photos are missing from search.
        </p>
      </div>
      <div class="flex max-w-[560px] flex-col gap-3">
        <p class="text-sm">
          {{ caps.coverage?.ready ?? 0 }} of {{ caps.coverage?.supported ?? 0 }} photos and videos
          indexed
        </p>
        <Progress :model-value="coveragePct" aria-label="Library indexing progress" class="h-1" />
        <div class="flex items-center gap-3">
          <Button
            type="button"
            variant="outline"
            :disabled="library.phase === 'running'"
            @click="store.indexLibrary()"
            >{{ library.phase === 'running' ? 'Indexing…' : 'Index My Photos Now' }}</Button
          >
          <span role="status" class="text-muted-foreground text-sm">{{ libraryStatus }}</span>
        </div>
      </div>
    </section>

    <section
      aria-labelledby="dev-h"
      class="grid gap-x-12 gap-y-5 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
    >
      <div>
        <h2 id="dev-h" :class="h2">This device</h2>
        <p class="text-muted-foreground text-sm">What Photoly found in this browser.</p>
      </div>
      <div class="flex max-w-[560px] flex-col gap-4">
        <dl class="grid grid-cols-[minmax(0,160px)_minmax(0,1fr)] gap-x-4 gap-y-2.5 text-sm">
          <dt class="text-muted-foreground">Graphics</dt>
          <dd>{{ graphics || 'Checking…' }}</dd>
          <dt class="text-muted-foreground">Processor</dt>
          <dd>{{ threads }} {{ threads === 1 ? 'thread' : 'threads' }}</dd>
          <dt class="text-muted-foreground">Search speed</dt>
          <dd>{{ speed }}</dd>
        </dl>
        <div class="flex max-w-xs flex-col gap-1.5">
          <label for="ai-text-model" class="text-sm font-medium">Search model</label>
          <select
            id="ai-text-model"
            :value="state.textModel"
            class="border-input bg-card h-9 rounded-md border px-2.5 text-sm shadow-xs"
            @change="chooseSize"
          >
            <option value="small">Small · {{ sizeLabel(sizes.textSmall) }} (recommended)</option>
            <option value="full">Full · {{ sizeLabel(sizes.textFull) }}</option>
          </select>
          <span class="text-muted-foreground text-xs"
            >Full is a little more precise and uses about 3 GB of memory while searching.</span
          >
        </div>
      </div>
    </section>
  </div>
</template>
