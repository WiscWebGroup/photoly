<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Server, RefreshCw, CircleAlert } from '@lucide/vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { call } from '@/lib/http'
import { toast } from 'vue-sonner'
import {
  storageFolder,
  useStorageRevision,
  type StorageStatus,
} from '@/composables/useStorageRevision'
import StorageScanProgress from './StorageScanProgress.vue'
const { sources, poll } = useStorageRevision(
  () => {},
  () => false,
)
const open = ref(false)
const busy = ref(new Set<number>())
const loadingFailures = ref(false)
const failureSource = ref<number | null>(null)
interface Failure {
  path: string
  code: string
  message: string
  retryable: boolean
  attemptedAt: string
}
const failures = ref<Failure[]>([])
const failureTotal = ref(0)
const active = computed(() =>
  sources.value.some((s) => s.scanStatus === 'scanning' || s.scanQueued),
)
const errors = computed(() => sources.value.some((s) => s.scanRetries || s.scanStatus === 'error'))
const current = computed(() =>
  sources.value.find((s) => s.sourceId === storageFolder.value?.sourceId),
)
watch(open, (value) => {
  if (value) void poll()
})
async function scan(source: StorageStatus, folder = false, force = false) {
  busy.value.add(source.sourceId)
  try {
    await call({
      method: 'post',
      url: `/user/storage/sources/${source.sourceId}/scan`,
      data: { ...(folder ? { nsId: storageFolder.value?.nsId } : {}), force },
    })
    toast.success('Scan queued')
    await poll()
  } catch {
    toast.error('Could not request scan')
  } finally {
    busy.value.delete(source.sourceId)
  }
}
async function showFailures(id: number, more = false) {
  if (loadingFailures.value) return
  loadingFailures.value = true
  const offset = more && failureSource.value === id ? failures.value.length : 0
  failureSource.value = id
  if (!offset) failures.value = []
  try {
    const result = await call<{ items: Failure[]; total: number }>({
      url: `/user/storage/sources/${id}/failures`,
      params: { offset, limit: 25 },
    })
    failures.value = offset ? [...failures.value, ...result.items] : result.items
    failureTotal.value = result.total
  } catch {
    toast.error('Could not load scan failures')
  } finally {
    loadingFailures.value = false
  }
}
</script>
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <button
        type="button"
        aria-label="Storage activity"
        title="Storage activity"
        class="hover:bg-accent relative grid size-9 shrink-0 place-items-center rounded-lg"
      >
        <Server class="size-4" />
        <span
          v-if="active"
          class="bg-primary absolute top-1 right-1 size-2 animate-pulse rounded-full"
          aria-label="Scanning"
        />
        <CircleAlert
          v-else-if="errors"
          class="text-destructive absolute top-0 right-0 size-3"
          aria-label="Storage needs attention"
        />
      </button>
    </PopoverTrigger>
    <PopoverContent
      align="end"
      data-storage-panel
      class="max-h-[75dvh] w-[min(440px,calc(100vw-24px))] space-y-4 overflow-y-auto"
    >
      <div class="flex items-center justify-between">
        <h2 class="font-semibold">Storage activity</h2>
        <Button variant="ghost" size="sm" aria-label="Refresh storage status" @click="poll"
          ><RefreshCw class="size-4"
        /></Button>
      </div>
      <Button
        v-if="storageFolder && current?.enabled && current.sourceType !== 's3'"
        variant="outline"
        size="sm"
        :disabled="busy.has(current.sourceId)"
        @click="scan(current, true)"
        ><RefreshCw class="size-4" />Refresh this folder</Button
      >
      <p
        v-if="storageFolder && current?.enabled && current.sourceType !== 's3'"
        class="text-muted-foreground text-xs"
      >
        {{ storageFolder.name }} and its subfolders
      </p>
      <p v-if="!sources.length" class="text-muted-foreground text-sm">
        No storage sources available.
      </p>
      <section v-for="source in sources" :key="source.sourceId" class="space-y-2 border-t pt-3">
        <div class="flex justify-between gap-2">
          <h3 class="font-medium">
            {{ source.name || (source.sourceType === 'local' ? 'Photoly' : 'S3') }}
          </h3>
          <span class="text-muted-foreground text-xs"
            >{{ source.sourceType?.toUpperCase() }} ·
            {{ source.enabled ? 'Enabled' : 'Disabled' }}</span
          >
        </div>
        <p v-if="source.sourceType === 's3'" class="text-muted-foreground text-sm">
          Only files uploaded through Photoly are managed. Bucket contents are not scanned.
        </p>
        <template v-else>
          <StorageScanProgress :status="source" />
          <div v-if="source.enabled" class="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              :disabled="busy.has(source.sourceId)"
              @click="scan(source)"
              >Scan source</Button
            >
            <Button
              size="sm"
              variant="ghost"
              :disabled="busy.has(source.sourceId)"
              @click="scan(source, false, true)"
              >Verify all content</Button
            >
          </div>
          <Button
            v-if="source.scanRetries"
            size="sm"
            variant="link"
            :disabled="loadingFailures"
            @click="showFailures(source.sourceId)"
            >View {{ source.scanRetries }} file issues</Button
          >
          <div v-if="failureSource === source.sourceId" class="space-y-3 text-xs">
            <div v-for="failure in failures" :key="failure.path" class="rounded border p-2">
              <p class="break-all font-medium">{{ failure.path }}</p>
              <p>{{ failure.message }}</p>
              <p class="text-muted-foreground">
                {{
                  failure.retryable
                    ? 'Will retry automatically'
                    : 'Waiting for file changes or forced verification'
                }}
                · {{ new Date(failure.attemptedAt).toLocaleString() }}
              </p>
            </div>
            <Button
              v-if="failures.length < failureTotal"
              size="sm"
              variant="outline"
              :disabled="loadingFailures"
              @click="showFailures(source.sourceId, true)"
              >Load more</Button
            >
          </div>
        </template>
      </section>
    </PopoverContent>
  </Popover>
</template>
