<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Download, Eye, FolderInput, SquareCheck, Trash2 } from '@lucide/vue'
import EmptyState from '@/components/EmptyState.vue'
import { startDrag } from '@/composables/dragDrop'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import PhotoViewer from '@/components/PhotoViewer.vue'
import ViewToggle from '@/components/ViewToggle.vue'
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
import { usePhotos, type Photo } from '@/composables/usePhotos'
import { viewMode } from '@/composables/useViewMode'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { cn } from '@/lib/utils'
import { photoUrl } from '@/lib/photoUrl'
import { useAuthStore } from '@/stores/auth'
import videoIcon from '@/assets/icons/Video.png'
import logo from '@/assets/logo.ico'

/** `dragFolders`: selected folders that travel along when a selected photo is dragged. */
const props = withDefaults(
  defineProps<{
    nsOrNot: boolean
    allowSimilar?: boolean
    detachEnabled?: boolean
    detachStr?: string
    dragFolders?: number[]
    /** The parent renders one shared selection bar instead (see HomePart). */
    hideBar?: boolean
  }>(),
  { allowSimilar: true },
)
/** `queryPhotos`: the list changed on the server, reload it. `doDetach`: remove the checked photos from the parent's tag/gallery. */
const emit = defineEmits<{
  queryPhotos: [change?: { type: string; photo: Photo }]
  doDetach: [photos: Photo[]]
}>()

const auth = useAuthStore()
const { photos, moveTo, remove } = usePhotos()

/** Shared with the parent so one select mode covers folders and photos. Entered via the right-click menu. */
const selectMode = defineModel<boolean>('selectMode', { default: false })
watch(selectMode, (on) => on || setChecked(false))
const viewing = ref<Photo | null>(null)
/** Where the clicked thumbnail sits, so the viewer can grow out of it. */
const origin = ref<DOMRect | null>(null)
const nsPath = ref('')
watch(
  () => auth.token,
  () => {
    viewing.value = null
    photos.value = []
    nsPath.value = ''
  },
)
const confirmDelete = ref(false)
const confirmDetach = ref(false)
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
/** The photos the next folder pick applies to. */
const moving = ref<Photo[]>([])

/** `uploadDate` arrives as an ISO timestamp ("2024-06-01T10:20:30"); show just the day, in the local format. */
function formatDate(iso?: string) {
  const [y, m, d] = (iso ?? '').slice(0, 10).split('-').map(Number)
  if (!y || !m || !d) return ''
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

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

function open(photo: Photo, e?: MouseEvent) {
  if (selectMode.value) {
    photo.checked = !photo.checked
    return
  }
  origin.value =
    (e?.currentTarget as HTMLElement | null)?.querySelector('img')?.getBoundingClientRect() ?? null
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

function selectPhoto(photo: Photo) {
  photo.checked = !photo.checked
  selectMode.value = true
}

function dragPhoto(e: DragEvent, photo: Photo) {
  const multi = selectMode.value && photo.checked
  startDrag(
    e,
    {
      folders: multi ? (props.dragFolders ?? []) : [],
      photos: multi ? checked.value.map((p) => p.photoId) : [photo.photoId],
    },
    `${photo.photoName}.${photo.format}`,
  )
}

const downloadPhoto = (p: Photo) =>
  download(`/photo/download/${auth.token}`, `${p.photoName}.${p.format}`, {
    params: { photoId: p.photoId },
  }).catch(() => toast.error('Download failed'))

defineExpose({
  checkedCount: computed(() => checked.value.length),
  setChecked,
  deleteChecked,
  moveChecked(targetNsId: number) {
    moving.value = [...checked.value]
    return moveChosen(targetNsId)
  },
  /** Photos checked in select mode (empty when it's off). */
  checkedIds: () => (selectMode.value ? checked.value.map((p) => p.photoId) : []),
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
      <h2 class="text-2xl font-semibold">Media</h2>
      <Transition name="reveal">
        <div v-if="selectMode && !hideBar" class="flex flex-wrap items-center gap-3">
          <Button variant="outline" size="sm" @click="selectMode = false">Done</Button>
          <Button variant="outline" size="sm" @click="setChecked(false)">Clear</Button>
          <Button variant="outline" size="sm" @click="setChecked(true)">Select All</Button>
          <Button variant="secondary" size="sm" @click="askMove([...checked])">Move</Button>
          <Button v-if="detachEnabled" variant="secondary" size="sm" @click="confirmDetach = true">
            Remove From {{ detachStr }}
          </Button>
          <Button variant="destructive" size="sm" @click="confirmDelete = true"
            >Delete Selected</Button
          >
        </div>
      </Transition>
      <ViewToggle v-if="!hideBar" class="ml-auto" />
    </div>

    <EmptyState v-if="!photos.length" title="No photos here" />
    <ul
      :class="
        viewMode === 'list'
          ? 'flex flex-col gap-1'
          : 'grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-6'
      "
    >
      <li
        v-for="(photo, i) in photos"
        :key="photo.photoId"
        class="animate-rise [content-visibility:auto]"
        :style="{ animationDelay: `${Math.min(i, 12) * 30}ms` }"
      >
        <ContextMenu>
          <ContextMenuTrigger as-child>
            <div
              :class="
                cn(
                  'group bg-card hover:bg-accent relative overflow-hidden border transition-[box-shadow,background-color,translate] duration-200',
                  viewMode === 'list'
                    ? 'flex items-center rounded-lg hover:shadow-sm'
                    : 'rounded-xl shadow-xs hover:-translate-y-0.5 hover:shadow-md',
                )
              "
              draggable="true"
              @dragstart="dragPhoto($event, photo)"
              @contextmenu.stop
            >
              <Checkbox
                v-if="selectMode"
                :class="
                  cn(
                    'bg-background animate-in fade-in zoom-in-75 size-5',
                    viewMode === 'list' ? 'mx-3 shrink-0' : 'absolute top-2 left-2 z-10',
                  )
                "
                :aria-label="`Select ${photo.photoName}`"
                :model-value="!!photo.checked"
                @update:model-value="photo.checked = $event === true"
              />
              <button
                type="button"
                :class="
                  cn(
                    'w-full min-w-0 text-left',
                    viewMode === 'list' ? 'flex items-center gap-3 p-2' : 'block',
                  )
                "
                @click="open(photo, $event)"
              >
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
                  draggable="false"
                  :class="
                    viewMode === 'list'
                      ? 'size-10 shrink-0 rounded object-cover'
                      : cn(
                          'aspect-[2/1] w-full object-cover transition-[transform,filter] duration-300 group-hover:scale-[1.03]',
                          // Dim thumbnails so the corner checkbox stands out.
                          selectMode && 'brightness-75',
                        )
                  "
                />
                <p
                  :class="
                    cn('truncate text-sm', viewMode === 'list' ? 'min-w-0 flex-1' : 'px-3 py-2')
                  "
                  :title="`${photo.photoName}.${photo.format}`"
                >
                  {{ photo.photoName }}.{{ photo.format }}
                </p>
                <span
                  v-if="viewMode === 'list'"
                  class="text-muted-foreground hidden shrink-0 text-xs sm:block"
                  >Created on {{ formatDate(photo.uploadDate) }}</span
                >
              </button>
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent>
            <ContextMenuItem @select="selectPhoto(photo)">
              <SquareCheck /> {{ photo.checked ? 'Deselect' : 'Select' }}
            </ContextMenuItem>
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
      :allow-similar="allowSimilar !== false"
      :photo="viewing"
      :origin="origin"
      :ns-path="nsPath"
      @close="viewing = null"
      @changed="(change) => emit('queryPhotos', change)"
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
