<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import MediaList from '@/components/MediaList.vue'
import AiSearchPanel from '@/components/ai/AiSearchPanel.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { Photo } from '@/composables/usePhotos'
import { useAuthStore } from '@/stores/auth'
import { useSmartSearch } from '@/composables/useSmartSearch'
import { ApiError, call } from '@/lib/http'

const searchTypes = [
  { value: 'smart', label: 'Smart Search' },
  { value: 'photoName', label: 'Photo Name' },
  { value: 'tag', label: 'Tag' },
  { value: 'gallery', label: 'Gallery' },
  { value: 'namespace', label: 'Namespace' },
  { value: 'text', label: 'Natural language' },
]

const mediaList = ref<InstanceType<typeof MediaList>>()
const searchType = ref('smart')
const query = ref('')
const smart = useSmartSearch()
const auth = useAuthStore()
let fieldSequence = 0
let fieldController: AbortController | undefined
function resetSearch() {
  fieldSequence++
  fieldController?.abort()
  smart.clear()
  mediaList.value?.updatePhotoList([])
}
watch(() => auth.token, resetSearch)
onBeforeUnmount(resetSearch)
watch(
  smart.results,
  (photos) => {
    if (searchType.value === 'smart') mediaList.value?.updatePhotoList(photos)
  },
  { flush: 'post' },
)
watch(searchType, resetSearch)

async function queryPhotos() {
  if (searchType.value === 'smart') return smart.search(query.value)
  const id = ++fieldSequence
  fieldController?.abort()
  fieldController = new AbortController()
  mediaList.value?.updatePhotoList([])
  if (!query.value.trim()) return
  try {
    const photos = await call<Photo[]>({
      url: '/photo/searchPhoto',
      signal: fieldController.signal,
      params: { option: searchType.value, query: query.value },
    })
    if (id !== fieldSequence) return
    mediaList.value?.updatePhotoList(photos)
    toast.success(`${photos.length} record(s) found`)
  } catch (e) {
    if (id !== fieldSequence) return
    if (e instanceof ApiError && e.msgCode === 404) toast.info('No record found')
    else toast.error('query photo Response Error!')
  }
}
</script>

<template>
  <div class="space-y-4" @contextmenu.prevent.stop>
    <h2 class="text-2xl font-semibold">Search</h2>
    <Card class="max-w-3xl">
      <CardHeader>
        <CardTitle>Query Information</CardTitle>
        <CardDescription
          >Search names, tags, galleries and image content together, or choose a specific search
          mode.</CardDescription
        >
      </CardHeader>
      <CardContent class="flex flex-wrap gap-2">
        <Select v-model="searchType">
          <SelectTrigger class="w-40" aria-label="Search by"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="t in searchTypes" :key="t.value" :value="t.value">{{
              t.label
            }}</SelectItem>
          </SelectContent>
        </Select>
        <Input
          v-if="searchType !== 'text'"
          v-model="query"
          class="min-w-40 flex-1"
          :maxlength="searchType === 'smart' ? 512 : undefined"
          :placeholder="searchType === 'smart' ? 'Search your photos…' : undefined"
          aria-label="Search query"
          @keyup.enter="queryPhotos"
        />
        <Button v-if="searchType !== 'text'" @click="queryPhotos">Search</Button>
      </CardContent>
    </Card>

    <div v-if="searchType === 'smart'" aria-live="polite" class="space-y-1 text-sm">
      <p v-if="smart.error.value" role="alert" class="text-destructive">{{ smart.error.value }}</p>
      <p v-if="smart.busy.value" role="status">
        {{
          smart.semanticBusy.value
            ? 'Searching… Results will update when semantic search finishes.'
            : 'Searching…'
        }}
      </p>
      <p v-if="smart.textNotice.value" role="status">{{ smart.textNotice.value }}</p>
      <p v-if="smart.semanticNotice.value" role="status">{{ smart.semanticNotice.value }}</p>
      <p
        v-if="smart.submitted.value && (!smart.busy.value || smart.results.value.length)"
        role="status"
      >
        {{
          smart.limited.value
            ? 'Showing top 100 results'
            : `${smart.results.value.length} result(s) found`
        }}
      </p>
    </div>
    <AiSearchPanel v-if="searchType === 'text'" :mode="searchType" />
    <MediaList
      v-else
      ref="mediaList"
      :ns-or-not="false"
      :detach-enabled="false"
      @query-photos="queryPhotos"
    />
  </div>
</template>
