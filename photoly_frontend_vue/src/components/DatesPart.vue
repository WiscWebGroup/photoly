<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronDown, ChevronRight } from '@lucide/vue'
import MediaList from '@/components/MediaList.vue'
import DatePhotoPage from '@/components/DatePhotoPage.vue'
import DatesCalendar from '@/components/DatesCalendar.vue'
import DateMonthPreview from '@/components/DateMonthPreview.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Button } from '@/components/ui/button'
import { useDateTimeline } from '@/composables/useDateTimeline'
import { useAuthStore } from '@/stores/auth'
import { useUploadsStore } from '@/stores/uploads'

const auth = useAuthStore()
const uploads = useUploadsStore()
const route = useRoute()
const router = useRouter()
const order = computed(() => (route.query.order === 'oldest' ? 'oldest' : 'latest'))
const media = ref<InstanceType<typeof MediaList>>()
const root = ref<HTMLElement>()
const selectMode = ref(false)
const closedYears = ref(new Set<string>())
const openMonths = ref(new Set<string>())
const undatedOpen = ref(false)
const calendarAnchor = ref('')
const selectedDay = ref('')
const jumping = ref(false)
const hasJumped = ref(false)
const previewRevision = ref(0)
let highlightTimer: ReturnType<typeof setTimeout> | undefined
let jumpVersion = 0
let fresh = true
const { summary, loading, error, photos, pagesFor, setVisible, cancelDay, refresh, retry } =
  useDateTimeline(() => order.value)
const years = computed(() => {
  const grouped = new Map<
    string,
    {
      key: string
      count: number
      months: { key: string; count: number; days: { date: string; count: number }[] }[]
    }
  >()
  const days = [...(summary.value?.days ?? [])].sort((a, b) =>
    order.value === 'latest' ? b.date.localeCompare(a.date) : a.date.localeCompare(b.date),
  )
  for (const day of days) {
    const yearKey = day.date.slice(0, 4),
      monthKey = day.date.slice(0, 7)
    let year = grouped.get(yearKey)
    if (!year) {
      year = { key: yearKey, count: 0, months: [] }
      grouped.set(yearKey, year)
    }
    let month = year.months.at(-1)
    if (month?.key !== monthKey) {
      month = { key: monthKey, count: 0, days: [] }
      year.months.push(month)
    }
    month.count += day.count
    month.days.push(day)
    year.count += day.count
  }
  return [...grouped.values()]
})
const monthLabel = (key: string) =>
  new Date(`${key}-01T00:00:00`).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
const dayLabel = (key: string) =>
  new Date(`${key}T00:00:00`).toLocaleDateString(undefined, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
function toggleMonth(key: string) {
  if (openMonths.value.has(key)) {
    openMonths.value.delete(key)
    for (const day of summary.value?.days ?? []) if (day.date.startsWith(key)) cancelDay(day.date)
  } else {
    openMonths.value.add(key)
    calendarAnchor.value = `${key}-01`
  }
}
function toggleYear(key: string) {
  if (closedYears.value.has(key)) closedYears.value.delete(key)
  else {
    closedYears.value.add(key)
    for (const day of summary.value?.days ?? []) if (day.date.startsWith(key)) cancelDay(day.date)
  }
}
function toggleUndated() {
  undatedOpen.value = !undatedOpen.value
  if (!undatedOpen.value) cancelDay('undated')
}
async function jump(day: string) {
  const version = ++jumpVersion
  jumping.value = true
  hasJumped.value = true
  closedYears.value.delete(day.slice(0, 4))
  openMonths.value.add(day.slice(0, 7))
  selectedDay.value = day
  calendarAnchor.value = day
  await nextTick()
  const target = root.value?.querySelector<HTMLElement>(`[data-date-day="${day}"]`)
  // Instant positioning avoids requesting intermediate dates during a long smooth scroll.
  target?.scrollIntoView?.({ block: 'start', behavior: 'instant' })
  target?.focus({ preventScroll: true })
  await nextTick()
  if (version !== jumpVersion) return
  jumping.value = false
  clearTimeout(highlightTimer)
  highlightTimer = setTimeout(() => {
    selectedDay.value = ''
  }, 2500)
}
watch(summary, (value) => {
  if (!value) return
  previewRevision.value++
  if (fresh) {
    const first = years.value[0]?.months[0]
    if (first) {
      openMonths.value.add(first.key)
      calendarAnchor.value = first.days[0]!.date
    } else undatedOpen.value = value.undatedCount > 0
    fresh = false
  }
})
watch(photos, (value) => media.value?.updatePhotoList(value), { flush: 'post' })
watch(
  [order, () => auth.token],
  () => {
    fresh = true
    hasJumped.value = false
    jumpVersion++
    jumping.value = false
    closedYears.value.clear()
    openMonths.value.clear()
    undatedOpen.value = false
    selectedDay.value = ''
    calendarAnchor.value = ''
    selectMode.value = false
    void refresh(!!auth.token)
  },
  { immediate: true },
)
watch(
  () => uploads.revision,
  () => refresh(!!auth.token),
)
onBeforeUnmount(() => {
  jumpVersion++
  clearTimeout(highlightTimer)
})
</script>

<template>
  <div ref="root" class="flex flex-col gap-6 p-4 pb-24 md:p-10">
    <PageHeader
      title="Dates"
      description="All your photos, by capture date. Upload date is used when capture date is unavailable."
    >
      <div class="flex gap-1" role="group" aria-label="Date order">
        <Button
          v-for="value in ['latest', 'oldest'] as const"
          :key="value"
          :variant="order === value ? 'default' : 'outline'"
          :aria-pressed="order === value"
          @click="router.replace({ query: { ...route.query, order: value } })"
          >{{ value === 'latest' ? 'Latest' : 'Oldest' }}</Button
        >
      </div>
    </PageHeader>
    <MediaList
      ref="media"
      v-model:select-mode="selectMode"
      :ns-or-not="false"
      timeline
      retain-state
      context="Dates"
      heading="Photos"
      :total-count="summary?.totalCount"
      @query-photos="refresh(!!auth.token)"
    >
      <template #content="{ actions, selectMode: selecting }">
        <p v-if="loading" role="status" class="text-muted-foreground text-sm">Loading dates…</p>
        <div v-else-if="error" role="alert" class="flex items-center gap-3">
          Could not load dates.
          <Button variant="outline" @click="refresh(!!auth.token)">Retry</Button>
        </div>
        <EmptyState
          v-else-if="summary && !summary.totalCount"
          title="No photos yet"
          description="Uploaded photos from all your folders will appear here."
        />
        <template v-else-if="summary">
          <p class="text-muted-foreground text-sm">
            {{ summary.totalCount.toLocaleString() }} photos
          </p>
          <section
            v-for="year in years"
            :key="year.key"
            :aria-label="year.key"
            class="flex flex-col gap-4"
          >
            <h2>
              <button
                type="button"
                class="flex w-full items-center gap-2 border-b py-2 text-left text-xl font-semibold"
                :aria-expanded="!closedYears.has(year.key)"
                :aria-label="`Year ${year.key}, ${year.count} ${year.count === 1 ? 'photo' : 'photos'}`"
                @click="toggleYear(year.key)"
              >
                <component
                  :is="closedYears.has(year.key) ? ChevronRight : ChevronDown"
                  class="size-5"
                  aria-hidden="true"
                />
                {{ year.key }}
                <span class="text-muted-foreground text-sm font-normal"
                  >{{ year.count.toLocaleString() }}
                  {{ year.count === 1 ? 'photo' : 'photos' }}</span
                >
              </button>
            </h2>
            <template v-if="!closedYears.has(year.key)">
              <section
                v-for="month in year.months"
                :key="month.key"
                class="flex flex-col gap-4"
                :aria-label="monthLabel(month.key)"
              >
                <h3>
                  <button
                    type="button"
                    class="bg-card hover:bg-accent/60 w-full min-w-0 rounded-xl border p-3 text-left font-medium transition-colors"
                    :aria-expanded="openMonths.has(month.key)"
                    :aria-label="`${monthLabel(month.key)}, ${month.count} ${month.count === 1 ? 'photo' : 'photos'}`"
                    @click="toggleMonth(month.key)"
                  >
                    <span class="flex items-center gap-2">
                      <component
                        :is="openMonths.has(month.key) ? ChevronDown : ChevronRight"
                        class="size-4"
                        aria-hidden="true"
                      />
                      {{ monthLabel(month.key) }}
                      <span class="text-muted-foreground ml-auto text-sm font-normal"
                        >{{ month.count.toLocaleString() }}
                        {{ month.count === 1 ? 'photo' : 'photos' }}</span
                      >
                    </span>
                    <DateMonthPreview
                      :month="month.key"
                      :order="order"
                      :count="month.count"
                      :revision="previewRevision"
                    />
                  </button>
                </h3>
                <template v-if="openMonths.has(month.key)">
                  <section
                    v-for="day in month.days"
                    :key="day.date"
                    :data-date-day="day.date"
                    tabindex="-1"
                    class="flex scroll-mt-4 flex-col gap-3 rounded-lg outline-none transition-colors"
                    :class="selectedDay === day.date ? 'bg-primary/5 ring-primary/40 ring-2' : ''"
                  >
                    <h4 class="flex items-baseline gap-2 text-sm font-medium">
                      {{ dayLabel(day.date) }}
                      <span class="text-muted-foreground text-xs font-normal"
                        >{{ day.count.toLocaleString() }}
                        {{ day.count === 1 ? 'photo' : 'photos' }}</span
                      >
                    </h4>
                    <DatePhotoPage
                      v-for="page in pagesFor(day.date)"
                      :key="page.key"
                      :page="page"
                      :actions="actions"
                      :select-mode="selecting"
                      :suspended="jumping"
                      @visibility="setVisible(page, $event)"
                      @retry="retry(page)"
                    />
                  </section>
                </template>
              </section>
            </template>
          </section>
          <section v-if="summary.undatedCount" aria-label="Undated" class="flex flex-col gap-4">
            <h2>
              <button
                type="button"
                class="flex w-full items-center gap-2 border-b py-2 text-left text-xl font-semibold"
                :aria-expanded="undatedOpen"
                @click="toggleUndated"
              >
                <component
                  :is="undatedOpen ? ChevronDown : ChevronRight"
                  class="size-5"
                  aria-hidden="true"
                />Undated
                <span class="text-muted-foreground text-sm font-normal"
                  >{{ summary.undatedCount }}
                  {{ summary.undatedCount === 1 ? 'photo' : 'photos' }}</span
                >
              </button>
            </h2>
            <template v-if="undatedOpen">
              <DatePhotoPage
                v-for="page in pagesFor('undated')"
                :key="page.key"
                :page="page"
                :actions="actions"
                :select-mode="selecting"
                @visibility="setVisible(page, $event)"
                @retry="retry(page)"
              />
            </template>
          </section>
        </template>
      </template>
    </MediaList>
    <!-- Leave room to align even the last day's photos above the floating calendar. -->
    <div v-if="hasJumped" aria-hidden="true" class="h-[75dvh] shrink-0" />
    <DatesCalendar
      v-if="summary"
      :summary="summary"
      :anchor="calendarAnchor"
      :selected="selectedDay"
      :selecting="selectMode"
      @pick="jump"
    />
  </div>
</template>
