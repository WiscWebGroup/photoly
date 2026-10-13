<script setup lang="ts">
import { ref } from 'vue'
import { toast } from 'vue-sonner'
import MediaList from '@/components/MediaList.vue'
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
import { ApiError, call } from '@/lib/http'

const searchTypes = [
  { value: 'photoName', label: 'Photo Name' },
  { value: 'tag', label: 'Tag' },
  { value: 'gallery', label: 'Gallery' },
  { value: 'namespace', label: 'Namespace' },
]

const mediaList = ref<InstanceType<typeof MediaList>>()
const searchType = ref('photoName')
const query = ref('')

async function queryPhotos() {
  if (!query.value.trim()) return mediaList.value?.updatePhotoList([])
  try {
    const photos = await call<Photo[]>({
      url: '/photo/searchPhoto',
      params: { option: searchType.value, query: query.value },
    })
    mediaList.value?.updatePhotoList(photos)
    toast.success(`${photos.length} record(s) found`)
  } catch (e) {
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
        <CardDescription>The search utilizes partial or full match.</CardDescription>
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
          v-model="query"
          class="min-w-40 flex-1"
          aria-label="Search query"
          @keyup.enter="queryPhotos"
        />
        <Button @click="queryPhotos">Search</Button>
      </CardContent>
    </Card>

    <MediaList
      ref="mediaList"
      :ns-or-not="false"
      :detach-enabled="false"
      @query-photos="queryPhotos"
    />
  </div>
</template>
