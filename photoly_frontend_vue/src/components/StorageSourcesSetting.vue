<script setup lang="ts">
import { onMounted } from 'vue'
import StorageScanProgress from './StorageScanProgress.vue'
import { useStorageRevision } from '@/composables/useStorageRevision'
import S3StorageSettings from './S3StorageSettings.vue'
import ExternalStorageSettings from './ExternalStorageSettings.vue'
import { RefreshCw } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { call } from '@/lib/http'

const { sources, poll } = useStorageRevision(
  () => {},
  () => false,
)
async function load() {
  await poll()
}
async function scanLocal(id: number) {
  try {
    await call({ url: `/user/storage/sources/${id}/scan`, method: 'post', data: {} })
    await poll()
  } catch {
    toast.error('Could not request scan')
  }
}
onMounted(load)
</script>

<template>
  <section class="flex max-w-[560px] flex-col gap-5" aria-label="Personal storage">
    <div>
      <h3 class="font-medium">Local library</h3>
      <p class="text-muted-foreground text-sm">
        Your library folders are stored on disk. External changes are scanned automatically,
        normally every minute.
      </p>
      <template
        v-for="source in sources.filter((s) => s.sourceType === 'local')"
        :key="source.sourceId"
      >
        <p class="mt-2 font-medium">{{ source.name || 'Photoly' }}</p>
        <StorageScanProgress :status="source" />
        <div class="mt-2 flex flex-wrap items-center gap-2">
          <Button
            v-if="source.enabled"
            size="sm"
            variant="outline"
            @click="scanLocal(source.sourceId)"
            >Scan source</Button
          >
          <Button size="sm" variant="outline" @click="load"
            ><RefreshCw class="size-4" /> Refresh status</Button
          >
        </div>
      </template>
    </div>
    <S3StorageSettings @changed="poll" />
    <ExternalStorageSettings />
  </section>
</template>
