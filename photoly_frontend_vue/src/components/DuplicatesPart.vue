<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Copy, RefreshCw, Trash2 } from '@lucide/vue'
import { call } from '@/lib/http'
import {
  duplicateError,
  type DuplicateGroup,
  type DuplicateMember,
  type DuplicatePage,
} from '@/lib/duplicates'
import { useStorageRevision } from '@/composables/useStorageRevision'
import { useUploadsStore } from '@/stores/uploads'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import DuplicateGroupCard from './DuplicateGroupCard.vue'
import PhotoViewer from './PhotoViewer.vue'
const groups = ref<DuplicateGroup[]>([]),
  total = ref(0),
  offset = ref(0),
  ignored = ref(false)
const loading = ref(false),
  busy = ref(false),
  error = ref(''),
  result = ref(''),
  generation = ref(0)
const selection = ref(new Map<string, DuplicateMember[]>()),
  confirming = ref(false),
  preview = ref<DuplicateMember | null>(null)
const limit = 10
const selectedCount = computed(() =>
  [...selection.value.values()].reduce((n, items) => n + items.length, 0),
)
let sequence = 0
async function load() {
  const seq = ++sequence
  loading.value = true
  error.value = ''
  selection.value.clear()
  try {
    const data = await call<DuplicatePage>({
      url: '/photo/duplicates',
      params: { ignored: ignored.value, offset: offset.value, limit },
    })
    if (seq !== sequence) return
    total.value = data.total
    if (offset.value >= data.total && offset.value > 0) {
      offset.value = Math.max(0, Math.floor((data.total - 1) / limit) * limit)
      return
    }
    groups.value = data.items
    generation.value++
  } catch (e) {
    if (seq === sequence) error.value = duplicateError(e)
  } finally {
    if (seq === sequence) loading.value = false
  }
}
function filter(value: boolean) {
  ignored.value = value
  offset.value = 0
}
async function remove() {
  const batches = groups.value.flatMap((group) => {
    const items = selection.value.get(group.key) ?? []
    return items.length ? [{ group, items }] : []
  })
  busy.value = true
  let deleted = 0
  const failures: string[] = []
  for (const { group, items } of batches) {
    try {
      await call({
        method: 'post',
        url: '/photo/duplicates/delete',
        data: { key: group.key, version: group.version, photoIds: items.map((p) => p.photoId) },
      })
      deleted += items.length
    } catch (e) {
      failures.push(duplicateError(e))
    }
  }
  confirming.value = false
  result.value = `${deleted} ${deleted === 1 ? 'photo' : 'photos'} deleted.${failures.length ? ` ${failures.length} group(s) failed: ${[...new Set(failures)].join(' ')}` : ''}`
  if (deleted) useUploadsStore().revision++
  await load()
  busy.value = false
}
watch([ignored, offset], load)
useStorageRevision(
  load,
  () =>
    busy.value || loading.value || confirming.value || !!preview.value || selectedCount.value > 0,
)
onMounted(load)
</script>
<template>
  <div class="mx-auto w-full max-w-[1600px] space-y-6 p-4 md:p-8">
    <header class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="flex items-center gap-2 text-2xl font-semibold">
          <Copy class="size-6" />Duplicate Detection
        </h1>
        <p class="text-muted-foreground mt-2 max-w-2xl text-sm">
          Review identical photos across your folders. Keep your favorites, ignore a group, or
          delete extra copies.
        </p>
        <p class="text-muted-foreground mt-1 max-w-2xl text-xs">
          Based on indexed file content. Missing or unindexed files and standalone videos are
          excluded. Live Photos must match both the photo and video.
        </p>
      </div>
      <Button variant="outline" :disabled="loading || busy" @click="load"
        ><RefreshCw class="size-4" />Refresh</Button
      >
    </header>
    <div class="flex flex-wrap items-center gap-3">
      <nav aria-label="Duplicate status" class="flex gap-2">
        <Button
          :variant="!ignored ? 'secondary' : 'ghost'"
          :aria-pressed="!ignored"
          :disabled="busy || loading"
          @click="filter(false)"
          >To review</Button
        ><Button
          :variant="ignored ? 'secondary' : 'ghost'"
          :aria-pressed="ignored"
          :disabled="busy || loading"
          @click="filter(true)"
          >Ignored</Button
        >
      </nav>
      <span class="text-muted-foreground mr-auto text-sm"
        >{{ total }} duplicate {{ total === 1 ? 'group' : 'groups' }}</span
      >
      <Button
        variant="destructive"
        :disabled="selectedCount === 0 || busy || loading"
        @click="confirming = true"
        ><Trash2 class="size-4" />Delete selected ({{ selectedCount }})</Button
      >
    </div>
    <p v-if="result" role="status" class="bg-muted rounded-lg p-4 text-sm">{{ result }}</p>
    <div v-if="error" role="alert" class="flex items-center gap-3 text-sm text-destructive">
      {{ error }}<Button variant="outline" @click="load">Retry</Button>
    </div>
    <p v-if="loading" role="status" class="text-muted-foreground py-8 text-center">
      Loading duplicates…
    </p>
    <div
      v-else-if="!error && !groups.length"
      class="rounded-xl border border-dashed px-6 py-16 text-center"
    >
      <Copy class="text-muted-foreground mx-auto mb-4 size-10" />
      <h2 class="font-medium">{{ ignored ? 'No ignored groups' : 'No duplicates to review' }}</h2>
      <p class="text-muted-foreground mt-2 text-sm">
        {{
          ignored
            ? 'Groups you ignore will appear here.'
            : 'Identical indexed photos will appear here when they are found.'
        }}
      </p>
    </div>
    <div v-else class="space-y-5">
      <DuplicateGroupCard
        v-for="group in groups"
        :key="`${generation}:${group.key}`"
        :group="group"
        :disabled="busy || loading"
        @changed="load"
        @select="selection.set(group.key, $event)"
        @preview="preview = $event"
      />
    </div>
    <div v-if="total > limit" class="flex items-center justify-center gap-4">
      <Button variant="outline" :disabled="offset === 0 || loading || busy" @click="offset -= limit"
        >Previous groups</Button
      ><span class="text-sm"
        >{{ Math.floor(offset / limit) + 1 }} / {{ Math.ceil(total / limit) }}</span
      ><Button
        variant="outline"
        :disabled="offset + limit >= total || loading || busy"
        @click="offset += limit"
        >Next groups</Button
      >
    </div>
    <AlertDialog :open="confirming" @update:open="!busy && (confirming = $event)"
      ><AlertDialogContent
        ><AlertDialogHeader
          ><AlertDialogTitle>Delete {{ selectedCount }} duplicate photos?</AlertDialogTitle
          ><AlertDialogDescription
            >This permanently deletes the selected original files and their associated media from
            storage. There is no trash or undo. At least one matching copy must remain in each
            group.</AlertDialogDescription
          ></AlertDialogHeader
        >
        <ul class="max-h-48 overflow-auto text-sm">
          <template v-for="[key, items] in selection" :key="key"
            ><li v-for="item in items" :key="item.photoId" class="break-all py-1">
              {{ item.fileName }} · {{ item.sourceName }} · {{ item.folder }}
            </li></template
          >
        </ul>
        <AlertDialogFooter
          ><AlertDialogCancel :disabled="busy">Cancel</AlertDialogCancel
          ><Button variant="destructive" :disabled="busy" @click="remove">{{
            busy ? 'Deleting…' : 'Permanently delete'
          }}</Button></AlertDialogFooter
        ></AlertDialogContent
      ></AlertDialog
    >
    <PhotoViewer
      :photo="preview"
      :ns-path="preview?.folder ?? ''"
      :allow-similar="false"
      @close="preview = null"
      @changed="load"
    />
  </div>
</template>
