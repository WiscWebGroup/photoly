<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ArrowLeft, FileImage, Upload } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useAuthStore } from '@/stores/auth'
import { useBrowserAiStore } from '@/stores/browserAi'
import { loadingCopy } from '@/lib/browserAiCopy'
import {
  bytesLabel,
  uploadCall,
  uploadError,
  useUploadsStore,
  type UploadBatch,
  type UploadItem,
} from '@/stores/uploads'
import { photoUrl } from '@/lib/photoUrl'
const uploads = useUploadsStore()
const auth = useAuthStore()
const browserAi = useBrowserAiStore()
const router = useRouter()
const indexed = computed(() => uploads.indexing.done + uploads.indexing.failed)
const indexRunning = computed(() => indexed.value < uploads.indexing.total)
const indexLoading = computed(
  () => browserAi.state.phase === 'loading' && browserAi.state.kind === 'image',
)
const indexLoadCopy = computed(() =>
  loadingCopy(browserAi.state.source, browserAi.state.loadedMB, browserAi.state.totalMB, 'image'),
)
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
  return ['pending', 'ready', 'processing', 'waiting'].reduce((n, s) => n + count(batch, s), 0)
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
async function clearActivity(batchId?: string) {
  if (await uploads.manageActivity('clear', batchId)) {
    detailSeq++
    selected.value = null
    items.value = []
  }
}
</script>
<template>
  <div class="space-y-4">
    <div class="flex gap-2">
      <Button size="sm" @click="uploads.rootPicker"><Upload />New upload</Button
      ><Button size="sm" variant="outline" @click="(uploads.refresh(), loadItems())"
        >Refresh</Button
      >
    </div>
    <div class="flex flex-wrap gap-2">
      <Button
        size="sm"
        variant="outline"
        :disabled="uploads.managing || !uploads.total"
        @click="clearActivity()"
        >Clear finished</Button
      >
      <Button
        v-if="uploads.failed"
        size="sm"
        variant="ghost"
        :disabled="uploads.managing"
        @click="uploads.manageActivity('acknowledge')"
        >Dismiss failure alerts</Button
      >
    </div>
    <p class="text-muted-foreground text-xs">
      Clearing finished activity keeps your photos. Active uploads stay here.
    </p>
    <p v-if="uploads.error" role="alert" class="text-destructive text-sm">{{ uploads.error }}</p>
    <div v-if="uploads.local.some((i) => i.state === 'sending')" class="space-y-1">
      <p class="text-muted-foreground text-xs">
        Sending from this browser… {{ uploads.progress }}%
      </p>
      <Progress :model-value="uploads.progress" aria-label="Upload progress" />
    </div>
    <div
      v-if="uploads.indexing.total"
      role="status"
      aria-live="polite"
      class="bg-card space-y-2 rounded-xl border p-3 text-sm"
    >
      <template v-if="indexLoading">
        <div class="flex items-baseline justify-between gap-2">
          <span class="font-medium">{{ indexLoadCopy.headline }}</span>
          <span class="text-muted-foreground text-xs tabular-nums">{{ indexLoadCopy.amount }}</span>
        </div>
        <Progress :model-value="browserAi.state.percent" aria-label="Image model loading" />
        <p class="text-muted-foreground text-xs">{{ indexLoadCopy.detail }}</p>
      </template>
      <template v-else>
        <div class="flex items-baseline justify-between gap-2">
          <span class="font-medium">{{
            indexRunning
              ? 'Indexing for search in this browser'
              : 'Indexed for search in this browser'
          }}</span>
          <span class="text-muted-foreground text-xs tabular-nums"
            >{{ indexed }} of {{ uploads.indexing.total }}</span
          >
        </div>
        <Progress
          :model-value="Math.round((indexed / uploads.indexing.total) * 100)"
          aria-label="Browser indexing progress"
        />
      </template>
      <p v-if="uploads.indexing.failed" class="text-muted-foreground text-xs">
        {{ uploads.indexing.failed }}
        {{ uploads.indexing.failed === 1 ? 'photo' : 'photos' }} will be indexed by the server
        instead.
      </p>
      <p v-if="indexRunning" class="text-muted-foreground text-xs">
        Keep this tab open to finish indexing here. If you close it, the server indexes the rest.
      </p>
    </div>
    <p
      v-if="uploads.local.some((i) => i.state === 'failed' && !i.acknowledged)"
      role="alert"
      class="text-destructive text-sm"
    >
      A file could not be sent. Use Retry on its upload.
    </p>
    <div class="min-w-0">
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
        <div v-if="!uploads.batches.length" class="text-muted-foreground py-12 text-center text-sm">
          Your recent uploads will appear here.
        </div>
        <article
          v-for="batch in uploads.batches"
          :key="batch.id"
          class="bg-muted/20 mb-3 rounded-lg border p-3"
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
            ><Button v-if="count(batch, 'pending')" variant="ghost" size="sm" @click="resume(batch)"
              >Reselect files</Button
            ><Button
              v-if="pending(batch) || !batch.sealed"
              variant="ghost"
              size="sm"
              @click="uploads.action(batch.id, 'cancel')"
              >Cancel remaining</Button
            ><Button
              v-if="batch.sealed && !pending(batch)"
              variant="ghost"
              size="sm"
              :disabled="uploads.managing"
              @click="clearActivity(batch.id)"
              >Clear activity</Button
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
  </div>
</template>
