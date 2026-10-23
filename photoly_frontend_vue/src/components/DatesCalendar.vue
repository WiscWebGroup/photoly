<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useStorage } from '@vueuse/core'
import { CalendarDays, ChevronLeft, ChevronRight, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import type { DateSummary } from '@/composables/useDateTimeline'

const props = defineProps<{
  summary: DateSummary
  anchor?: string
  selected?: string
  selecting?: boolean
}>()
const emit = defineEmits<{ pick: [day: string] }>()
const shown = useStorage('photoly-dates-calendar-visible', true)
const month = ref('')
const counts = computed(() => new Map(props.summary.days.map((d) => [d.date, d.count])))
const years = computed(() => {
  const first = Number(props.summary.days[0]?.date.slice(0, 4))
  const last = Number(props.summary.days.at(-1)?.date.slice(0, 4))
  return first && last
    ? Array.from({ length: last - first + 1 }, (_, index) => String(last - index).padStart(4, '0'))
    : []
})
watch(
  () => props.anchor,
  (value) => {
    if (value) month.value = value.slice(0, 7)
  },
  { immediate: true },
)
watch(
  years,
  () => {
    if (!years.value.includes(month.value.slice(0, 4)))
      month.value = props.summary.days.at(-1)?.date.slice(0, 7) ?? ''
  },
  { immediate: true },
)
const parse = (value: string) => new Date(`${value}T00:00:00`)
const monthNames = Array.from({ length: 12 }, (_, index) => ({
  value: String(index + 1).padStart(2, '0'),
  label: new Date(2024, index, 1).toLocaleDateString(undefined, { month: 'long' }),
}))
const monthCount = computed(() =>
  props.summary.days.reduce((n, d) => n + (d.date.startsWith(month.value) ? d.count : 0), 0),
)
const cells = computed(() => {
  if (!month.value) return []
  const first = parse(`${month.value}-01`)
  const last = new Date(first)
  last.setMonth(last.getMonth() + 1, 0)
  return [
    ...Array.from({ length: (first.getDay() + 6) % 7 }, () => null),
    ...Array.from(
      { length: last.getDate() },
      (_, i) => `${month.value}-${String(i + 1).padStart(2, '0')}`,
    ),
  ]
})
const firstMonth = computed(() => props.summary.days[0]?.date.slice(0, 7) ?? '')
const lastMonth = computed(() => props.summary.days.at(-1)?.date.slice(0, 7) ?? '')
function step(delta: number) {
  const date = parse(`${month.value}-01`)
  date.setMonth(date.getMonth() + delta)
  month.value = `${String(date.getFullYear()).padStart(4, '0')}-${String(date.getMonth() + 1).padStart(2, '0')}`
}
function selectYear(event: Event) {
  month.value = `${(event.target as HTMLSelectElement).value}-${month.value.slice(5)}`
}
function selectMonth(event: Event) {
  month.value = `${month.value.slice(0, 4)}-${(event.target as HTMLSelectElement).value}`
}
</script>

<template>
  <aside
    v-if="years.length"
    aria-label="Photo calendar"
    class="fixed right-4 z-30 max-w-[calc(100vw-2rem)] md:right-6"
    :class="selecting ? 'bottom-36 md:bottom-20' : 'bottom-20 md:bottom-6'"
  >
    <Button
      v-if="!shown"
      size="icon"
      class="shadow-lg"
      aria-label="Show photo calendar"
      @click="shown = true"
    >
      <CalendarDays aria-hidden="true" />
    </Button>
    <div v-else class="bg-card w-72 max-h-[65dvh] overflow-auto rounded-xl border p-3 shadow-lg">
      <div class="mb-2 flex items-center gap-2">
        <CalendarDays class="size-4" aria-hidden="true" />
        <h2 class="text-sm font-semibold">Photo calendar</h2>
        <Button
          variant="ghost"
          size="icon-sm"
          class="ml-auto"
          aria-label="Hide photo calendar"
          @click="shown = false"
          ><X aria-hidden="true"
        /></Button>
      </div>
      <div class="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Previous month"
          :disabled="month <= firstMonth"
          @click="step(-1)"
          ><ChevronLeft aria-hidden="true"
        /></Button>
        <select
          aria-label="Calendar month"
          :value="month.slice(5)"
          class="bg-card min-w-0 flex-1 rounded border p-1 text-sm"
          @change="selectMonth"
        >
          <option v-for="m in monthNames" :key="m.value" :value="m.value">{{ m.label }}</option>
        </select>
        <select
          aria-label="Calendar year"
          :value="month.slice(0, 4)"
          class="bg-card rounded border p-1 text-sm"
          @change="selectYear"
        >
          <option v-for="year in years" :key="year" :value="year">{{ year }}</option>
        </select>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Next month"
          :disabled="month >= lastMonth"
          @click="step(1)"
          ><ChevronRight aria-hidden="true"
        /></Button>
      </div>
      <p class="text-muted-foreground my-2 text-center text-xs" aria-live="polite">
        {{ monthCount }} {{ monthCount === 1 ? 'photo' : 'photos' }} this month
      </p>
      <div class="grid grid-cols-7 gap-0.5">
        <span
          v-for="day in ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']"
          :key="day"
          class="text-muted-foreground py-1 text-center text-xs"
          >{{ day }}</span
        >
        <template v-for="(day, i) in cells" :key="day ?? `blank-${i}`">
          <span v-if="!day" />
          <button
            v-else
            type="button"
            :disabled="!counts.has(day)"
            :aria-label="`${day}, ${counts.get(day) ?? 0} ${counts.get(day) === 1 ? 'photo' : 'photos'}`"
            :aria-pressed="selected === day"
            class="flex h-11 flex-col items-center justify-center rounded-md text-xs transition-colors enabled:hover:bg-accent disabled:opacity-35"
            :class="selected === day ? 'bg-primary/15 ring-primary ring-1' : ''"
            @click="emit('pick', day)"
          >
            <span class="font-medium">{{ Number(day.slice(8)) }}</span>
            <span
              v-if="counts.has(day)"
              class="text-primary max-w-full truncate text-[10px] tabular-nums"
              >{{ counts.get(day)!.toLocaleString() }}</span
            >
          </button>
        </template>
      </div>
    </div>
  </aside>
</template>
