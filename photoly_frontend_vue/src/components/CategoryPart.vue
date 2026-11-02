<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Grid3X3, ChevronLeft } from '@lucide/vue'
import { call } from '@/lib/http'
import { mediaCategories, type MediaCategory } from '@/lib/mediaCategories'
import type { Photo } from '@/composables/usePhotos'
import MediaList from './MediaList.vue'
import PageHeader from './PageHeader.vue'
import { Button } from '@/components/ui/button'
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
let sequence = 0
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
        items: { category: MediaCategory; count: number }[]
        pending: number
      }>({ url: '/photo/categories' })
      if (seq !== sequence) return
      counts.value = Object.fromEntries(result.items.map((c) => [c.category, c.count]))
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
  () => {
    photos.value = []
    after.value = null
    void load()
  },
  { immediate: true },
)
function visit(id?: string) {
  void router.push({ path: '/home/CategoryPart', query: id ? { category: id } : {} })
}
</script>
<template>
  <div class="flex flex-col gap-6 p-4 md:p-8">
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
    <div v-if="!category" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <button
        v-for="item in mediaCategories"
        :key="item.id"
        class="bg-card hover:bg-accent flex flex-col items-start gap-3 rounded-xl border p-6 text-left"
        @click="visit(item.id)"
      >
        <Grid3X3 class="text-muted-foreground size-6" /><span class="font-semibold">{{
          item.name
        }}</span>
        <span class="text-muted-foreground text-sm"
          >{{ (counts[item.id] ?? 0).toLocaleString() }} items</span
        >
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
