<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MediaList from '@/components/MediaList.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Button } from '@/components/ui/button'
import { call } from '@/lib/http'
import type { Photo } from '@/composables/usePhotos'
import { useAuthStore } from '@/stores/auth'
import { useUploadsStore } from '@/stores/uploads'

const auth = useAuthStore()
const uploads = useUploadsStore()
const route = useRoute()
const router = useRouter()
const order = computed(() => (route.query.order === 'oldest' ? 'oldest' : 'latest'))
const media = ref<InstanceType<typeof MediaList>>()
const items = ref<Photo[]>([])
const cursor = ref<string | null>(null)
const loading = ref(false)
const error = ref(false)
let generation = 0
let request: AbortController | undefined

async function load(reset = false) {
  if (!reset && loading.value) return
  if (reset) {
    request?.abort()
    generation++
    loading.value = false
    items.value = []
    cursor.value = null
    media.value?.updatePhotoList([])
  }
  const current = generation
  if (!auth.token) return
  request = new AbortController()
  loading.value = true
  error.value = false
  try {
    const data = await call<{ items: Photo[]; nextCursor: string | null }>({
      url: '/photo/getByDate',
      params: { order: order.value, limit: 100, cursor: cursor.value || undefined },
      signal: request.signal,
    })
    if (current !== generation) return
    const seen = new Set(items.value.map((p) => p.photoId))
    items.value = [...items.value, ...data.items.filter((p) => !seen.has(p.photoId))]
    cursor.value = data.nextCursor
    await nextTick()
    if (current === generation) media.value?.updatePhotoList(items.value)
  } catch {
    if (current === generation) error.value = true
  } finally {
    if (current === generation) loading.value = false
  }
}
function setOrder(value: string) {
  router.replace({ query: { ...route.query, order: value } })
}
watch([order, () => auth.token, () => uploads.revision], () => load(true), { immediate: true })
onBeforeUnmount(() => {
  generation++
  request?.abort()
})
</script>

<template>
  <div class="flex flex-col gap-6 p-4 pb-24 md:p-10">
    <PageHeader
      title="Dates"
      description="All your photos, by capture date. Upload date is used when capture date is unavailable."
    >
      <div class="flex gap-1" role="group" aria-label="Date order">
        <Button
          :variant="order === 'latest' ? 'default' : 'outline'"
          :aria-pressed="order === 'latest'"
          @click="setOrder('latest')"
          >Latest</Button
        >
        <Button
          :variant="order === 'oldest' ? 'default' : 'outline'"
          :aria-pressed="order === 'oldest'"
          @click="setOrder('oldest')"
          >Oldest</Button
        >
      </div>
    </PageHeader>
    <MediaList
      ref="media"
      :ns-or-not="false"
      group-by-day
      timeline
      context="Dates"
      heading="Photos"
      empty-title="No photos yet"
      empty-description="Uploaded photos from all your folders will appear here."
      @query-photos="load(true)"
    />
    <p v-if="loading" role="status" class="text-muted-foreground text-sm">Loading photos…</p>
    <div v-else-if="error" role="alert" class="flex items-center gap-3">
      Could not load photos. <Button variant="outline" @click="load()">Retry</Button>
    </div>
    <Button v-else-if="cursor" variant="outline" class="self-center" @click="load()"
      >Load more photos</Button
    >
  </div>
</template>
