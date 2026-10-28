<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Switch } from '@/components/ui/switch'
import { call } from '@/lib/http'

interface StorageInfo {
  showOtherStorage?: boolean
  backend?: string
}

const show = ref<boolean | null>(null)
const backend = ref('')
const busy = ref(false)

onMounted(async () => {
  try {
    const info = await call<StorageInfo>({ url: '/user/getStorage' })
    show.value = !!info?.showOtherStorage
    backend.value = info?.backend ?? ''
  } catch {
    show.value = null
  }
})

async function set(value: boolean) {
  if (busy.value) return
  busy.value = true
  try {
    await call({
      method: 'post',
      url: '/user/updateStorageVisibility',
      params: { show: String(value) },
    })
    show.value = value
    toast.success(value ? 'Showing media from all storage' : 'Showing current storage only')
  } catch {
    toast.error('Could not save this setting')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex max-w-[560px] flex-col gap-2">
    <label class="flex items-center gap-3">
      <Switch
        aria-label="Show media from other storage"
        :model-value="!!show"
        :disabled="show === null || busy"
        @update:model-value="set($event)"
      />
      <span class="text-sm font-medium">Show media from other storage</span>
    </label>
    <p class="text-muted-foreground text-sm">
      <template v-if="backend">New uploads are saved to {{ backend }}. </template>
      Photoly lists only media stored there. Turn this on to also list media kept in other storage;
      anything whose storage is no longer connected may not load.
    </p>
  </div>
</template>
