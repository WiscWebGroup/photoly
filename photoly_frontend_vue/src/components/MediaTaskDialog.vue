<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import ActivityPopover from './ActivityPopover.vue'
import { Download } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import type { Photo } from '@/composables/usePhotos'
import { uploadCall, useUploadsStore } from '@/stores/uploads'
import { useAuthStore } from '@/stores/auth'
import { newUuid } from '@/lib/uuid'
const props = defineProps<{ photo?: Photo | null; popover?: boolean }>()
const open = defineModel<boolean>('open', { default: false })
const route = useRoute()
const uploads = useUploadsStore()
const auth = useAuthStore()
const format = ref('jpeg')
const url = ref('')
const nsId = ref<number>()
const folder = ref('Loading…')
const animated = ref(false)
const live = ref(false)
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const picker = ref<InstanceType<typeof MoveToShowFolder>>()
let controller: AbortController | undefined
let requestKey = { signature: '', id: '' }
async function chooseFolder(id: number) {
  nsId.value = id
  const signal = controller?.signal
  const path = await uploadCall<{ nsName: string }[]>(
    `/namespace/trace?nsId=${id}`,
    'get',
    undefined,
    controller?.signal,
  )
  if (signal?.aborted) return
  folder.value = ['Library', ...path.slice(1).map((n) => n.nsName)].join(' / ')
}
watch(
  open,
  async (value) => {
    controller?.abort()
    controller = new AbortController()
    if (!value) return
    const signal = controller.signal
    error.value = ''
    loading.value = true
    folder.value = 'Loading…'
    animated.value = live.value = false
    nsId.value = undefined
    try {
      if (props.photo) {
        const info = await uploadCall<{ nsId: number; animated: boolean; live: boolean }>(
          `/photo/mediaInfo?photoId=${props.photo.photoId}`,
          'get',
          undefined,
          signal,
        )
        if (signal.aborted) return
        animated.value = info.animated
        live.value = info.live
        await chooseFolder(info.nsId)
      } else {
        const current =
          route.path === '/home/HomePart'
            ? (uploads.currentFolder?.nsId ?? Number(route.query.folder))
            : 0
        const id =
          current > 0
            ? current
            : (await uploadCall<{ nsId: number }>('/namespace/getRoot', 'get', undefined, signal))
                .nsId
        if (!signal.aborted) await chooseFolder(id)
      }
    } catch {
      if (!signal.aborted) error.value = 'Could not load the destination. Close and try again.'
    } finally {
      if (!signal.aborted) loading.value = false
    }
  },
  { immediate: true },
)
watch(
  () => auth.token,
  () => {
    open.value = false
    controller?.abort()
    requestKey = { signature: '', id: '' }
    url.value = ''
    format.value = 'jpeg'
  },
)
onUnmounted(() => controller?.abort())
async function submit() {
  if (busy.value || loading.value || !nsId.value) return
  busy.value = true
  error.value = ''
  const payload = props.photo
    ? { photoId: props.photo.photoId, format: format.value }
    : { url: url.value.trim(), nsId: nsId.value, format: format.value }
  const signature = JSON.stringify(payload)
  if (requestKey.signature !== signature) requestKey = { signature, id: newUuid() }
  const signal = controller?.signal
  try {
    await uploadCall<string>(
      props.photo ? '/photo/convert' : '/photo/onlineDownload',
      'post',
      { ...payload, id: requestKey.id },
      signal,
    )
    if (signal?.aborted) return
    requestKey = { signature: '', id: '' }
    open.value = false
    uploads.page = 1
    uploads.open = true
    await uploads.refresh()
    toast.success('Queued. Follow progress in Upload activity.')
  } catch (e) {
    if (!signal?.aborted) error.value = e instanceof Error ? e.message : 'Could not queue the task'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <component
    :is="popover ? ActivityPopover : Dialog"
    v-model:open="open"
    :title="popover ? 'Online Download' : undefined"
    :icon="popover ? Download : undefined"
    :description="popover ? 'Save an image from a public link.' : undefined"
    :panel="popover ? 'download' : undefined"
  >
    <template #trigger><slot name="trigger" /></template>
    <component :is="popover ? 'div' : DialogContent" :class="popover ? undefined : 'sm:max-w-md'">
      <DialogHeader v-if="!popover">
        <DialogTitle>{{ photo ? 'Format Convert' : 'Online Download' }}</DialogTitle>
        <DialogDescription class="sr-only">{{
          photo
            ? 'Save a private copy in the original photo’s folder.'
            : 'Download a public image link into your library.'
        }}</DialogDescription>
      </DialogHeader>
      <form class="flex flex-col gap-4" @submit.prevent="submit">
        <label v-if="!photo" class="flex flex-col gap-1 text-sm"
          >Image URL<input
            v-model="url"
            type="url"
            required
            placeholder="https://example.com/image.jpg"
            class="bg-background rounded-md border px-3 py-2"
        /></label>
        <label class="flex flex-col gap-1 text-sm"
          >Format<select
            aria-label="Format"
            v-model="format"
            class="bg-background rounded-md border px-3 py-2"
          >
            <option value="jpeg">JPEG</option>
            <option value="png">PNG</option>
            <option value="gif">GIF</option>
          </select></label
        >
        <div class="text-sm">
          <span class="text-muted-foreground">Save to</span>
          <p class="break-words">{{ folder }}</p>
          <Button
            v-if="!photo"
            type="button"
            variant="outline"
            size="sm"
            class="mt-2"
            @click="picker?.openModal()"
            >Choose folder</Button
          >
        </div>
        <p v-if="(animated || live) && format !== 'gif'" class="text-muted-foreground text-sm">
          {{
            live
              ? 'Uses the static Live Photo cover.'
              : 'Only the first animation frame will be saved.'
          }}
        </p>
        <p v-if="format === 'jpeg'" class="text-muted-foreground text-sm">
          JPEG quality 90. Transparent areas become white.
        </p>
        <p v-if="error" role="alert" class="text-destructive text-sm">{{ error }}</p>
        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" @click="open = false">Cancel</Button
          ><Button type="submit" :disabled="busy || loading || !nsId">{{
            busy ? 'Queuing…' : photo ? 'Save copy' : 'Download'
          }}</Button>
        </div>
      </form>
      <MoveToShowFolder
        v-if="!photo"
        ref="picker"
        title="Save to folder"
        confirm="Use folder"
        @move-to-update="(id) => chooseFolder(id).catch(() => (error = 'Could not load folder'))"
      />
    </component>
  </component>
</template>
