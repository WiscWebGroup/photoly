<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { aiCall, aiMessage, AiError, type AiCapabilities, type AiSearchResponse } from '@/lib/ai'
import { photoUrl } from '@/lib/photoUrl'
import type { Photo } from '@/composables/usePhotos'
import { useAuthStore } from '@/stores/auth'

export interface RecommendationSnapshot {
  result: AiSearchResponse | null
  cursor: string | null
  hasMore: boolean
  expiresAt: number
}
interface PageResult extends AiSearchResponse {
  nextCursor: string | null
  hasMore: boolean
}
const props = defineProps<{
  photo: Photo
  scrollRoot: HTMLElement | null
  resume?: RecommendationSnapshot
}>()
const emit = defineEmits<{ navigate: [photo: Photo]; invalidated: [] }>()
const auth = useAuthStore()
const restored = props.resume && props.resume.expiresAt > Date.now() ? props.resume : undefined
const result = ref<AiSearchResponse | null>(restored?.result ?? null)
const cursor = ref<string | null>(restored?.cursor ?? null)
const hasMore = ref(restored?.hasMore ?? true)
const expiresAt = ref(restored?.expiresAt ?? 0)
const caps = ref<AiCapabilities | null>(null)
const busy = ref(false)
const error = ref('')
const restartRequired = ref(false)
const sentinel = ref<HTMLElement>()
let sequence = 0
let controller: AbortController | undefined
let capsController: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let scrolled = false
let userIntent = false
let lastTriggeredScroll = -1
let observer: IntersectionObserver | undefined
const supported = computed(() =>
  caps.value?.formats.includes((props.photo.format ?? '').toLowerCase()),
)
const allowed = computed(() => caps.value?.ready && supported.value)
const unavailable = computed(() =>
  caps.value
    ? !supported.value
      ? 'This image format is not supported for similar images.'
      : aiMessage(caps.value.reason ?? 'model_not_ready')
    : '',
)

function cancel() {
  sequence++
  controller?.abort()
  busy.value = false
}
function invalidate() {
  cancel()
  result.value = null
  cursor.value = null
  hasMore.value = true
  expiresAt.value = 0
  scrolled = false
  emit('invalidated')
}
function capture(): RecommendationSnapshot {
  return {
    result: result.value,
    cursor: cursor.value,
    hasMore: hasMore.value,
    expiresAt: expiresAt.value,
  }
}
defineExpose({ capture, invalidate })
async function load(restart = false) {
  if (busy.value || !allowed.value || (!hasMore.value && !restart)) return
  if (restart) {
    invalidate()
    error.value = ''
    restartRequired.value = false
  }
  if (result.value && expiresAt.value <= Date.now()) {
    error.value = 'These recommendations expired. Reload to continue.'
    restartRequired.value = true
    return
  }
  const restoreFocus = sentinel.value?.contains(document.activeElement)
  userIntent = false
  scrolled = false
  const request = ++sequence,
    token = auth.token
  controller = new AbortController()
  busy.value = true
  error.value = ''
  const first = !result.value
  try {
    const page = await aiCall<PageResult>({
      method: 'post',
      url: `/ai/photos/${props.photo.photoId}/similar/page`,
      data: first ? { pageSize: 24 } : { cursor: cursor.value },
      signal: controller.signal,
    })
    if (disposed || request !== sequence || token !== auth.token) return
    if (result.value && result.value.profileId !== page.profileId) {
      invalidate()
      error.value = 'The active model changed. Reload recommendations.'
      restartRequired.value = true
      return
    }
    if (
      result.value &&
      (result.value.similarityThreshold ?? null) !== (page.similarityThreshold ?? null)
    ) {
      invalidate()
      error.value = aiMessage('search_settings_changed')
      restartRequired.value = true
      return
    }
    const ids = new Set(result.value?.items.map((x) => x.photo.photoId) ?? [])
    result.value = {
      ...page,
      items: [
        ...(result.value?.items ?? []),
        ...page.items.filter((x) => !ids.has(x.photo.photoId)),
      ],
    }
    cursor.value = page.nextCursor
    hasMore.value = page.hasMore
    if (first) expiresAt.value = Date.now() + 14 * 60 * 1000
  } catch (e) {
    if (request !== sequence || disposed || token !== auth.token) return
    error.value = aiMessage(e)
    restartRequired.value =
      e instanceof AiError &&
      [
        'cursor_expired',
        'invalid_cursor',
        'profile_changed',
        'search_settings_changed',
        'stale_content',
        'photo_not_found',
      ].includes(e.code)
    if (
      e instanceof AiError &&
      [
        'ai_not_enabled',
        'profile_changed',
        'search_settings_changed',
        'stale_content',
        'photo_not_found',
      ].includes(e.code)
    )
      invalidate()
  } finally {
    if (request === sequence) {
      busy.value = false
      await nextTick()
      if (restoreFocus && document.activeElement === document.body)
        sentinel.value?.focus({ preventScroll: true })
    }
  }
}
function maybeLoad() {
  const root = props.scrollRoot,
    target = sentinel.value
  if (
    !root ||
    !target ||
    !scrolled ||
    !allowed.value ||
    busy.value ||
    error.value ||
    !hasMore.value
  )
    return
  const scroll = root.scrollTop
  if (scroll <= lastTriggeredScroll + 8) return
  const bounds = target.getBoundingClientRect(),
    viewport = root.getBoundingClientRect()
  if (bounds.top <= viewport.bottom + (result.value ? 160 : 0) && bounds.bottom >= viewport.top) {
    lastTriggeredScroll = scroll
    void load()
  }
}
function intent() {
  userIntent = true
}
function onScroll() {
  if (!userIntent) return
  if ((props.scrollRoot?.scrollTop ?? 0) > 48) scrolled = true
  maybeLoad()
}
watch(
  () => props.scrollRoot,
  (root, old) => {
    old?.removeEventListener('scroll', onScroll)
    for (const event of ['wheel', 'touchmove', 'keydown', 'pointerdown'])
      old?.removeEventListener(event, intent)
    observer?.disconnect()
    if (!root) return
    root.addEventListener('scroll', onScroll, { passive: true })
    for (const event of ['wheel', 'touchmove', 'keydown', 'pointerdown'])
      root.addEventListener(event, intent, { passive: true })
    if (typeof IntersectionObserver !== 'undefined') {
      observer = new IntersectionObserver(maybeLoad, { root, rootMargin: '160px' })
      if (sentinel.value) observer.observe(sentinel.value)
    }
  },
  { immediate: true, flush: 'post' },
)
async function refresh() {
  const token = auth.token
  capsController?.abort()
  capsController = new AbortController()
  try {
    const value = await aiCall<AiCapabilities>({
      url: '/ai/capabilities',
      timeout: 8000,
      signal: capsController.signal,
    })
    if (disposed || token !== auth.token) return
    if (!value.ready || (result.value && result.value.profileId !== value.profileId)) invalidate()
    if (
      result.value &&
      (result.value.similarityThreshold ?? null) !== (value.similarityThreshold ?? null)
    ) {
      invalidate()
      error.value = aiMessage('search_settings_changed')
      restartRequired.value = true
    }
    caps.value = value
    await nextTick()
    maybeLoad()
  } catch {
    if (!disposed && token === auth.token) {
      caps.value = null
      error.value = 'AI availability could not be checked. Retry.'
    }
  }
  if (!disposed) timer = setTimeout(refresh, document.hidden ? 30000 : 15000)
}
function retry() {
  if (!caps.value) {
    clearTimeout(timer)
    error.value = ''
    void refresh()
  } else void load(restartRequired.value)
}
watch(
  () => auth.token,
  () => {
    invalidate()
    caps.value = null
    clearTimeout(timer)
    void refresh()
  },
)
void refresh()
onBeforeUnmount(() => {
  disposed = true
  cancel()
  capsController?.abort()
  clearTimeout(timer)
  observer?.disconnect()
  props.scrollRoot?.removeEventListener('scroll', onScroll)
  for (const event of ['wheel', 'touchmove', 'keydown', 'pointerdown'])
    props.scrollRoot?.removeEventListener(event, intent)
})
</script>

<template>
  <section aria-label="Similar images" class="mt-6 border-t pt-6">
    <div class="flex min-h-28 flex-col items-center justify-center gap-2 text-center">
      <h2 class="text-lg font-semibold">Similar images</h2>
      <p class="text-muted-foreground text-sm">Scroll down to discover similar images</p>
      <span aria-hidden="true">↓</span>
    </div>
    <p v-if="caps && !allowed" role="status" class="py-4 text-sm text-muted-foreground">
      {{ unavailable }}
    </p>
    <div v-if="result" class="mb-4 text-sm text-muted-foreground" role="status">
      <p>{{ result.items.length }} similar image(s) · Profile #{{ result.profileId }}</p>
      <p>
        Indexed {{ result.coverage.ready }} / {{ result.coverage.supported }} supported images.
        Ordered by similarity.
      </p>
      <p v-if="result.similarityThreshold != null">
        Minimum similarity: {{ result.similarityThreshold }}
      </p>
      <p v-if="result.coverage.pending || result.coverage.failed">
        Coverage is partial. Reload later to include newly indexed images.
      </p>
    </div>
    <ul
      class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
      aria-label="Similar image results"
    >
      <li v-for="hit in result?.items ?? []" :key="hit.photo.photoId">
        <button
          type="button"
          class="w-full overflow-hidden rounded-lg border text-left focus-visible:ring-2"
          :aria-label="`View ${hit.photo.photoName}.${hit.photo.format}`"
          @click="emit('navigate', hit.photo)"
        >
          <img
            :src="photoUrl(hit.photo.photoId, 'renderThumbnail')"
            :alt="hit.photo.photoName"
            loading="lazy"
            decoding="async"
            class="aspect-square w-full object-cover"
          />
          <span class="block truncate p-2 text-sm"
            >{{ hit.photo.photoName }}.{{ hit.photo.format }}</span
          >
        </button>
      </li>
    </ul>
    <div
      tabindex="-1"
      ref="sentinel"
      class="flex min-h-24 flex-col items-center justify-center gap-3 py-4"
    >
      <div v-if="busy" role="status" class="w-full">
        <p class="mb-3 text-sm">Loading similar images…</p>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <Skeleton v-for="n in 5" :key="n" class="aspect-square" />
        </div>
      </div>
      <template v-else-if="error"
        ><p role="alert" class="text-sm">{{ error }}</p>
        <Button variant="outline" @click="retry">{{
          restartRequired ? 'Reload recommendations' : 'Retry'
        }}</Button></template
      >
      <template v-else-if="allowed"
        ><Button v-if="hasMore" variant="outline" @click="load()">Load more</Button>
        <p v-else class="text-muted-foreground text-sm">No more similar images</p></template
      >
    </div>
  </section>
</template>
