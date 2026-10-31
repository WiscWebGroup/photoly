<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Archive, FileImage, FolderUp, Upload } from '@lucide/vue'
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
import { useAuthStore } from '@/stores/auth'
import { bytesLabel, supportedUpload, useUploadsStore, type PickedFile } from '@/stores/uploads'
import { droppedFiles } from '@/lib/uploadFiles'
import { newUuid } from '@/lib/uuid'

const uploads = useUploadsStore()
const auth = useAuthStore()
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
      pickPage.value = 1
      uploads.open = false
    }
  },
)
function paste(event: ClipboardEvent) {
  if (!uploads.pickerOpen || uploads.submitting) return
  const images = [...(event.clipboardData?.items ?? [])]
    .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
    .map((item) => item.getAsFile())
    .filter((file): file is File => !!file)
  if (!images.length) return
  event.preventDefault()
  picked.value = picked.value.concat(
    images.map((file) => {
      const ext =
        (
          {
            'image/png': 'png',
            'image/jpeg': 'jpg',
            'image/gif': 'gif',
            'image/webp': 'webp',
          } as Record<string, string>
        )[file.type] ?? 'png'
      const name =
        /^.+\.[a-z0-9]+$/i.test(file.name) && file.name !== `image.${ext}`
          ? file.name
          : `Clipboard ${new Date().toISOString().replace(/[:.]/g, '-')}-${newUuid().slice(0, 8)}.${ext}`
      const named = new File([file], name, { type: file.type, lastModified: file.lastModified })
      return { file: named, path: name }
    }),
  )
}
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
watch(
  () => auth.token,
  () => {
    picked.value = []
  },
)
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
    <DialogContent class="sm:max-w-xl" @paste="paste">
      <DialogHeader
        ><DialogTitle>{{
          uploads.resumeBatch ? 'Reselect pending files' : `Upload to ${uploads.target?.name}`
        }}</DialogTitle
        ><DialogDescription
          >Choose files, a folder, or a ZIP, or paste an image with Cmd+V /
          Ctrl+V.</DialogDescription
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
</template>
