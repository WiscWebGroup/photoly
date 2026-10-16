<script setup lang="ts">
import type { SmartSource } from '@/lib/smartOrganization'
import { mediaKind } from '@/lib/media'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import MediaList from '@/components/MediaList.vue'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAiSearch } from '@/composables/useAiSearch'
import type { Photo } from '@/composables/usePhotos'
import { aiMessage } from '@/lib/ai'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

const emit = defineEmits<{ libraryChanged: [change?: { type: string; photo: Photo }] }>()
const autoSubmitted = ref(false)
const props = defineProps<{ mode: 'text' | 'image' | 'similar'; source?: Photo }>()
const { capabilities, result, busy, error, notice, search, cancel, clear, refreshCapabilities } =
  useAiSearch()
const auth = useAuthStore()
const text = ref(''),
  topK = ref(20),
  file = ref<File | null>(null),
  preview = ref(''),
  previewFailed = ref(false)
const galleryId = ref(''),
  folderId = ref<number>(),
  folderName = ref('All folders')
const galleries = ref<{ gaId: number; gaName: string; smart?: SmartSource | null }[]>([])
const folderPicker = ref<InstanceType<typeof MoveToShowFolder>>()
const list = ref<InstanceType<typeof MediaList>>()
const formats = computed(() =>
  (capabilities.value?.formats ?? []).filter((format) => mediaKind(format) !== 'video'),
)
const accept = computed(() => formats.value.map((f) => `.${f}`).join(','))
const allowed = computed(
  () => capabilities.value?.ready && capabilities.value.enabled && capabilities.value.globalEnabled,
)
const unavailable = computed(() =>
  capabilities.value?.reason ? aiMessage(capabilities.value.reason) : '',
)
const inputClass = 'h-9 rounded-md border bg-background px-3 text-sm'
function removeQueryImage() {
  removeFile()
  clear()
}
function clearFolder() {
  folderId.value = undefined
  folderName.value = 'All folders'
}
function cancelWaiting() {
  cancel()
  notice.value = 'Stopped waiting. The server may still be finishing this query.'
}
function removeFile() {
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = ''
  file.value = null
  previewFailed.value = false
}
function chooseFile(value?: File) {
  removeFile()
  clear()
  if (!value) return
  if (value.size > (capabilities.value?.limits.imageBytes ?? 20 * 1024 ** 2)) {
    error.value = 'The image exceeds the file size limit.'
    return
  }
  if (!formats.value.includes(value.name.split('.').pop()?.toLowerCase() ?? '')) {
    error.value = 'This image format is not supported.'
    return
  }
  file.value = value
  preview.value = URL.createObjectURL(value)
}
function fileInput(event: Event) {
  const input = event.target as HTMLInputElement
  chooseFile(input.files?.[0])
  input.value = ''
}
function drop(event: DragEvent) {
  chooseFile(event.dataTransfer?.files[0])
}
async function submit() {
  if (!allowed.value || busy.value) return
  if (
    !Number.isInteger(topK.value) ||
    topK.value < 1 ||
    topK.value > (capabilities.value?.limits.topK ?? 100)
  ) {
    error.value = 'Result count must be an integer from 1 to 100.'
    return
  }
  const value =
    props.mode === 'text'
      ? text.value.trim()
      : props.mode === 'image'
        ? file.value
        : props.source?.photoId
  if (!value) {
    error.value = props.mode === 'text' ? 'Enter a search description.' : 'Choose a query image.'
    return
  }
  await search(props.mode, value, {
    topK: topK.value,
    namespaceId: folderId.value,
    galleryId: galleryId.value ? Number(galleryId.value) : undefined,
  })
}
async function loadGalleries() {
  const token = auth.token
  try {
    const rows = await call<typeof galleries.value>({
      url: '/gallery/getAll',
      params: { includeSmart: true },
    })
    if (token === auth.token) galleries.value = rows
  } catch {
    error.value = 'Could not load albums. Please retry.'
  }
}
function changed(change?: { type: string; photo: Photo }) {
  emit('libraryChanged', change)
  if (change?.type === 'metadata' && result.value) {
    const hit = result.value.items.find((x) => x.photo.photoId === change.photo.photoId)
    if (hit) {
      Object.assign(hit.photo, change.photo)
      result.value = { ...result.value, items: [...result.value.items] }
    }
    return
  }
  result.value = null
  notice.value = 'The library changed. Search again to refresh these results.'
}
watch(
  result,
  (value) => list.value?.updatePhotoList(value?.items.map((x) => ({ ...x.photo })) ?? []),
  { flush: 'post' },
)
watch(
  () => [props.mode, props.source?.photoId],
  () => {
    clear()
    removeFile()
    autoSubmitted.value = false
  },
)
watch([allowed, () => props.source?.photoId], () => {
  if (props.mode === 'similar' && allowed.value && props.source && !autoSubmitted.value) {
    autoSubmitted.value = true
    void submit()
  }
})
watch(
  () => auth.token,
  () => {
    removeFile()
    text.value = ''
    galleryId.value = ''
    folderId.value = undefined
    galleries.value = []
    folderName.value = 'All folders'
  },
)
onBeforeUnmount(removeFile)
</script>

<template>
  <section class="space-y-4" aria-label="AI search">
    <div class="rounded-lg border bg-card p-4 space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-lg font-semibold">
          {{
            mode === 'similar'
              ? 'Similar photos and videos'
              : mode === 'image'
                ? 'Image search'
                : 'Natural language search'
          }}
        </h3>
        <Button variant="outline" size="sm" @click="refreshCapabilities"
          >Refresh availability</Button
        >
      </div>
      <p v-if="unavailable" role="status" class="text-sm text-muted-foreground">
        {{ unavailable }}
      </p>
      <p v-if="source" class="text-sm">Source: {{ source.photoName }}.{{ source.format }}</p>
      <form class="space-y-3" @submit.prevent="submit">
        <template v-if="mode === 'text'">
          <label for="ai-query" class="text-sm font-medium">Describe the image</label>
          <Input
            id="ai-query"
            v-model="text"
            :maxlength="capabilities?.limits.textCharacters ?? 512"
            placeholder="A cat in the sunlight / 阳光下的猫"
          />
          <p class="text-xs text-muted-foreground">
            {{ text.length }} / {{ capabilities?.limits.textCharacters ?? 512 }} characters. The
            model also has a token limit.
          </p>
        </template>
        <div
          v-if="mode === 'image'"
          class="rounded-md border border-dashed p-4 space-y-2"
          @dragover.prevent
          @drop.prevent="drop"
        >
          <label class="block text-sm font-medium" for="ai-image"
            >Query image — choose or drop a file</label
          >
          <input id="ai-image" type="file" :accept="accept" @change="fileInput" />
          <p class="text-xs text-muted-foreground">
            Up to {{ (capabilities?.limits.imageBytes ?? 20971520) / 1024 ** 2 }} MiB and
            {{ (capabilities?.limits.imagePixels ?? 40000000) / 1000000 }} MP. Used only for this
            search; not added to your library.
          </p>
          <template v-if="file">
            <img
              v-if="preview && !previewFailed"
              :src="preview"
              alt="Query preview"
              class="max-h-40 rounded"
              @error="previewFailed = true"
            />
            <p>
              {{ file.name }}
              <span v-if="previewFailed"
                >(Preview unavailable; the server can still validate this image.)</span
              >
            </p>
            <Button type="button" variant="outline" size="sm" @click="removeQueryImage"
              >Remove image</Button
            >
          </template>
        </div>
        <div class="flex flex-wrap items-end gap-3">
          <label class="space-y-1 text-sm"
            >Results<input
              v-model.number="topK"
              aria-label="Result count"
              type="number"
              min="1"
              :max="capabilities?.limits.topK ?? 100"
              :class="[inputClass, 'block w-24']"
          /></label>
          <Button type="button" variant="outline" @click="folderPicker?.openModal()">{{
            folderName
          }}</Button>
          <Button v-if="folderId" type="button" variant="ghost" @click="clearFolder"
            >Clear folder</Button
          >
          <label class="space-y-1 text-sm"
            >Album<select
              v-model="galleryId"
              aria-label="Filter album"
              :class="[inputClass, 'block max-w-60']"
              @focus="loadGalleries"
            >
              <option value="">All albums</option>
              <option v-for="g in galleries" :key="g.gaId" :value="String(g.gaId)">
                {{ g.gaName }}{{ g.smart ? ' · ✨ Smart' : '' }}
              </option>
            </select></label
          >
          <Button type="submit" :disabled="!allowed || busy">{{
            busy ? 'Searching…' : 'Search images'
          }}</Button>
          <Button v-if="busy" type="button" variant="outline" @click="cancelWaiting"
            >Cancel waiting</Button
          >
        </div>
        <p class="text-xs text-muted-foreground">
          Folder filters apply to that folder only, not its subfolders.
        </p>
      </form>
    </div>
    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <p v-if="notice" role="status" class="text-sm">{{ notice }}</p>
    <div v-if="result" role="status" class="text-sm space-y-1">
      <p>
        {{ result.items.length }} result(s) · {{ capabilities?.profile?.modelId ?? 'Profile' }} #{{
          result.profileId
        }}
      </p>
      <p>
        Indexed {{ result.coverage.ready }} / {{ result.coverage.supported }} supported photos and
        videos; {{ result.coverage.pending }} pending, {{ result.coverage.failed }} failed,
        {{ result.coverage.unsupported }} unsupported.
      </p>
      <p v-if="result.coverage.ready < result.coverage.supported">
        Coverage is partial. More results may become available as indexing continues.
      </p>
      <p v-if="!result.items.length">No matching indexed photos and videos were returned.</p>
      <p class="text-muted-foreground">
        Ordered by cosine similarity, not a match probability. Request: {{ result.requestId }}
      </p>
    </div>
    <MediaList
      ref="list"
      :ns-or-not="false"
      :detach-enabled="false"
      :allow-similar="mode !== 'similar'"
      @query-photos="changed"
    />
    <MoveToShowFolder
      ref="folderPicker"
      title="Choose a search folder"
      @move-to-update="
        (id: number) => {
          folderId = id
          folderName = `Folder #${id}`
        }
      "
    />
  </section>
</template>
