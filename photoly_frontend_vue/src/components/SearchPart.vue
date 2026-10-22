<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import MediaList from '@/components/MediaList.vue'
import AiSearchPanel from '@/components/ai/AiSearchPanel.vue'
import { Search, Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { Photo } from '@/composables/usePhotos'
import { useAuthStore } from '@/stores/auth'
import { useSmartSearch } from '@/composables/useSmartSearch'
import { ApiError, call } from '@/lib/http'

const searchTypes = [
  { value: 'smart', label: 'Smart', placeholder: 'Describe a photo, or type a name or tag' },
  { value: 'photoName', label: 'Name', placeholder: 'Photo name' },
  { value: 'tag', label: 'Tag', placeholder: 'Tag name' },
  { value: 'gallery', label: 'Gallery', placeholder: 'Gallery name' },
  { value: 'namespace', label: 'Folder', placeholder: 'Folder name' },
  { value: 'text', label: 'Natural language', placeholder: '' },
]
const input = ref<HTMLInputElement>()
const placeholder = () => searchTypes.find((t) => t.value === searchType.value)?.placeholder

// Arriving here (the "/" shortcut included) puts the cursor in the box, except on touch screens.
onMounted(() => {
  if (window.matchMedia?.('(pointer: fine)').matches) input.value?.focus()
})

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

/** Whether a search has run, so the empty state can tell "nothing yet" from "no matches". */
const submitted = ref(false)
watch(searchType, () => (submitted.value = false))

async function queryPhotos() {
  submitted.value = !!query.value.trim()
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
  <div class="flex min-h-full flex-col" @contextmenu.prevent.stop>
    <div class="bg-card flex flex-col gap-4 border-b px-4 pt-5 pb-5 md:px-10 md:pt-10">
      <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Search</h1>
      <form
        v-if="searchType !== 'text'"
        class="flex max-w-[880px] gap-2"
        role="search"
        @submit.prevent="queryPhotos"
      >
        <label class="relative min-w-0 flex-1">
          <component
            :is="searchType === 'smart' ? Sparkles : Search"
            class="text-primary pointer-events-none absolute top-1/2 left-4 size-[18px] -translate-y-1/2"
            aria-hidden="true"
          />
          <input
            ref="input"
            v-model="query"
            type="text"
            enterkeyhint="search"
            :maxlength="searchType === 'smart' ? 512 : undefined"
            :placeholder="placeholder()"
            aria-label="Search query"
            class="bg-background placeholder:text-muted-foreground h-12 w-full rounded-xl border pr-4 pl-11 text-base shadow-xs transition-[border-color,box-shadow] outline-none focus:border-primary focus:ring-3 focus:ring-ring/25"
          />
        </label>
        <Button type="submit" class="h-12 rounded-xl px-6 text-base">Search</Button>
      </form>
      <div
        role="radiogroup"
        aria-label="Search by"
        class="bg-muted flex max-w-full gap-0.5 self-start overflow-x-auto rounded-lg p-0.5 [scrollbar-width:none]"
      >
        <button
          v-for="t in searchTypes"
          :key="t.value"
          type="button"
          role="radio"
          :aria-checked="searchType === t.value"
          :class="
            cn(
              'text-muted-foreground hover:text-foreground flex h-8 shrink-0 items-center gap-1.5 rounded-md px-3 text-sm font-medium whitespace-nowrap transition-[background-color,color,box-shadow]',
              searchType === t.value && 'bg-card text-foreground shadow-xs',
            )
          "
          @click="searchType = t.value"
        >
          <Sparkles v-if="t.value === 'smart'" class="text-primary size-3.5" aria-hidden="true" />
          {{ t.label }}
        </button>
      </div>
    </div>

    <div class="flex w-full max-w-[1280px] flex-col gap-5 px-4 pt-6 pb-32 md:px-10">
      <div v-if="searchType === 'smart'" aria-live="polite" class="flex flex-col gap-1 text-sm">
        <p v-if="smart.error.value" role="alert" class="text-destructive">
          {{ smart.error.value }}
        </p>
        <p
          v-if="smart.busy.value"
          role="status"
          class="text-muted-foreground flex items-center gap-2"
        >
          <span class="bg-primary size-1.5 animate-pulse rounded-full" aria-hidden="true" />
          {{
            smart.semanticBusy.value
              ? 'Searching… Results will update when semantic search finishes.'
              : 'Searching…'
          }}
        </p>
        <p v-if="smart.textNotice.value" role="status" class="text-muted-foreground">
          {{ smart.textNotice.value }}
        </p>
        <p v-if="smart.semanticNotice.value" role="status" class="text-muted-foreground">
          {{ smart.semanticNotice.value }}
        </p>
        <p
          v-if="smart.submitted.value && (!smart.busy.value || smart.results.value.length)"
          role="status"
        >
          <span class="font-semibold">{{
            smart.limited.value
              ? 'Showing top 100 results'
              : `${smart.results.value.length} result(s) found`
          }}</span>
          <span class="text-muted-foreground"> · Ranked by visual match, then names and tags</span>
        </p>
      </div>
      <AiSearchPanel v-if="searchType === 'text'" :mode="searchType" />
      <MediaList
        v-else
        ref="mediaList"
        heading="Results"
        context="Search"
        :empty-title="submitted ? 'No matches' : 'Search your library'"
        :empty-description="
          submitted
            ? 'Try other words, or another way of searching.'
            : 'Describe a scene, or switch to search by name, tag, gallery or folder.'
        "
        :ns-or-not="false"
        :detach-enabled="false"
        @query-photos="queryPhotos"
      />
    </div>
  </div>
</template>
