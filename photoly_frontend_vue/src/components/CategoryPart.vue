<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Video,
  RectangleHorizontal,
  CircleDot,
  Clapperboard,
  ScanLine,
  UserRound,
  ChevronLeft,
} from '@lucide/vue'
import { call } from '@/lib/http'
import { mediaCategories, type MediaCategory } from '@/lib/mediaCategories'
import type { Photo } from '@/composables/usePhotos'
import MediaList from './MediaList.vue'
import MediaThumbnail from './MediaThumbnail.vue'
import PageHeader from './PageHeader.vue'
import { Button } from '@/components/ui/button'
const icons = {
  video: Video,
  panorama: RectangleHorizontal,
  live: CircleDot,
  animated: Clapperboard,
  screenshot: ScanLine,
  portrait: UserRound,
}
const previews = ref<Partial<Record<MediaCategory, Photo[]>>>({})
const route = useRoute(),
  router = useRouter()
const category = computed(() => mediaCategories.find((c) => c.id === route.query.category))
const counts = ref<Record<string, number>>({}),
  pending = ref(0),
  error = ref(''),
  loading = ref(false)
const media = ref<InstanceType<typeof MediaList>>(),
  photos = ref<Photo[]>([]),
  after = ref<number | null>(null)
const page = ref<HTMLElement>()
const positions = new Map<string, number>()
let sequence = 0
function scrollContainer() {
  return page.value?.closest('main')
}

async function load(more = false) {
  const seq = ++sequence
  loading.value = true
  error.value = ''
  try {
    if (category.value) {
      const result = await call<{ items: Photo[]; nextAfter: number | null }>({
        url: `/photo/categories/${category.value.id}/photos`,
        params: { after: more ? after.value : 0 },
      })
      if (seq !== sequence) return
      photos.value = more ? [...photos.value, ...result.items] : result.items
      after.value = result.nextAfter
      await nextTick()
      media.value?.updatePhotoList(photos.value)
    } else {
      const result = await call<{
        items: { category: MediaCategory; count: number; previews: Photo[] }[]
        pending: number
      }>({ url: '/photo/categories' })
      if (seq !== sequence) return
      counts.value = Object.fromEntries(result.items.map((c) => [c.category, c.count]))
      previews.value = Object.fromEntries(result.items.map((c) => [c.category, c.previews ?? []]))
      pending.value = result.pending
    }
  } catch {
    if (seq === sequence) error.value = 'Could not load categories. Please retry.'
  } finally {
    if (seq === sequence) loading.value = false
  }
}
watch(
  category,
  async (_, previous) => {
    const scroller = scrollContainer()
    if (scroller) positions.set(previous?.id ?? 'overview', scroller.scrollTop)
    photos.value = []
    after.value = null
    await load()
    await nextTick()
    scrollContainer()?.scrollTo({ top: positions.get(category.value?.id ?? 'overview') ?? 0 })
  },
  { immediate: true },
)
function visit(id?: string) {
  void router.push({ path: '/home/CategoryPart', query: id ? { category: id } : {} })
}
</script>
<template>
  <div ref="page" class="flex flex-col gap-6 p-4 md:px-10 md:py-8">
    <Button v-if="category" variant="ghost" class="self-start" @click="visit()"
      ><ChevronLeft />Category</Button
    >
    <PageHeader
      :title="category?.name ?? 'Category'"
      :description="category?.description ?? 'Browse your library by media type.'"
    >
      <Button variant="outline" :disabled="loading" @click="load()">Refresh</Button>
    </PageHeader>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="!category && pending" class="text-muted-foreground text-sm">
      Classifying {{ pending.toLocaleString() }} media items in the background. Unavailable files
      will be retried.
    </p>
    <div
      v-if="!category"
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-3 2xl:grid-cols-[repeat(auto-fill,minmax(280px,1fr))]"
    >
      <button
        v-for="item in mediaCategories"
        :key="item.id"
        type="button"
        class="group bg-card hover:border-primary/40 overflow-hidden rounded-[14px] border shadow-xs text-left transition-all duration-200 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        :aria-label="`${item.name} ${(counts[item.id] ?? 0).toLocaleString()} items`"
        :aria-busy="loading"
        @click="visit(item.id)"
      >
        <div
          class="relative grid aspect-[16/10] grid-cols-[3fr_2fr] grid-rows-2 gap-0.5 overflow-hidden bg-muted/25"
          aria-hidden="true"
        >
          <div
            v-if="loading && !Object.keys(counts).length"
            class="absolute inset-0 animate-pulse bg-muted motion-reduce:animate-none"
          />
          <component
            v-else-if="!previews[item.id]?.length"
            :is="icons[item.id]"
            class="absolute top-1/2 left-1/2 size-12 -translate-x-1/2 -translate-y-1/2 text-primary stroke-[1.5]"
          />
          <MediaThumbnail
            v-for="(cover, i) in (previews[item.id] ?? []).slice(0, 3)"
            :key="cover.photoId"
            :photo-id="cover.photoId"
            :format="cover.format"
            :content-version="cover.contentVersion"
            :class="[
              'size-full',
              i === 0 && 'row-span-2',
              previews[item.id]?.length === 1 && 'col-span-2',
            ]"
          />
        </div>
        <div class="px-4 py-3.5">
          <div class="flex items-center gap-2.5 font-semibold">
            <component
              :is="icons[item.id]"
              class="text-muted-foreground size-5"
              aria-hidden="true"
            />{{ item.name }}
          </div>
          <p class="text-muted-foreground mt-1 text-xs">
            {{
              Object.keys(counts).length
                ? (counts[item.id] ?? 0).toLocaleString() + ' items'
                : loading
                  ? 'Loading…'
                  : '— items'
            }}
          </p>
        </div>
      </button>
    </div>
    <MediaList
      v-else
      ref="media"
      :ns-or-not="false"
      :context="category.name"
      :heading="category.name"
      :group-by-day="false"
      @query-photos="load()"
    />
    <Button
      v-if="category && after"
      variant="outline"
      class="self-center"
      :disabled="loading"
      @click="load(true)"
      >Load more</Button
    >
    <p v-if="loading" role="status" class="text-muted-foreground text-sm">Loading…</p>
  </div>
</template>
