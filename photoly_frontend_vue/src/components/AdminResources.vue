<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { call } from '@/lib/http'
import { Button } from '@/components/ui/button'
import { RefreshCw } from '@lucide/vue'
const props = defineProps<{ refreshKey?: number }>()
interface Snapshot {
  sampledAt: string
  scope: string
  cpuPercent: number | null
  backendBytes: number | null
  aiBytes: number | null
}
interface Point {
  bootId: string
  bucket: string
  firstSampleAt: string
  lastSampleAt: string
  cpuSamples: number
  memorySamples: number
  cpuMean: number | null
  cpuPeak: number | null
  backendMeanBytes: number | null
  aiMeanBytes: number | null
  memoryPeakBytes: number | null
  partial: boolean
}
const current = ref<Snapshot>()
const points = ref<Point[]>([])
const hours = ref(24)
const error = ref('')
const historyError = ref('')
const loading = ref(true)
const selected = ref<Point>()
const end = ref(Date.now())
let sequence = 0
let lastHistory = 0
let busy = false
let disposed = false
const bytes = (v: number | null | undefined) =>
  v == null
    ? 'Unavailable'
    : v >= 1024 ** 3
      ? `${(v / 1024 ** 3).toFixed(2)} GB`
      : `${(v / 1024 ** 2).toFixed(1)} MB`
const percent = (v: number | null | undefined) => (v == null ? 'Unavailable' : `${v.toFixed(1)}%`)
const stamp = (v: string) => new Date(v).toLocaleString()
const total = computed(() =>
  current.value?.backendBytes != null && current.value.aiBytes != null
    ? current.value.backendBytes + current.value.aiBytes
    : null,
)
const aiPercent = computed(() => (total.value ? (100 * current.value!.aiBytes!) / total.value : 0))
const sorted = computed(() =>
  [...points.value].sort((a, b) => Date.parse(a.firstSampleAt) - Date.parse(b.firstSampleAt)),
)
const peakMemory = computed(() => Math.max(1, ...points.value.map((p) => p.memoryPeakBytes ?? 0)))
const chartKinds = ['cpu', 'memory'] as const
const x = (p: Point) =>
  Math.max(
    8,
    Math.min(
      512,
      8 +
        (504 *
          ((Date.parse(p.firstSampleAt) + Date.parse(p.lastSampleAt)) / 2 -
            (end.value - hours.value * 3600000))) /
          (hours.value * 3600000),
    ),
  )
const value = (p: Point, kind: string, ai = true) =>
  kind === 'cpu'
    ? p.cpuMean
    : p.backendMeanBytes == null || p.aiMeanBytes == null
      ? null
      : p.backendMeanBytes + (ai ? p.aiMeanBytes : 0)
const y = (v: number, kind: string) => 172 - (156 * v) / (kind === 'cpu' ? 100 : peakMemory.value)
function groups(kind: string) {
  const result: Point[][] = []
  let previous: Point | undefined
  for (const p of sorted.value) {
    if (value(p, kind) == null) {
      previous = undefined
      continue
    }
    if (
      !previous ||
      previous.bootId !== p.bootId ||
      Date.parse(p.firstSampleAt) - Date.parse(previous.lastSampleAt) > 30000
    )
      result.push([])
    result[result.length - 1]!.push(p)
    previous = p
  }
  return result
}
const line = (group: Point[], kind: string, ai = true) =>
  group.map((p) => `${x(p)},${y(value(p, kind, ai)!, kind)}`).join(' ')
const area = (group: Point[], ai: boolean) =>
  `${line(group, 'memory', ai)} ${[...group]
    .reverse()
    .map((p) => `${x(p)},${ai ? y(value(p, 'memory', false)!, 'memory') : 172}`)
    .join(' ')}`
function describe(p: Point) {
  return `${stamp(p.firstSampleAt)} – ${new Date(p.lastSampleAt).toLocaleTimeString()}: CPU ${percent(p.cpuMean)} (peak ${percent(p.cpuPeak)}); Backend ${bytes(p.backendMeanBytes)}, AI ${bytes(p.aiMeanBytes)} (total peak ${bytes(p.memoryPeakBytes)}). ${p.partial ? 'Collecting' : ''}`
}
async function load(history = false) {
  if (disposed || document.hidden || (busy && !history)) return
  const seq = ++sequence
  busy = true
  const readHistory = history || Date.now() - lastHistory >= 600000
  await Promise.all([
    call<Snapshot>({ url: '/admin/resources/current' })
      .then((v) => {
        if (seq === sequence && !disposed) {
          current.value = v
          error.value = ''
        }
      })
      .catch(() => {
        if (seq === sequence && !disposed)
          error.value = 'Could not refresh resource usage. Showing the last known sample.'
      }),
    readHistory
      ? call<{ items: Point[] }>({
          url: '/admin/resources/history',
          params: { hours: hours.value },
        })
          .then((v) => {
            if (seq === sequence && !disposed) {
              points.value = v.items
              selected.value = undefined
              lastHistory = Date.now()
              end.value = Date.now()
              historyError.value = ''
            }
          })
          .catch(() => {
            if (seq === sequence && !disposed)
              historyError.value = 'Could not load resource history.'
          })
      : Promise.resolve(),
  ])
  if (seq === sequence) {
    loading.value = false
    busy = false
  }
}
const visible = () => {
  if (!document.hidden) void load(true)
}
let timer: ReturnType<typeof setInterval>
onMounted(() => {
  void load(true)
  timer = setInterval(() => void load(), 30000)
  document.addEventListener('visibilitychange', visible)
})
onUnmounted(() => {
  disposed = true
  sequence++
  clearInterval(timer)
  document.removeEventListener('visibilitychange', visible)
})
watch(hours, () => {
  points.value = []
  void load(true)
})
watch(
  () => props.refreshKey,
  () => void load(true),
)
</script>
<template>
  <section aria-labelledby="resource-h" class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center gap-3">
      <h2 id="resource-h" class="mr-auto text-base font-semibold">Resource</h2>
      <div class="bg-muted flex rounded-lg p-1" aria-label="Resource history range">
        <button
          v-for="h in [12, 24]"
          :key="h"
          type="button"
          :aria-pressed="hours === h"
          :class="[
            'rounded-md px-3 py-1.5 text-sm focus-visible:outline-2 focus-visible:outline-primary',
            hours === h && 'bg-background shadow-sm',
          ]"
          @click="hours = h"
        >
          {{ h }} hours
        </button>
      </div>
      <Button variant="outline" size="sm" aria-label="Refresh resources" @click="load(true)"
        ><RefreshCw class="size-4" />Refresh</Button
      >
    </div>
    <p v-if="error || historyError" role="alert" class="text-destructive text-sm">
      {{ [error, historyError].filter(Boolean).join(' ') }}
    </p>
    <p v-if="loading" role="status" class="text-muted-foreground text-sm">Loading resources…</p>
    <div class="grid gap-4 md:grid-cols-2">
      <article
        v-for="kind in chartKinds"
        :key="kind"
        class="bg-card flex min-w-0 flex-col gap-4 rounded-xl border p-5 shadow-xs"
      >
        <div>
          <h3 class="font-medium">{{ kind === 'cpu' ? 'CPU' : 'Memory' }}</h3>
          <p class="text-muted-foreground mt-1 min-h-10 text-xs">
            {{
              kind === 'cpu'
                ? 'Host CPU · container-visible Linux host / VM'
                : 'Backend (includes Scheduler) + AI worker · resident memory'
            }}
          </p>
        </div>
        <div
          class="relative mx-auto size-44"
          role="img"
          :aria-label="
            kind === 'cpu' ? `CPU ${percent(current?.cpuPercent)}` : `Memory ${bytes(total)}`
          "
        >
          <svg viewBox="0 0 100 100" class="size-full -rotate-90" aria-hidden="true">
            <circle cx="50" cy="50" r="42" fill="none" class="stroke-muted" stroke-width="8" />
            <circle
              v-if="kind === 'cpu' && current?.cpuPercent != null"
              cx="50"
              cy="50"
              r="42"
              fill="none"
              class="stroke-primary"
              stroke-width="8"
              pathLength="100"
              :stroke-dasharray="`${current.cpuPercent} 100`"
              stroke-linecap="round"
            />
            <template v-if="kind === 'memory' && total">
              <circle cx="50" cy="50" r="42" fill="none" class="stroke-primary" stroke-width="8" />
              <circle
                cx="50"
                cy="50"
                r="42"
                fill="none"
                class="stroke-violet-500"
                stroke-width="8"
                pathLength="100"
                :stroke-dasharray="`${aiPercent} 100`"
              />
            </template>
          </svg>
          <div class="absolute inset-0 flex flex-col items-center justify-center">
            <strong class="text-xl tabular-nums">{{
              kind === 'cpu' ? percent(current?.cpuPercent) : bytes(total)
            }}</strong
            ><span class="text-muted-foreground mt-1 text-xs">{{
              kind === 'cpu' ? 'CPU usage' : 'Total RSS'
            }}</span>
          </div>
        </div>
        <div v-if="kind === 'memory'" class="flex flex-wrap justify-center gap-x-5 gap-y-1 text-xs">
          <span
            ><i class="bg-primary mr-1 inline-block size-2 rounded-full" />Backend
            {{ bytes(current?.backendBytes) }}</span
          ><span
            ><i class="mr-1 inline-block size-2 rounded-full bg-violet-500" />AI
            {{ bytes(current?.aiBytes) }}</span
          >
        </div>
        <div v-else class="text-muted-foreground text-center text-xs">
          0–100% of host CPU capacity
        </div>
        <div class="mt-auto border-t pt-4">
          <div class="text-muted-foreground flex justify-between text-xs">
            <span>Past {{ hours }} hours · 10 min averages</span
            ><span>{{ kind === 'cpu' ? '100%' : bytes(peakMemory > 1 ? peakMemory : null) }}</span>
          </div>
          <svg
            viewBox="0 0 520 184"
            class="mt-2 w-full overflow-visible"
            :aria-label="`${kind === 'cpu' ? 'CPU' : 'Memory'} history`"
            role="group"
          >
            <path d="M8 16H512 M8 94H512 M8 172H512" class="stroke-border" fill="none" />
            <g v-for="(group, index) in groups(kind)" :key="index">
              <template v-if="kind === 'memory'">
                <polygon :points="area(group, false)" class="fill-primary/25" />
                <polygon :points="area(group, true)" class="fill-violet-500/40" />
              </template>
              <polyline
                :points="line(group, kind)"
                fill="none"
                class="stroke-primary"
                stroke-width="2"
              />
              <circle
                v-for="p in group"
                :key="`${p.bootId}-${p.bucket}`"
                :cx="x(p)"
                :cy="y(value(p, kind)!, kind)"
                r="4"
                tabindex="0"
                class="fill-primary cursor-pointer focus:stroke-foreground focus:stroke-2"
                :aria-label="describe(p)"
                @focus="selected = p"
                @pointerenter="selected = p"
                @click="selected = p"
              >
                <title>{{ describe(p) }}</title>
              </circle>
            </g>
          </svg>
          <div class="text-muted-foreground flex justify-between text-xs">
            <span>{{
              new Date(end - hours * 3600000).toLocaleTimeString([], {
                hour: '2-digit',
                minute: '2-digit',
              })
            }}</span
            ><span>{{
              new Date(end).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            }}</span>
          </div>
          <p v-if="!groups(kind).length" class="text-muted-foreground mt-2 text-sm">
            {{ historyError ? 'History unavailable.' : 'Collecting history. No samples yet.' }}
          </p>
        </div>
      </article>
    </div>
    <p
      v-if="current?.sampledAt && !current.sampledAt.startsWith('0001')"
      class="text-muted-foreground text-xs"
    >
      Sampled {{ stamp(current.sampledAt) }} · Refreshes every 30 seconds · History is retained for
      48 hours.
    </p>
    <p v-if="selected" role="status" class="bg-muted rounded-lg p-3 text-xs">
      {{ describe(selected) }}
    </p>
    <details class="text-sm">
      <summary class="cursor-pointer">History data · {{ points.length }} intervals</summary>
      <div class="mt-3 overflow-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr>
              <th class="p-2">Time</th>
              <th>CPU avg / peak</th>
              <th>Backend</th>
              <th>AI</th>
              <th>Memory peak</th>
              <th>Samples CPU / memory</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in sorted" :key="`${p.bootId}-${p.bucket}`" class="border-t">
              <td class="p-2">
                {{ stamp(p.firstSampleAt) }}{{ p.partial ? ' · Collecting' : '' }}
              </td>
              <td>{{ percent(p.cpuMean) }} / {{ percent(p.cpuPeak) }}</td>
              <td>{{ bytes(p.backendMeanBytes) }}</td>
              <td>{{ bytes(p.aiMeanBytes) }}</td>
              <td>{{ bytes(p.memoryPeakBytes) }}</td>
              <td>{{ p.cpuSamples }} / {{ p.memorySamples }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </details>
  </section>
</template>
