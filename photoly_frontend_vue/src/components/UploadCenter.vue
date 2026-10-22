<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Archive, ArrowLeft, FileImage, FolderUp, Upload } from '@lucide/vue'
import { toast } from 'vue-sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useAuthStore } from '@/stores/auth'
import {
  bytesLabel,
  supportedUpload,
  uploadCall,
  uploadError,
  useUploadsStore,
  type PickedFile,
  type UploadBatch,
  type UploadItem,
} from '@/stores/uploads'
import { droppedFiles } from '@/lib/uploadFiles'
import { photoUrl } from '@/lib/photoUrl'

const uploads = useUploadsStore()
const auth = useAuthStore()
const router = useRouter()
const picked = ref<PickedFile[]>([])
const pickPage = ref(1)
const scanning = ref(false)
const pickedPage = computed(() =>
  picked.value.slice((pickPage.value - 1) * 50, pickPage.value * 50),
)
const previews = ref<(string | null)[]>([])
watch(pickedPage, (items, _, cleanup) => {
  previews.value = items.map((i) =>
    i.file.type.startsWith('image/') && typeof URL.createObjectURL === 'function'
      ? URL.createObjectURL(i.file)
      : null,
  )
  const urls = previews.value
  cleanup(() => urls.forEach((url) => url && URL.revokeObjectURL(url)))
})
watch(
  () => uploads.pickerOpen,
  (value) => {
    picked.value = []
    if (value) {
      selected.value = null
      pickPage.value = 1
      uploads.open = false
    }
  },
)
function choose(event: Event) {
  const input = event.target as HTMLInputElement
  picked.value = picked.value.concat(
    [...(input.files ?? [])].map((file) => ({ file, path: file.webkitRelativePath || file.name })),
  )
  input.value = ''
}
async function drop(event: DragEvent) {
  if (!event.dataTransfer) return
  scanning.value = true
  try {
    picked.value = picked.value.concat(await droppedFiles(event.dataTransfer))
  } catch {
    toast.error('Could not read the dropped folder. Try Choose folder.')
  } finally {
    scanning.value = false
  }
}
const selected = ref<UploadBatch | null>(null)
const items = ref<UploadItem[]>([])
const itemPage = ref(1)
const itemTotal = ref(0)
const detailError = ref('')
let detailSeq = 0
async function loadItems() {
  if (!selected.value || !uploads.open) return
  const seq = ++detailSeq
  try {
    const data = await uploadCall<{ items: UploadItem[]; total: number }>(
      `/uploads/${selected.value.id}/items?page=${itemPage.value}`,
    )
    if (seq !== detailSeq) return
    items.value = data.items
    itemTotal.value = data.total
    detailError.value = ''
  } catch {
    if (seq === detailSeq) detailError.value = 'Could not load upload details. Try again.'
  }
}
watch([selected, itemPage, () => uploads.revision, () => uploads.open], loadItems)
watch(
  () => auth.token,
  () => {
    detailSeq++
    selected.value = null
    items.value = []
    picked.value = []
  },
)
function details(batch: UploadBatch) {
  selected.value = batch
  itemPage.value = 1
}
function count(batch: UploadBatch, state: string) {
  return batch.counts[state]?.count ?? 0
}
function pending(batch: UploadBatch) {
  return ['pending', 'ready', 'processing'].reduce((n, s) => n + count(batch, s), 0)
}
function completion(batch: UploadBatch) {
  const finished = ['done', 'failed', 'skipped', 'cancelled'].reduce(
    (n, s) => n + count(batch, s),
    0,
  )
  return finished + pending(batch)
    ? Math.round((finished / (finished + pending(batch))) * 100)
    : 100
}
function locate(item: UploadItem) {
  if (item.nsId === null) return
  uploads.open = false
  router.push({
    path: '/home/HomePart',
    query: { folder: item.nsId },
  })
}
function resume(batch: UploadBatch) {
  if (batch.nsId === null) return toast.error('Destination folder was deleted')
  uploads.start(batch.nsId, batch.name, batch.id)
}
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void uploads.refresh()
  timer = setInterval(() => {
    if (auth.token) void uploads.refresh()
  }, 3000)
})
onUnmounted(() => clearInterval(timer))
watch(
  () => auth.token,
  () => uploads.refresh(),
)
</script>

<template>
  <Dialog v-model:open="uploads.pickerOpen">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader
        ><DialogTitle>{{
          uploads.resumeBatch ? 'Reselect pending files' : `Upload to ${uploads.target?.name}`
        }}</DialogTitle
        ><DialogDescription
          >Choose files, a folder, or a ZIP. Folder structure is preserved. You can keep browsing
          after starting.</DialogDescription
        ></DialogHeader
      >
      <div
        class="border-primary/50 bg-accent/30 flex flex-col items-center gap-3 rounded-xl border-2 border-dashed p-6 text-center"
        @dragover.prevent
        @drop.prevent="drop"
      >
        <Upload class="text-primary size-7" />
        <p class="text-sm font-medium">Drop photos, videos, folders or ZIPs here</p>
        <div class="flex flex-wrap justify-center gap-2">
          <label
            class="bg-background hover:bg-accent cursor-pointer rounded-md border px-3 py-2 text-sm focus-within:ring-2"
            ><FileImage class="mr-1 inline size-4" /> Choose files / ZIP<input
              type="file"
              multiple
              class="sr-only"
              aria-label="Choose files or ZIP"
              @change="choose"
          /></label>
          <label
            class="bg-background hover:bg-accent cursor-pointer rounded-md border px-3 py-2 text-sm focus-within:ring-2"
            ><FolderUp class="mr-1 inline size-4" /> Choose folder<input
              type="file"
              multiple
              webkitdirectory
              class="sr-only"
              aria-label="Choose folder"
              @change="choose"
          /></label>
        </div>
        <p class="text-muted-foreground text-xs">
          No batch count limit · {{ bytesLabel(uploads.maxFileBytes) }} per file / ZIP
        </p>
      </div>
      <p v-if="scanning" role="status" class="text-sm">Reading folder…</p>
      <div v-if="picked.length">
        <p class="text-muted-foreground mb-2 text-xs">
          {{ picked.length.toLocaleString() }} files ·
          {{ bytesLabel(picked.reduce((n, i) => n + i.file.size, 0)) }}
        </p>
        <ul class="max-h-56 overflow-auto divide-y">
          <li
            v-for="(item, i) in pickedPage"
            :key="`${item.path}-${i}`"
            class="flex items-center gap-2 py-2 text-sm"
          >
            <img
              v-if="previews[i]"
              :src="previews[i]!"
              alt=""
              class="size-9 rounded object-cover"
            /><Archive v-else class="text-muted-foreground size-5 shrink-0" />
            <div class="min-w-0 flex-1">
              <p class="truncate" :title="item.path">{{ item.path }}</p>
              <p class="text-muted-foreground text-xs">
                {{
                  !supportedUpload(item.path)
                    ? 'Will skip: unsupported type'
                    : item.file.size > uploads.maxFileBytes
                      ? 'Too large for one request'
                      : bytesLabel(item.file.size)
                }}
              </p>
            </div>
          </li>
        </ul>
        <div v-if="picked.length > 50" class="mt-2 flex items-center justify-between text-xs">
          <Button variant="ghost" size="sm" :disabled="pickPage === 1" @click="pickPage--"
            >Previous</Button
          >{{ pickPage }} / {{ Math.ceil(picked.length / 50)
          }}<Button
            variant="ghost"
            size="sm"
            :disabled="pickPage * 50 >= picked.length"
            @click="pickPage++"
            >Next</Button
          >
        </div>
      </div>
      <DialogFooter
        ><Button variant="outline" @click="uploads.pickerOpen = false">Close</Button
        ><Button
          :disabled="!picked.length || scanning || uploads.submitting"
          @click="uploads.enqueue(picked)"
          ><Upload />{{ uploads.submitting ? 'Preparing…' : 'Upload' }}</Button
        ></DialogFooter
      >
    </DialogContent>
  </Dialog>

  <Dialog v-model:open="uploads.open">
    <DialogContent
      class="top-0 right-0 left-auto flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 flex-col rounded-none sm:max-w-lg"
    >
      <DialogHeader
        ><DialogTitle>Uploads</DialogTitle
        ><DialogDescription
          >Keep browsing while uploads finish. Recent activity is kept for 30
          days.</DialogDescription
        ></DialogHeader
      >
      <div class="flex gap-2">
        <Button size="sm" @click="uploads.rootPicker"><Upload />New upload</Button
        ><Button size="sm" variant="outline" @click="(uploads.refresh(), loadItems())"
          >Refresh</Button
        >
      </div>
      <p v-if="uploads.error" role="alert" class="text-destructive text-sm">{{ uploads.error }}</p>
      <div v-if="uploads.local.some((i) => i.state === 'sending')" class="space-y-1">
        <p class="text-muted-foreground text-xs">
          Sending from this browser… {{ uploads.progress }}%
        </p>
        <Progress :model-value="uploads.progress" aria-label="Upload progress" />
      </div>
      <p
        v-if="uploads.local.some((i) => i.state === 'failed')"
        role="alert"
        class="text-destructive text-sm"
      >
        A file could not be sent. Use Retry on its upload.
      </p>
      <div class="min-h-0 flex-1 overflow-auto">
        <template v-if="selected">
          <Button variant="ghost" size="sm" class="mb-3" @click="selected = null"
            ><ArrowLeft />All uploads</Button
          >
          <h3 class="mb-2 truncate font-medium">{{ selected.name }}</h3>
          <p v-if="detailError" role="alert" class="text-destructive text-sm">{{ detailError }}</p>
          <ul class="divide-y">
            <li v-for="item in items" :key="item.id" class="flex gap-3 py-3">
              <img
                v-if="
                  item.photoId && /\.(jpg|jpeg|png|gif|webp|bmp|tiff?|ico|jfif)$/i.test(item.path)
                "
                :src="photoUrl(item.photoId, 'renderThumbnail')"
                alt=""
                loading="lazy"
                class="size-12 rounded-md object-cover"
              />
              <FileImage v-else class="text-muted-foreground mt-1 size-6 shrink-0" />
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm" :title="item.path">{{ item.path }}</p>
                <p class="text-muted-foreground text-xs">
                  {{ item.state }} · {{ bytesLabel(item.size) }}
                </p>
                <p v-if="item.error" class="text-destructive text-xs">
                  {{ uploadError(item.error) }}
                </p>
                <Button
                  v-if="item.nsId"
                  variant="link"
                  size="sm"
                  class="h-6 px-0"
                  @click="locate(item)"
                  >View in folder</Button
                >
              </div>
            </li>
          </ul>
          <div class="flex items-center justify-between text-xs">
            <Button variant="ghost" size="sm" :disabled="itemPage === 1" @click="itemPage--"
              >Previous</Button
            >{{ itemPage }} / {{ Math.max(1, Math.ceil(itemTotal / 50))
            }}<Button
              variant="ghost"
              size="sm"
              :disabled="itemPage * 50 >= itemTotal"
              @click="itemPage++"
              >Next</Button
            >
          </div>
        </template>
        <template v-else>
          <div
            v-if="!uploads.batches.length"
            class="text-muted-foreground py-12 text-center text-sm"
          >
            Your recent uploads will appear here.
          </div>
          <article
            v-for="batch in uploads.batches"
            :key="batch.id"
            class="mb-3 rounded-xl border p-4"
          >
            <button
              class="w-full truncate text-left text-sm font-semibold hover:underline"
              @click="details(batch)"
            >
              {{ batch.name }}
            </button>
            <p class="text-muted-foreground mt-1 text-xs">
              {{ new Date(batch.createdAt).toLocaleString() }}
            </p>
            <Progress
              class="my-3"
              :model-value="completion(batch)"
              :aria-label="`Progress for ${batch.name}`"
            />
            <p class="text-muted-foreground text-xs">
              {{ count(batch, 'done') }} uploaded · {{ count(batch, 'failed') }} failed ·
              {{ count(batch, 'skipped') }} skipped<span v-if="pending(batch)">
                · {{ pending(batch) }} remaining</span
              >
            </p>
            <p
              v-if="
                count(batch, 'pending') &&
                !uploads.local.some(
                  (i) => i.batchId === batch.id && ['pending', 'sending'].includes(i.state),
                )
              "
              class="text-muted-foreground mt-2 text-xs"
            >
              Some files have not reached the server. Reselect them to continue.
            </p>
            <div class="mt-3 flex flex-wrap gap-1">
              <Button variant="outline" size="sm" @click="details(batch)">Details</Button
              ><Button
                v-if="
                  count(batch, 'failed') ||
                  count(batch, 'cancelled') ||
                  uploads.local.some((i) => i.batchId === batch.id && i.state === 'failed')
                "
                variant="ghost"
                size="sm"
                @click="uploads.action(batch.id, 'retry')"
                >Retry</Button
              ><Button
                v-if="count(batch, 'pending')"
                variant="ghost"
                size="sm"
                @click="resume(batch)"
                >Reselect files</Button
              ><Button
                v-if="pending(batch)"
                variant="ghost"
                size="sm"
                @click="uploads.action(batch.id, 'cancel')"
                >Cancel remaining</Button
              >
            </div>
          </article>
          <div v-if="uploads.total > 20" class="flex items-center justify-between text-xs">
            <Button
              variant="ghost"
              size="sm"
              :disabled="uploads.page === 1"
              @click="(uploads.page--, uploads.refresh())"
              >Previous</Button
            >{{ uploads.page }} / {{ Math.ceil(uploads.total / 20)
            }}<Button
              variant="ghost"
              size="sm"
              :disabled="uploads.page * 20 >= uploads.total"
              @click="(uploads.page++, uploads.refresh())"
              >Next</Button
            >
          </div>
        </template>
      </div>
      <p class="text-muted-foreground border-t pt-3 text-xs">
        Keep this browser tab open while sending. Files already received continue processing on the
        server.
      </p>
    </DialogContent>
  </Dialog>
</template>
