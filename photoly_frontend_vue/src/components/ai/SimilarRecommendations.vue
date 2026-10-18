<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { aiCall, aiMessage, AiError, type AiCapabilities, type AiSearchResponse } from '@/lib/ai'
import { photoUrl } from '@/lib/photoUrl'
import { mediaKind } from '@/lib/media'
import videoIcon from '@/assets/icons/Video.png'
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
const props = defineProps<{ photo: Photo; resume?: RecommendationSnapshot }>()
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
const supported = computed(() =>
  caps.value?.formats.includes((props.photo.format ?? '').toLowerCase()),
)
const allowed = computed(() => caps.value?.ready && supported.value)
const unavailable = computed(() =>
  caps.value
    ? !supported.value
      ? 'This media format is not supported for similar photos and videos.'
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
  emit('invalidated')
}
function collapse() {
  cancel()
  result.value = null
  cursor.value = null
  hasMore.value = true
  expiresAt.value = 0
  error.value = ''
  restartRequired.value = false
}
const expanded = computed(() => busy.value || !!result.value || !!error.value)
function capture(): RecommendationSnapshot {
  return {
    result: result.value,
    cursor: cursor.value,
    hasMore: hasMore.value,
    expiresAt: expiresAt.value,
  }
}
function start() {
  if (!result.value && !busy.value) void load()
}
defineExpose({ capture, cancel, invalidate, collapse, start, expanded, allowed, unavailable })
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
})
</script>

<template>
  <section
    v-if="expanded"
    aria-label="Similar photos and videos"
    class="mt-6 space-y-3 border-t pt-5"
  >
    <div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <h2 class="text-lg font-semibold">Similar photos and videos</h2>
      <p v-if="result" class="text-sm text-muted-foreground" role="status">
        {{ result.items.length }} found, most similar first
      </p>
    </div>
    <p
      v-if="result?.coverage.pending || result?.coverage.failed"
      class="text-sm text-muted-foreground"
    >
      Some photos and videos aren't indexed yet ({{ result.coverage.ready }} of
      {{ result.coverage.supported }}). Reload later to include them.
    </p>
    <ul
      class="grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-6"
      aria-label="Similar photos and videos results"
    >
      <li v-for="hit in result?.items ?? []" :key="hit.photo.photoId">
        <button
          type="button"
          class="group w-full overflow-hidden rounded-md border text-left transition-colors hover:border-primary/50 hover:bg-accent focus-visible:ring-2"
          :aria-label="`View ${hit.photo.photoName}.${hit.photo.format}`"
          @click="emit('navigate', hit.photo)"
        >
          <img
            :src="
              mediaKind(hit.photo.format) === 'video'
                ? videoIcon
                : photoUrl(hit.photo.photoId, 'renderThumbnail')
            "
            :alt="hit.photo.photoName"
            loading="lazy"
            decoding="async"
            class="aspect-square w-full object-cover transition-transform group-hover:scale-105"
          />
          <span class="block truncate px-2 py-1 text-xs"
            >{{ hit.photo.photoName }}.{{ hit.photo.format }}</span
          >
        </button>
      </li>
    </ul>
    <div tabindex="-1" ref="sentinel" class="flex flex-col items-center gap-3">
      <div v-if="busy" role="status" class="w-full">
        <p class="mb-2 text-sm">Loading similar photos and videos…</p>
        <div class="grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-6">
          <Skeleton v-for="n in 6" :key="n" class="aspect-square" />
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
        <p v-else class="text-sm text-muted-foreground">
          No more similar photos and videos
        </p></template
      >
    </div>
  </section>
</template>
