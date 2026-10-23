import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { call } from '@/lib/http'
import type { Photo } from './usePhotos'

export interface DateSummary {
  totalCount: number
  undatedCount: number
  days: { date: string; count: number }[]
}
export interface DatePage {
  key: string
  day: string
  cursor: string | null
  items: Photo[] | undefined
  loading: boolean
  error: boolean
  visible: boolean
  touched: number
}

/** Cursor descriptors survive eviction; photo records do not. All requests share one queue. */
export function useDateTimeline(order: () => 'latest' | 'oldest') {
  const summary = ref<DateSummary>()
  const loading = ref(false)
  const error = ref(false)
  const pages = reactive(new Map<string, DatePage[]>())
  const requests = new Map<string, AbortController>()
  const queue: DatePage[] = []
  let generation = 0
  let active = 0
  let tick = 0
  let summaryRequest: AbortController | undefined
  let disposed = false

  function pagesFor(day: string) {
    if (!pages.has(day)) pages.set(day, [makePage(day, null, 0)])
    return pages.get(day)!
  }
  function makePage(day: string, cursor: string | null, index: number): DatePage {
    return reactive({
      key: `${day}:${index}`,
      day,
      cursor,
      items: undefined,
      loading: false,
      error: false,
      visible: false,
      touched: 0,
    })
  }
  function trim() {
    const cached = [...pages.values()].flat().filter((p) => p.items)
    const candidates = cached.filter((p) => !p.visible).sort((a, b) => a.touched - b.touched)
    // Visible pages are active working data; only ordinary cached pages may be evicted.
    let count = cached.length
    for (const page of candidates) {
      if (count <= 20) break
      page.items = undefined
      count--
    }
  }
  function enqueue(page: DatePage, retry = false) {
    if (disposed || page.items || page.loading || (page.error && !retry) || queue.includes(page))
      return
    page.error = false
    queue.push(page)
    pump()
  }
  function pump() {
    while (!disposed && active < 3 && queue.length) {
      const page = queue.shift()!
      if (!page.visible) continue
      void fetchPage(page)
    }
  }
  async function fetchPage(page: DatePage) {
    const current = generation
    const controller = new AbortController()
    requests.set(page.key, controller)
    active++
    page.loading = true
    try {
      const data = await call<{ items: Photo[]; nextCursor: string | null }>({
        url: '/photo/getByDate',
        params: { day: page.day, order: order(), limit: 100, cursor: page.cursor || undefined },
        signal: controller.signal,
      })
      if (current !== generation || controller.signal.aborted) return
      page.items = [...new Map(data.items.map((p) => [p.photoId, p])).values()]
      page.touched = ++tick
      const siblings = pages.get(page.day)!
      const index = siblings.indexOf(page)
      if (data.nextCursor && !siblings[index + 1]) {
        siblings.push(makePage(page.day, data.nextCursor, index + 1))
      }
      trim()
    } catch {
      if (current === generation && !controller.signal.aborted) page.error = true
    } finally {
      page.loading = false
      if (requests.get(page.key) === controller) requests.delete(page.key)
      active--
      // A collapsed day can be reopened before its aborted request settles.
      if (current === generation && controller.signal.aborted && page.visible) enqueue(page)
      pump()
    }
  }
  function setVisible(page: DatePage, visible: boolean) {
    page.visible = visible
    if (visible) {
      page.touched = ++tick
      enqueue(page)
    } else trim()
  }
  function cancelDay(day: string) {
    for (const page of pages.get(day) ?? []) {
      page.visible = false
      requests.get(page.key)?.abort()
      const index = queue.indexOf(page)
      if (index !== -1) queue.splice(index, 1)
    }
    trim()
  }
  function clear() {
    generation++
    summaryRequest?.abort()
    for (const controller of requests.values()) controller.abort()
    requests.clear()
    queue.length = 0
    pages.clear()
    summary.value = undefined
    error.value = false
    loading.value = false
  }
  async function refresh(enabled = true) {
    clear()
    if (!enabled || disposed) return
    const current = generation
    const controller = new AbortController()
    summaryRequest = controller
    loading.value = true
    try {
      const data = await call<DateSummary>({
        url: '/photo/getDateSummary',
        signal: controller.signal,
      })
      if (current === generation) summary.value = data
    } catch {
      if (current === generation && !controller.signal.aborted) error.value = true
    } finally {
      if (current === generation) loading.value = false
    }
  }
  const photos = computed(() =>
    [...pages.entries()]
      .sort(([a], [b]) =>
        a === 'undated'
          ? 1
          : b === 'undated'
            ? -1
            : order() === 'latest'
              ? b.localeCompare(a)
              : a.localeCompare(b),
      )
      .flatMap(([, entries]) => entries.flatMap((p) => p.items ?? [])),
  )

  onBeforeUnmount(() => {
    disposed = true
    clear()
  })
  return {
    summary,
    loading,
    error,
    pagesFor,
    setVisible,
    cancelDay,
    refresh,
    photos,
    retry: (p: DatePage) => enqueue(p, true),
  }
}
