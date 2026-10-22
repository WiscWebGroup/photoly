<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight } from '@lucide/vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** An inclusive range of `YYYY-MM-DD` days; `null` means all time. */
export type DateRange = { from: string; to: string } | null

// `dates` are the taken dates of the photos on the map, for the dots and the photo count.
defineOptions({ inheritAttrs: false })
const props = defineProps<{ dates: string[] }>()
const range = defineModel<DateRange>({ default: null })
const open = ref(false)

const pad = (n: number) => String(n).padStart(2, '0')
const iso = (y: number, m: number, d: number) => `${y}-${pad(m + 1)}-${pad(d)}`
const parse = (day: string) => new Date(`${day}T00:00:00`)

/** "Jun 9 – 16, 2026", in the reader's locale. */
function label(r: DateRange) {
  if (!r) return 'All time'
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).formatRange(parse(r.from), parse(r.to))
}

const photoDays = computed(() => new Set(props.dates))
const years = computed(() =>
  [...new Set(props.dates.map((d) => d.slice(0, 4)))].sort().reverse().slice(0, 3),
)
const yearRange = (y: string) => ({ from: `${y}-01-01`, to: `${y}-12-31` })

// The popover edits a draft; "Show N Photos" applies it.
const draft = ref<DateRange>(null)
const picking = ref<string | null>(null)
const month = ref(new Date())
watch(open, (isOpen) => {
  if (!isOpen) return
  draft.value = range.value
  picking.value = null
  const anchor = range.value?.to ?? [...props.dates].sort().at(-1)
  month.value = anchor ? parse(anchor) : new Date()
})

const presets = computed(() => [
  { value: 'all', label: 'All time' },
  ...years.value.map((y) => ({ value: y, label: y })),
  { value: 'custom', label: 'Custom' },
])
const preset = computed(() => {
  if (!draft.value) return 'all'
  const y = years.value.find(
    (y) => draft.value!.from === `${y}-01-01` && draft.value!.to === `${y}-12-31`,
  )
  return y ?? 'custom'
})
function choose(value: string) {
  picking.value = null
  if (value === 'all') draft.value = null
  else if (value !== 'custom') {
    draft.value = yearRange(value)
    month.value = parse(`${value}-12-01`)
  }
}

const monthLabel = computed(() =>
  month.value.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }),
)
function step(by: number) {
  month.value = new Date(month.value.getFullYear(), month.value.getMonth() + by, 1)
}
/** Leading blanks (weeks start on Monday) then the month's days. */
const cells = computed(() => {
  const y = month.value.getFullYear()
  const m = month.value.getMonth()
  const blanks = (new Date(y, m, 1).getDay() + 6) % 7
  const days = new Date(y, m + 1, 0).getDate()
  return [
    ...Array.from({ length: blanks }, () => null),
    ...Array.from({ length: days }, (_, i) => iso(y, m, i + 1)),
  ]
})
const weekdays = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

/** First click starts a range, the second ends it (in either order). */
function pick(day: string) {
  if (!picking.value) {
    picking.value = day
    draft.value = { from: day, to: day }
  } else {
    const [from, to] = [picking.value, day].sort() as [string, string]
    draft.value = { from, to }
    picking.value = null
  }
}
const edge = (day: string) => !!draft.value && (day === draft.value.from || day === draft.value.to)
const inside = (day: string) => !!draft.value && day > draft.value.from && day < draft.value.to

const draftCount = computed(() =>
  draft.value
    ? props.dates.filter((d) => d >= draft.value!.from && d <= draft.value!.to).length
    : props.dates.length,
)
function apply() {
  range.value = draft.value
  open.value = false
}
function clear() {
  range.value = null
  open.value = false
}

const chip =
  'inline-flex h-7 items-center rounded-full border px-2.5 text-sm transition-colors hover:bg-accent'
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <button
        type="button"
        :aria-label="`Date range: ${label(range)}`"
        :class="
          cn(
            'bg-card flex h-9 items-center gap-1.5 rounded-lg border px-3 text-sm shadow-[0_2px_6px_rgb(0_0_0/0.08)] transition-colors hover:bg-accent',
            $attrs.class as string,
          )
        "
      >
        <CalendarDays class="text-muted-foreground size-4" aria-hidden="true" />
        {{ label(range) }}
        <ChevronDown class="text-muted-foreground size-3.5" aria-hidden="true" />
      </button>
    </PopoverTrigger>
    <PopoverContent align="start" class="w-[344px] max-w-[calc(100vw-2rem)] p-0">
      <div class="flex flex-col gap-3 border-b p-3.5 pb-3">
        <div class="flex items-baseline gap-2">
          <h2 class="text-sm font-semibold">Date range</h2>
          <span class="text-muted-foreground ml-auto text-sm">{{ label(draft) }}</span>
        </div>
        <div role="radiogroup" aria-label="Presets" class="flex flex-wrap gap-1.5">
          <button
            v-for="p in presets"
            :key="p.value"
            type="button"
            role="radio"
            :aria-checked="preset === p.value"
            :class="
              cn(
                chip,
                preset === p.value &&
                  'border-primary bg-primary/10 text-foreground hover:bg-primary/15',
              )
            "
            @click="choose(p.value)"
          >
            {{ p.label }}
          </button>
        </div>
      </div>
      <div class="flex flex-col gap-2 px-3.5 py-3">
        <div class="flex items-center gap-1">
          <Button variant="ghost" size="icon-sm" aria-label="Previous month" @click="step(-1)">
            <ChevronLeft aria-hidden="true" />
          </Button>
          <span class="flex-1 text-center text-sm font-medium">{{ monthLabel }}</span>
          <Button variant="ghost" size="icon-sm" aria-label="Next month" @click="step(1)">
            <ChevronRight aria-hidden="true" />
          </Button>
        </div>
        <div class="grid grid-cols-7 gap-y-0.5">
          <span
            v-for="w in weekdays"
            :key="w"
            class="text-muted-foreground grid h-6 place-items-center text-xs"
            >{{ w }}</span
          >
          <template v-for="(day, i) in cells" :key="day ?? `blank-${i}`">
            <span v-if="!day" />
            <button
              v-else
              type="button"
              :aria-label="`${parse(day).toLocaleDateString(undefined, { day: 'numeric', month: 'long' })}${photoDays.has(day) ? ', has photos' : ''}`"
              :aria-pressed="edge(day) || inside(day)"
              :class="
                cn(
                  'relative grid h-10 place-items-center rounded-lg text-sm transition-colors hover:bg-accent',
                  inside(day) && 'bg-primary/10 rounded-none hover:bg-primary/15',
                  edge(day) && 'bg-primary text-primary-foreground font-semibold hover:bg-primary',
                )
              "
              @click="pick(day)"
            >
              {{ Number(day.slice(8)) }}
              <span
                v-if="photoDays.has(day)"
                aria-hidden="true"
                :class="
                  cn(
                    'absolute bottom-1 left-1/2 size-1 -translate-x-1/2 rounded-full',
                    edge(day) ? 'bg-primary-foreground' : 'bg-primary',
                  )
                "
              />
            </button>
          </template>
        </div>
        <p class="text-muted-foreground flex items-center gap-1.5 text-xs">
          <span class="bg-primary size-1 rounded-full" aria-hidden="true" />Days with photos
        </p>
      </div>
      <div class="flex items-center gap-2 border-t px-3.5 py-3">
        <Button variant="ghost" size="sm" @click="clear">Clear</Button>
        <Button size="sm" class="ml-auto" @click="apply">
          Show {{ draftCount }} {{ draftCount === 1 ? 'Photo' : 'Photos' }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
