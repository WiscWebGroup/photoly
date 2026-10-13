<script setup lang="ts">
import { computed, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Download, Eye, FolderInput, Trash2 } from '@lucide/vue'
import EmptyState from '@/components/EmptyState.vue'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import PhotoViewer from '@/components/PhotoViewer.vue'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { Switch } from '@/components/ui/switch'
import { usePhotos, type Photo } from '@/composables/usePhotos'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { photoUrl } from '@/lib/photoUrl'
import { useAuthStore } from '@/stores/auth'
import videoIcon from '@/assets/icons/Video.png'
import logo from '@/assets/logo.ico'

const props = defineProps<{ nsOrNot: boolean; detachEnabled?: boolean; detachStr?: string }>()
/** `queryPhotos`: the list changed on the server, reload it. `doDetach`: remove the checked photos from the parent's tag/gallery. */
const emit = defineEmits<{ queryPhotos: []; doDetach: [photos: Photo[]] }>()

const auth = useAuthStore()
const { photos, moveTo, remove } = usePhotos()

const selectMode = ref(false)
const viewing = ref<Photo | null>(null)
const nsPath = ref('')
const confirmDelete = ref(false)
const confirmDetach = ref(false)
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
/** The photos the next folder pick applies to. */
const moving = ref<Photo[]>([])

const checked = computed(() => photos.value.filter((p) => p.checked))

function setChecked(value: boolean) {
  photos.value.forEach((p) => (p.checked = value))
}

async function trace(id: number) {
  try {
    const path = await call<{ nsName: string }[]>({ url: '/namespace/trace', params: { nsId: id } })
    nsPath.value =
      '/root' +
      path
        .slice(1)
        .map((n) => `/${n.nsName}`)
        .join('')
  } catch {
    toast.error('trace Response Error!')
  }
}

function open(photo: Photo) {
  if (selectMode.value) {
    photo.checked = !photo.checked
    return
  }
  viewing.value = photo
  if (!props.nsOrNot && photo.nsId !== undefined) trace(photo.nsId)
}

/** Runs one request per photo, reports the outcome once, and asks the parent to reload. */
async function applyToAll(
  targets: Photo[],
  action: (p: Photo) => Promise<unknown>,
  success: string,
  failure: string,
) {
  try {
    await Promise.all(targets.map(action))
    toast.success(success)
  } catch {
    toast.warning(failure)
  }
  emit('queryPhotos')
}

function deleteChecked() {
  confirmDelete.value = false
  selectMode.value = false
  const targets = [...checked.value]
  setChecked(false)
  return applyToAll(
    targets,
    (p) => remove(p.photoId),
    'Photo deleted!',
    'delete photo Response Error!',
  )
}

function detachChecked() {
  confirmDetach.value = false
  selectMode.value = false
  emit('doDetach', photos.value)
}

function askMove(targets: Photo[]) {
  moving.value = targets
  movePicker.value?.openModal()
}

function moveChosen(targetNsId: number) {
  const targets = moving.value
  if (targets.length > 1) {
    selectMode.value = false
    setChecked(false)
  }
  return applyToAll(
    targets,
    (p) => moveTo(p.photoId, targetNsId),
    'Media Location Updated!',
    'Update Error!',
  )
}

const downloadPhoto = (p: Photo) =>
  download(`/photo/download/${auth.token}`, `${p.photoName}.${p.format}`, {
    params: { photoId: p.photoId },
  }).catch(() => toast.error('Download failed'))

defineExpose({
  updateNsId(id: number) {
    if (props.nsOrNot) trace(id)
  },
  updatePhotoList(list: Photo[]) {
    photos.value = list
  },
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-3">
      <h2 class="text-2xl font-semibold">Medias</h2>
      <label class="flex items-center gap-2 text-sm">
        <Switch v-model="selectMode" />
        {{ selectMode ? 'Photo Select Mode' : 'Select Mode Off' }}
      </label>
      <template v-if="selectMode">
        <Button variant="outline" size="sm" @click="setChecked(false)">Clear</Button>
        <Button variant="outline" size="sm" @click="setChecked(true)">Select All</Button>
        <Button variant="secondary" size="sm" @click="askMove([...checked])">Move</Button>
        <Button v-if="detachEnabled" variant="secondary" size="sm" @click="confirmDetach = true">
          Remove From {{ detachStr }}
        </Button>
        <Button variant="destructive" size="sm" @click="confirmDelete = true"
          >Delete Selected</Button
        >
      </template>
    </div>

    <EmptyState v-if="!photos.length" title="No photos here" />
    <ul v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-6">
      <li v-for="photo in photos" :key="photo.photoId" class="[content-visibility:auto]">
        <ContextMenu>
          <ContextMenuTrigger as-child>
            <div
              class="bg-card relative overflow-hidden rounded-xl border shadow-xs transition-shadow hover:shadow-md"
            >
              <button type="button" class="block w-full text-left" @click="open(photo)">
                <img
                  :src="
                    {
                      photo: photoUrl(photo.photoId, 'renderThumbnail'),
                      video: videoIcon,
                      other: logo,
                    }[mediaKind(photo.format)]
                  "
                  alt=""
                  loading="lazy"
                  decoding="async"
                  class="aspect-[2/1] w-full object-cover"
                />
                <p class="truncate px-3 py-2 text-sm" :title="`${photo.photoName}.${photo.format}`">
                  {{ photo.photoName }}.{{ photo.format }}
                </p>
              </button>
              <Checkbox
                v-if="selectMode"
                class="bg-background absolute top-2 left-2 size-5"
                :aria-label="`Select ${photo.photoName}`"
                :model-value="!!photo.checked"
                @update:model-value="photo.checked = $event === true"
              />
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent>
            <ContextMenuItem @select="open(photo)"><Eye /> View</ContextMenuItem>
            <ContextMenuItem @select="downloadPhoto(photo)"><Download /> Download</ContextMenuItem>
            <ContextMenuItem
              variant="destructive"
              @select="
                applyToAll(
                  [photo],
                  (p) => remove(p.photoId),
                  'Photo deleted!',
                  'delete photo Response Error!',
                )
              "
            >
              <Trash2 /> Delete
            </ContextMenuItem>
            <ContextMenuItem @select="askMove([photo])"><FolderInput /> Move To</ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      </li>
    </ul>

    <PhotoViewer
      :photo="viewing"
      :ns-path="nsPath"
      @close="viewing = null"
      @changed="emit('queryPhotos')"
    />

    <AlertDialog v-model:open="confirmDelete">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription
            >Do you really want to DELETE these photos? *All photos will be
            destroied.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteChecked">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <AlertDialog v-model:open="confirmDetach">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Detach</AlertDialogTitle>
          <AlertDialogDescription
            >Do you really want to Detach these photos from this
            {{ detachStr }}?</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button @click="detachChecked">Confirm</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <MoveToShowFolder ref="movePicker" @move-to-update="moveChosen" />
  </div>
</template>
