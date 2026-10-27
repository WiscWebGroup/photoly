<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

interface CountedName {
  name: string
  count: number
}
interface Details {
  nsId: number
  nsName: string
  photoCount: number
  videoCount: number
  folderCount: number
  originalBytes: number
  thumbnailBytes: number
  totalBytes: number
  complete: boolean
  missingFileCount: number
  places: CountedName[]
  cameras: CountedName[]
  lenses: CountedName[]
}
const props = defineProps<{ folder: { nsId: number; nsName: string } | null }>()
const emit = defineEmits<{ close: [] }>()
const auth = useAuthStore()
const details = ref<Details | null>(null)
const loading = ref(false)
const error = ref(false)
const pages = ref({ places: 0, cameras: 0, lenses: 0 })
const sections = computed(() =>
  (['places', 'cameras', 'lenses'] as const).map((key) => ({
    key,
    title: { places: 'Nearby places', cameras: 'Cameras', lenses: 'Lenses' }[key],
    all: details.value?.[key] ?? [],
    items: details.value?.[key].slice(pages.value[key] * 10, (pages.value[key] + 1) * 10) ?? [],
  })),
)
let version = 0
let request: AbortController | undefined
async function load() {
  const current = ++version
  request?.abort()
  details.value = null
  error.value = false
  loading.value = false
  pages.value = { places: 0, cameras: 0, lenses: 0 }
  if (!props.folder || !auth.token) return
  loading.value = true
  request = new AbortController()
  try {
    const data = await call<Details>({
      url: '/namespace/getDetails',
      params: { nsId: props.folder.nsId },
      signal: request.signal,
    })
    if (current === version) details.value = data
  } catch {
    if (current === version) error.value = true
  } finally {
    if (current === version) loading.value = false
  }
}
function bytes(value: number) {
  if (value < 1000) return `${value} B`
  const unit = Math.min(Math.floor(Math.log10(value) / 3), 4)
  return `${(value / 1000 ** unit).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${['B', 'kB', 'MB', 'GB', 'TB'][unit]}`
}
watch([() => props.folder?.nsId, () => auth.token], load, { immediate: true })
onBeforeUnmount(() => {
  version++
  request?.abort()
})
</script>

<template>
  <Dialog :open="!!folder" @update:open="(value) => !value && emit('close')">
    <DialogContent class="max-h-[85dvh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle class="break-all"
          >{{ folder?.nsName === '/' ? 'Library' : folder?.nsName }} — Details</DialogTitle
        >
        <DialogDescription
          >Includes all nested folders. Device and place counts refer to photos.</DialogDescription
        >
      </DialogHeader>
      <p v-if="loading" role="status">Loading folder details…</p>
      <div v-else-if="error" role="alert">
        Could not load folder details. The folder may have been removed.
        <Button variant="outline" class="mt-2" @click="load">Retry</Button>
      </div>
      <template v-else-if="details">
        <dl class="grid grid-cols-3 gap-4 rounded-lg border p-4">
          <div>
            <dt class="text-muted-foreground text-sm">Photos</dt>
            <dd class="text-xl font-semibold">{{ details.photoCount }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground text-sm">Videos</dt>
            <dd class="text-xl font-semibold">{{ details.videoCount }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground text-sm">Subfolders</dt>
            <dd class="text-xl font-semibold">{{ details.folderCount }}</dd>
          </div>
        </dl>
        <dl class="grid grid-cols-2 gap-2 text-sm">
          <dt>Original files</dt>
          <dd class="text-right" :title="`${details.originalBytes} bytes`">
            {{ bytes(details.originalBytes) }}
          </dd>
          <dt>Thumbnails</dt>
          <dd class="text-right" :title="`${details.thumbnailBytes} bytes`">
            {{ bytes(details.thumbnailBytes) }}
          </dd>
          <dt class="font-semibold">Total storage</dt>
          <dd class="text-right font-semibold" :title="`${details.totalBytes} bytes`">
            {{ bytes(details.totalBytes) }}
          </dd>
        </dl>
        <p v-if="!details.complete" role="status" class="text-amber-700 dark:text-amber-400">
          Storage totals are incomplete: {{ details.missingFileCount }} expected files are missing
          or unavailable.
        </p>
        <section
          v-for="section in sections"
          :key="section.key"
          class="border-t pt-3"
          :aria-label="section.title"
        >
          <h3 class="font-semibold">{{ section.title }}</h3>
          <p v-if="!section.all.length" class="text-muted-foreground mt-2 text-sm">
            No information available.
          </p>
          <ul v-else class="mt-2 space-y-2 text-sm">
            <li v-for="item in section.items" :key="item.name" class="flex justify-between gap-4">
              <span class="break-words">{{ item.name }}</span
              ><span class="shrink-0 text-muted-foreground"
                >{{ item.count }} {{ item.count === 1 ? 'photo' : 'photos' }}</span
              >
            </li>
          </ul>
          <div v-if="section.all.length > 10" class="mt-2 flex items-center justify-between gap-2">
            <Button
              variant="outline"
              size="sm"
              :disabled="pages[section.key] === 0"
              :aria-label="`Previous ${section.title}`"
              @click="pages[section.key]--"
              >Previous</Button
            >
            <span class="text-xs"
              >{{ pages[section.key] + 1 }} / {{ Math.ceil(section.all.length / 10) }}</span
            >
            <Button
              variant="outline"
              size="sm"
              :disabled="(pages[section.key] + 1) * 10 >= section.all.length"
              :aria-label="`Next ${section.title}`"
              @click="pages[section.key]++"
              >Next</Button
            >
          </div>
        </section>
      </template>
    </DialogContent>
  </Dialog>
</template>
